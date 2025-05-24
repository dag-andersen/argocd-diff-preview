package extract

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	argocdPkg "github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/vars"
)

// Error and timeout messages that we look for in application status
// var errorMessages = []string{
// 	"helm template .",
// 	"authentication required",
// 	"authentication failed",
// 	"path does not exist",
// 	"error converting YAML to JSON",
// 	"Unknown desc = `helm template .",
// 	"Unknown desc = `kustomize build",
// 	"Unknown desc = Unable to resolve",
// 	"is not a valid chart repository or cannot be reached",
// 	"Unknown desc = repository not found",
// 	"to a commit SHA",
// }

var timeoutMessages = []string{
	"Client.Timeout",
	"failed to get git client for repo",
	"rpc error: code = Unknown desc = Get \"https",
	"i/o timeout",
	"Could not resolve host: github.com",
	":8081: connect: connection refused",
	"Temporary failure in name resolution",
	"=git-upload-pack",
}

// const worker count
const maxWorkers = 40

// contains a app name, source path, and extracted manifest
type ExtractedApp struct {
	Id         string
	Name       string
	SourcePath string
	Manifest   []unstructured.Unstructured
	Branch     git.BranchType
}

// CreateExtractedApp creates an ExtractedApp from an ArgoResource
func CreateExtractedApp(id string, name string, sourcePath string, manifest []unstructured.Unstructured, branch git.BranchType) ExtractedApp {
	return ExtractedApp{
		Id:         id,
		Name:       name,
		SourcePath: sourcePath,
		Manifest:   manifest,
		Branch:     branch,
	}
}

// GetResourcesFromBothBranches extracts resources from both base and target branches
// by applying their manifests to the cluster and capturing the resulting resources
func GetResourcesFromBothBranches(
	argocd *argocdPkg.ArgoCDInstallation,
	timeout uint64,
	baseApps []argoapplication.ArgoResource,
	targetApps []argoapplication.ArgoResource,
	prefix string,
	deleteAfterProcessing bool,
) ([]ExtractedApp, []ExtractedApp, time.Duration, error) {
	startTime := time.Now()

	if err := verifyNoDuplicateAppIds(baseApps); err != nil {
		return nil, nil, time.Since(startTime), err
	}

	if err := verifyNoDuplicateAppIds(targetApps); err != nil {
		return nil, nil, time.Since(startTime), err
	}

	apps := append(baseApps, targetApps...)

	log.Debug().Msg("Applied manifest for both branches")
	extractedBaseApps, extractedTargetApps, err := getResourcesFromApps(argocd, apps, timeout, prefix, deleteAfterProcessing)
	if err != nil {
		return nil, nil, time.Since(startTime), fmt.Errorf("failed to get resources: %w", err)
	}
	log.Debug().Msg("Extracted manifests for both branches")

	return extractedBaseApps, extractedTargetApps, time.Since(startTime), nil
}

func verifyNoDuplicateAppIds(apps []argoapplication.ArgoResource) error {
	appNames := make(map[string]bool)
	for _, app := range apps {
		if appNames[app.Id] {
			return fmt.Errorf("duplicate app name: %s", app.Id)
		}
		appNames[app.Id] = true
	}
	return nil
}

