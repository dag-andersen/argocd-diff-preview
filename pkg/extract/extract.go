package extract

import (
	"errors"
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

// const worker count
const maxWorkers = 40

// RenderApplicaitonsFromBothBranches extracts resources from both base and target branches
// by applying their manifests to the cluster and capturing the resulting resources
func RenderApplicaitonsFromBothBranches(
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

	// print how many applications are being rendered for each branch
	log.Info().Msgf("📌 Final number of Applications planned to be rendered: [Base: %d], [Target: %d]", len(baseApps), len(targetApps))

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

	var namespacedScopedResources map[string]bool
	if argocd.UseAPI() {
		nsr, err := argocd.K8sClient.GetListOfNamespacedScopedResources()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get list of namespaced scoped resources: %w", err)
		}
		namespacedScopedResources = nsr
	}

	log.Info().Msgf("🤖 Rendering Applications (timeout in %d seconds)", timeout)

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
			result, k8sName, err := getResourcesFromApp(argocd, app, timeout, prefix, namespacedScopedResources)
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

	for range len(apps) {
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

	log.Info().Msgf("🎉 Rendered all %d applications", renderedApps)

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
func getResourcesFromApp(
	argocd *argocdPkg.ArgoCDInstallation,
	app argoapplication.ArgoResource,
	timeout uint64,
	prefix string,
	namespacedScopedResources map[string]bool,
) (ExtractedApp, string, error) {

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

		// If Application is still marked, then we need to wait for it to be refreshed
		if isMarked, _ := argocd.K8sClient.IsApplicationMarkedForRefresh(argocd.Namespace, app.Id); isMarked {
			log.Debug().Str("App", app.GetLongName()).Msg("Waiting for Application to be refreshed")
			time.Sleep(2 * time.Second)
			continue
		}

		manifestsContent, err := getManifestsFromApp(argocd, app, namespacedScopedResources)

		// If the application is seemingly empty, check the application status and refresh and try again
		attempts := 0
		for attempts < 2 && err == nil && len(manifestsContent) == 0 {
			attempts++

			reconsiled, argoErrMessage, internalError := argoapplication.GetErrorStatusFromApplication(argocd, app)
			log.Debug().Str("App", app.GetLongName()).Msgf("Application is empty. Argo CD Error: %v, Internal Error: %v", argoErrMessage, internalError)
			if argoErrMessage != nil {
				if argocd.UseAPI() && isExpectedError(argoErrMessage.Error()) {
					log.Debug().Str("App", app.GetLongName()).Err(argoErrMessage).Msgf("Expected error because Argo CD is running with '--use-argocd-api=true' and Argo CD may be running with 'createClusterRoles: false'")
					break
				} else {
					err = argoErrMessage
					break
				}
			} else if internalError != nil {
				err = internalError
				break
			}

			if !reconsiled {
				err = errors.New("application is not reconciled")
				break
			}

			// refresh application just to be sure
			log.Debug().Str("App", app.GetLongName()).Msg("No manifests and no error found for application, refreshing and trying again just to be sure")
			if err := argocd.RefreshApp(app.Id); err != nil {
				log.Error().Err(err).Str("App", app.GetLongName()).Msg("⚠️ Failed to refresh application")
			} else {
				log.Debug().Str("App", app.GetLongName()).Msg("Refreshed application")
			}

			time.Sleep(time.Second)
			manifestsContent, err = getManifestsFromApp(argocd, app, namespacedScopedResources)
		}

		// If still no error, return the extracted app
		if err == nil {
			if len(manifestsContent) == 0 {
				log.Warn().Str("App", app.GetLongName()).Msg("⚠️ No manifests found for application")
			}
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
		time.Sleep(3 * time.Second)
	}
}

func getManifestsFromApp(argocd *argocdPkg.ArgoCDInstallation, app argoapplication.ArgoResource, namespacedScopedResources map[string]bool) ([]unstructured.Unstructured, error) {
	log.Debug().Str("App", app.GetLongName()).Msg("Extracting manifests from Application")

	extractionTimer := time.Now()
	manifests, exists, err := argocd.GetManifests(app.Id)
	if !exists {
		return nil, fmt.Errorf("%s", string(errorApplicationNotFound))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get manifests for application %s: %w", app.GetLongName(), err)
	}

	if strings.TrimSpace(manifests) == "" {
		log.Debug().Str("App", app.GetLongName()).Msgf("No manifests found for application in %s", time.Since(extractionTimer).Round(time.Second))
		return []unstructured.Unstructured{}, nil
	}

	log.Debug().Str("App", app.GetLongName()).Msgf("Extracted manifests from Application in %s", time.Since(extractionTimer).Round(time.Second))

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

	if argocd.UseAPI() {
		// set the namespace if not set
		for _, manifest := range manifestsContent {
			if manifest.GetNamespace() == "" {
				// namespace specified in ArgoCD application - spec.destination.namespace
				namespace, found, err := unstructured.NestedString(app.Yaml.Object, "spec", "destination", "namespace")
				if err != nil {
					return nil, fmt.Errorf("failed to get namespace from application: %w", err)
				}
				if found {
					key := fmt.Sprintf("%s/%s", manifest.GetKind(), manifest.GetAPIVersion())
					if namespaced, found := namespacedScopedResources[key]; found && namespaced {
						manifest.SetNamespace(namespace)
					}
				}
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
