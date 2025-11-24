package extract

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	argocdPkg "github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/vars"
)

// Error and timeout messages that we look for in application status
var timeoutMessages = []string{
	"Client.Timeout",
	"failed to get git client for repo",
	"rpc error: code = Unknown desc = Get \"https",
	"i/o timeout",
	"Could not resolve host: github.com",
	":8081: connect: connection refused",
	"Temporary failure in name resolution",
	"=git-upload-pack",
	"DeadlineExceeded",
}

var errorMessages = []string{
	"helm template .",
	"authentication required",
	"authentication failed",
	"error logging into OCI registry",
	"path does not exist",
	"error converting YAML to JSON",
	"Unknown desc = `helm template .",
	"Unknown desc = `kustomize build",
	"Unknown desc = Unable to resolve",
	"is not a valid chart repository or cannot be reached",
	"Unknown desc = repository not found",
	"to a commit SHA",
	"error fetching chart: failed to fetch chart: failed to get command args to log",
	"ComparisonError: Failed to load target state: failed to get cluster version for cluster",
}

var helpMessages = map[string]string{
	"ComparisonError: Failed to load target state: failed to get cluster version for cluster": "This error usually happens if your cluster is configured with 'createClusterRoles: false' and '--use-argocd-api=true' is not set",
}