// getResourcesFromApps extracts resources from Argo CD for a specific branch as ExtractedApp structs
func getResourcesFromApps(
	argocd *argocdPkg.ArgoCDInstallation,
	apps []argoapplication.ArgoResource,
	timeout uint64,
	prefix string,
	deleteAfterProcessing bool,
) ([]ExtractedApp, []ExtractedApp, error) {
	startTime := time.Now()

	log.Info().Msg("🤖 Getting Applications")

	// Process apps in parallel with a worker pool
	results := make(chan struct {
		app ExtractedApp
		err error
	}, len(apps))

	// Create a semaphore channel to limit concurrent workers
	sem := make(chan struct{}, maxWorkers)

	// Use WaitGroup to wait for all goroutines to complete (including deletions)
	var wg sync.WaitGroup

	for _, app := range apps {
		sem <- struct{}{} // Acquire semaphore
		wg.Add(1)         // Add to wait group
		go func(app argoapplication.ArgoResource) {
			defer wg.Done() // Signal completion when goroutine ends
			result, err := getResourcesFromApp(argocd, app, timeout, prefix)
			results <- struct {
				app ExtractedApp
				err error
			}{app: result, err: err}

			// release semaphore
			<-sem

			if deleteAfterProcessing {
				// Delete Application from cluster
				log.Debug().Str("App", app.GetLongName()).Msg("Deleting application from cluster")
				if err := argocd.K8sClient.DeleteArgoCDApplication(argocd.Namespace, result.Id); err != nil {
					log.Error().Err(err).Str("App", app.GetLongName()).Msg("⚠️ Failed to delete application from cluster")
				}
				log.Debug().Str("App", app.GetLongName()).Msg("Deleted application from cluster")
			}
		}(app)
	}

	// Collect results
	extractedBaseApps := make([]ExtractedApp, 0, len(apps))
	extractedTargetApps := make([]ExtractedApp, 0, len(apps))
	var firstError error

	// Setup progress tracking
	totalApps := len(apps)
	renderedApps := 0
	progressDone := make(chan bool)

	// Start progress reporting goroutine
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				remainingTimeSeconds := int(timeout) - int(time.Since(startTime).Seconds())
				log.Info().Msgf("🤖 Rendered %d out of %d applications (timeout in %d seconds)", renderedApps, totalApps, remainingTimeSeconds)
			case <-progressDone:
				return
			}
		}
	}()

	for i := 0; i < len(apps); i++ {
		result := <-results
		if result.err != nil {
			if firstError == nil {
				firstError = result.err
			}
			log.Error().Err(result.err).Msg("Failed to extract app")
			continue
		}
		switch result.app.Branch {
		case git.Base:
			extractedBaseApps = append(extractedBaseApps, result.app)
		case git.Target:
			extractedTargetApps = append(extractedTargetApps, result.app)
		default:
			return nil, nil, fmt.Errorf("unknown branch type: '%s'", result.app.Branch)
		}
		renderedApps++
	}

	// Signal progress reporting to stop
	close(progressDone)

	if firstError != nil {
		return nil, nil, firstError
	}

	// Wait for all goroutines to complete (including deletions)
	log.Info().Msg("🧼 Waiting for all application deletions to complete...")
	wg.Wait()
	log.Info().Msg("🧼 All application deletions completed")

	duration := time.Since(startTime)
	log.Info().Msgf("🤖 Got all resources from %d applications from %s-branch and got %d from %s-branch in %s", len(extractedBaseApps), git.Base, len(extractedTargetApps), git.Target, duration.Round(time.Second))

	return extractedBaseApps, extractedTargetApps, nil
}

// getResourcesFromApp extracts a single application from the cluster
func getResourcesFromApp(argocd *argocdPkg.ArgoCDInstallation, app argoapplication.ArgoResource, timeout uint64, prefix string) (ExtractedApp, error) {
	// Apply the application manifest first

	err := prefixApplication(&app, prefix)
	if err != nil {
		return ExtractedApp{}, fmt.Errorf("failed to prefix application name with prefix: %w", err)
	}

	err = labelApplicationWithRunID(&app, prefix)
	if err != nil {
		return ExtractedApp{}, fmt.Errorf("failed to label application with run ID: %w", err)
	}

	if err := argocd.K8sClient.ApplyManifest(app.Yaml, "string", argocd.Namespace); err != nil {
		return ExtractedApp{}, fmt.Errorf("failed to apply manifest for application %s: %w", app.GetLongName(), err)
	}

	log.Debug().Str("name", app.GetLongName()).Msg("Applied manifest for application")

	startTime := time.Now()
	var result ExtractedApp

	for {
		// Check if we've exceeded timeout
		if time.Since(startTime).Seconds() > float64(timeout) {
			return result, fmt.Errorf("timed out waiting for application %s", app.GetLongName())
		}

		// Get application status
		output, err := argocd.K8sClient.GetArgoCDApplication(argocd.Namespace, app.Id)
		if err != nil {
			return result, fmt.Errorf("failed to get application %s: %w", app.GetLongName(), err)
		}

		var appStatus struct {
			Status struct {
				Sync struct {
					Status string `yaml:"status"`
				} `yaml:"sync"`
				Conditions []struct {
					Type    string `yaml:"type"`
					Message string `yaml:"message"`
				} `yaml:"conditions"`
			} `yaml:"status"`
		}

		if err := yaml.Unmarshal([]byte(output), &appStatus); err != nil {
			return result, fmt.Errorf("failed to parse application yaml for %s: %w", app.GetLongName(), err)
		}

		switch appStatus.Status.Sync.Status {
		case "OutOfSync", "Synced":
			log.Debug().Str("name", app.GetLongName()).Msg("Extracting manifests from Application")
			manifests, exists, err := argocd.GetManifests(app.Id)
			if !exists {
				return result, fmt.Errorf("application %s does not exist", app.GetLongName())
			}

			if err != nil {
				return result, fmt.Errorf("failed to get manifests for application %s: %w", app.GetLongName(), err)
			}

			log.Debug().Str("name", app.GetLongName()).Msg("Extracted manifests from Application")

			manifests = strings.ReplaceAll(manifests, app.Id, app.Name)
			manifestsContent, err := processYamlChunk(manifests)
			if err != nil {
				return result, fmt.Errorf("failed to process YAML: %w", err)
			}

			err = removeArgoCDTrackingID(manifestsContent)
			if err != nil {
				return result, fmt.Errorf("failed to remove Argo CD tracking ID: %w", err)
			}

			// Parse the first non-empty manifest from the string
			extractedApp := CreateExtractedApp(app.Id, app.Name, app.FileName, manifestsContent, app.Branch)

			return extractedApp, nil

		case "Unknown":
			for _, condition := range appStatus.Status.Conditions {
				if isErrorCondition(condition.Type) {
					msg := condition.Message
					if containsAny(msg, timeoutMessages) {
						log.Warn().Str("App", app.GetLongName()).Msgf("⚠️ Application timed out with error: %s", msg)
						if err := argocd.RefreshApp(app.Id); err != nil {
							log.Error().Err(err).Str("App", app.GetLongName()).Msg("⚠️ Failed to refresh application")
						} else {
							log.Info().Str("App", app.GetLongName()).Msg("🔄 Refreshed application")
						}
					} else {
						log.Error().Str("App", app.GetLongName()).Msgf("❌ Application failed with error: %s", msg)
						return result, fmt.Errorf("application %s failed: %s", app.Name, msg)
					}
				}
			}
		}

		// Sleep before next iteration
		time.Sleep(5 * time.Second)
	}
}

// processYamlChunk parses a YAML chunk into an unstructured.Unstructured
// A chunk is a single YAML object, e.g. a Deployment, Service, etc.
func processYamlChunk(chunk string) ([]unstructured.Unstructured, error) {

	// split
	documents := strings.Split(chunk, "---")

	manifests := make([]unstructured.Unstructured, 0)

	for _, doc := range documents {
		// Skip empty documents
		trimmedDoc := strings.TrimSpace(doc)

		if trimmedDoc == "" {
			continue
		}

		// Create a new map to hold the parsed YAML
		var yamlObj map[string]interface{}
		err := yaml.Unmarshal([]byte(trimmedDoc), &yamlObj)
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}

		// Skip empty objects
		if len(yamlObj) == 0 {
			continue
		}

		// Check if this is a valid Kubernetes resource
		apiVersion, found, _ := unstructured.NestedString(yamlObj, "apiVersion")
		kind, kindFound, _ := unstructured.NestedString(yamlObj, "kind")

		if !found || !kindFound || apiVersion == "" || kind == "" {
			log.Debug().Msgf("Found manifest with no apiVersion or kind: %s", trimmedDoc)
			continue
		}

		manifests = append(manifests, unstructured.Unstructured{Object: yamlObj})
	}

	log.Debug().Msgf("Parsed %d manifests", len(manifests))

	return manifests, nil
}

// prefixApplication prefixes the application name with the branch name and a unique ID
func prefixApplication(a *argoapplication.ArgoResource, prefix string) error {
	if a.Branch == "" {
		log.Warn().Str(a.Kind.ShortName(), a.GetLongName()).Msg("⚠️ Can't prefix application name with prefix because branch is empty")
		return nil
	}

	var branchShortName string
	switch a.Branch {
	case git.Base:
		branchShortName = "b"
	case git.Target:
		branchShortName = "t"
	}

	prefixSize := len(prefix) + len(branchShortName) + len("--")
	var newId string
	if prefixSize+len(a.Id) > 53 {
		// hash id so it becomes shorter
		hashedId := fmt.Sprintf("%x", sha256.Sum256([]byte(a.Id)))
		newId = fmt.Sprintf("%s-%s-%s", prefix, branchShortName, hashedId[:53-prefixSize])
	} else {
		newId = fmt.Sprintf("%s-%s-%s", prefix, branchShortName, a.Id)
	}

	a.Id = newId
	a.Yaml.SetName(newId)

	return nil
}

func labelApplicationWithRunID(a *argoapplication.ArgoResource, runID string) error {
	labels := a.Yaml.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[vars.ArgoCDApplicationLabelKey] = runID
	a.Yaml.SetLabels(labels)
	return nil
}

// removeArgoCDTrackingID removes the "argocd.argoproj.io/tracking-id" annotation from the application
func removeArgoCDTrackingID(a []unstructured.Unstructured) error {
	for _, obj := range a {
		annotations := obj.GetAnnotations()
		if annotations == nil {
			continue
		}

		for key := range annotations {
			if key == "argocd.argoproj.io/tracking-id" {
				delete(annotations, key)
			}
		}

		obj.SetAnnotations(annotations)
	}

	return nil
}

func isErrorCondition(condType string) bool {
	return condType != "" && containsIgnoreCase(condType, "error")
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if s != "" && strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