func GetHelpMessage(err error) string {
	for errorMessage, helpMessage := range helpMessages {
		if strings.Contains(err.Error(), errorMessage) {
			return helpMessage
		}
	}
	return ""
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

// getResourcesFromApps extracts resources from Argo CD for a specific branch as ExtractedApp structs
func getResourcesFromApps(
	argocd *argocdPkg.ArgoCDInstallation,
	apps []argoapplication.ArgoResource,
	timeout uint64,
	prefix string,
	deleteAfterProcessing bool,
) ([]ExtractedApp, []ExtractedApp, error) {
	startTime := time.Now()

	log.Info().Msgf("🤖 Getting Applications (timeout in %d seconds)", timeout)

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
			result, k8sName, err := getResourcesFromApp(argocd, app, timeout, prefix)
			results <- struct {
				app ExtractedApp
				err error
			}{app: result, err: err}

			// release semaphore
			<-sem

			if deleteAfterProcessing {
				// Delete Application from cluster
				log.Debug().Str("App", app.GetLongName()).Msg("Deleting application from cluster")
				if err := argocd.K8sClient.DeleteArgoCDApplication(argocd.Namespace, k8sName); err != nil {
					log.Error().Err(err).Str("App", app.GetLongName()).Msg("⚠️ Failed to delete application from cluster")
				} else {
					log.Debug().Str("App", app.GetLongName()).Msg("Deleted application from cluster")
				}
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
				remainingTimeSeconds := max(0, int(timeout)-int(time.Since(startTime).Seconds()))
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
			log.Error().Err(result.err).Msg("Failed to extract app:")
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
// returns the extracted app, the k8s resource name, and an error
func getResourcesFromApp(argocd *argocdPkg.ArgoCDInstallation, app argoapplication.ArgoResource, timeout uint64, prefix string) (ExtractedApp, string, error) {

	// Store ID (kubernetes resource name) before we add a prefix and hash
	uniqueIdBeforeModifications := app.Id

	err := addApplicationPrefix(&app, prefix)
	if err != nil {
		return ExtractedApp{}, "", fmt.Errorf("failed to prefix application name with prefix: %w", err)
	}

	// After patching the application name, we can get the k8s resource name
	k8sName := app.Id

	err = labelApplicationWithRunID(&app, prefix)
	if err != nil {
		return ExtractedApp{}, k8sName, fmt.Errorf("failed to label application with run ID: %w", err)
	}

	if err := argocd.K8sClient.ApplyManifest(app.Yaml, "string", argocd.Namespace); err != nil {
		return ExtractedApp{}, k8sName, fmt.Errorf("failed to apply manifest for application %s: %w", app.GetLongName(), err)
	}

	log.Debug().Str("App", app.GetLongName()).Msg("Applied manifest for application")

	startTime := time.Now()

	for {
		// Check if we've exceeded timeout
		if time.Since(startTime).Seconds() > float64(timeout) {
			return ExtractedApp{}, k8sName, fmt.Errorf("timed out waiting for application %s", app.GetLongName())
		}

		manifestsContent, err := getManifestsFromApp(argocd, app)
		if err == nil {
			extractedApp := CreateExtractedApp(uniqueIdBeforeModifications, app.Name, app.FileName, manifestsContent, app.Branch)
			return extractedApp, k8sName, nil
		}

		log.Debug().Err(err).Str("App", app.GetLongName()).Msg("Failed to get manifests from application")

		errMsg := err.Error()
		if containsAny(errMsg, errorMessages) {
			log.Error().Str("App", app.GetLongName()).Msgf("❌ Application failed with error: %s", errMsg)
			return ExtractedApp{}, k8sName, err
		} else if containsAny(errMsg, timeoutMessages) {
			log.Warn().Str("App", app.GetLongName()).Msgf("⚠️ Application timed out with error: %s", errMsg)
			if err := argocd.RefreshApp(app.Id); err != nil {
				log.Error().Err(err).Str("App", app.GetLongName()).Msg("⚠️ Failed to refresh application")
			} else {
				log.Info().Str("App", app.GetLongName()).Msg("🔄 Refreshed application")
			}
		}

		// Check if we've exceeded timeout
		if time.Since(startTime).Seconds() > float64(timeout) {
			return ExtractedApp{}, k8sName, fmt.Errorf("timed out waiting for application %s", app.GetLongName())
		}

		// Sleep before next iteration
		time.Sleep(5 * time.Second)
	}
}

func getManifestsFromApp(argocd *argocdPkg.ArgoCDInstallation, app argoapplication.ArgoResource) ([]unstructured.Unstructured, error) {
	log.Debug().Str("App", app.GetLongName()).Msg("Extracting manifests from Application")

	attempts := 3
	manifests, err := argocd.GetManifestsWithRetry(app.Id, attempts)
	if err != nil {
		return nil, fmt.Errorf("failed to get manifests for application %s: %w", app.GetLongName(), err)
	}

	log.Debug().Str("App", app.GetLongName()).Msg("Extracted manifests from Application")

	// Replace all application IDs with the application name (relevant for annotations)
	manifests = strings.ReplaceAll(manifests, app.Id, app.Name)

	// Process the manifests into unstructured.Unstructured objects
	manifestsContent, err := processYamlOutput(manifests)
	if err != nil {
		log.Error().Err(err).Str("App", app.GetLongName()).Msg("Failed to process YAML")
		return nil, fmt.Errorf("failed to process YAML: %w", err)
	}

	// Apply Application-level ignoreDifferences (jsonPointers) before comparing diffs
	rules := parseIgnoreDifferencesFromApp(app)
	if len(rules) > 0 {
		applyIgnoreDifferencesToManifests(manifestsContent, rules)
	}

	err = removeArgoCDTrackingID(manifestsContent)
	if err != nil {
		return nil, fmt.Errorf("failed to remove Argo CD tracking ID: %w", err)
	}

	// set the namespace if not set
	for _, manifest := range manifestsContent {
		if manifest.GetNamespace() == "" {

			// namespace specified in ArgoCD application - spec.destination.namespace
			namespace, found, err := unstructured.NestedString(app.Yaml.Object, "spec", "destination", "namespace")
			if err != nil {
				return nil, fmt.Errorf("failed to get namespace from application: %w", err)
			}
			if found {
				manifest.SetNamespace(namespace)
			}
		}
	}

	// remove Helm hooks resources
	newManifestsContent := make([]unstructured.Unstructured, 0, len(manifestsContent))
	for _, manifest := range manifestsContent {
		if HelmHookFilter(manifest) {
			newManifestsContent = append(newManifestsContent, manifest)
		}
	}
	manifestsContent = newManifestsContent

	// Parse the first non-empty manifest from the string
	return manifestsContent, nil
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

		// Remove the entire annotations field if it's now empty
		if len(annotations) == 0 {
			obj.SetAnnotations(nil)
		} else {
			obj.SetAnnotations(annotations)
		}
	}

	return nil
}

// returns true if the object is NOT a Helm hook
func HelmHookFilter(obj unstructured.Unstructured) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return true
	}
	_, exists := annotations["helm.sh/hook"]
	return !exists
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if s != "" && strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
