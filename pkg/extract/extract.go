package extract

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/controller"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	argocdPkg "github.com/dag-andersen/argocd-diff-preview/pkg/argocd"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/vars"
)

// resourceInfoProvider implements kubeutil.ResourceInfoProvider interface
// to provide namespace scope information for Kubernetes resources
type resourceInfoProvider struct {
	namespacedByGk map[schema.GroupKind]bool
}

// IsNamespaced returns true if the given GroupKind is namespaced
func (p *resourceInfoProvider) IsNamespaced(gk schema.GroupKind) (bool, error) {
	return p.namespacedByGk[gk], nil
}

// const worker count
const maxWorkers = 40

// RenderApplicationsFromBothBranches extracts resources from both base and target branches
// by applying their manifests to the cluster and capturing the resulting resources
func RenderApplicationsFromBothBranches(
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

	// Get list of namespaced resources for namespace normalization
	// This is needed because `argocd app manifests --revision` returns raw manifests
	// without namespace normalization that the controller cache would normally provide
	namespacedScopedResources, err := argocd.K8sClient.GetListOfNamespacedScopedResources()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get list of namespaced scoped resources: %w", err)
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

	// Setup progress tracking
	totalApps := len(apps)
	var renderedApps atomic.Int32
	progressDone := make(chan bool)

	// remainingTime returns the seconds left until timeout
	remainingTime := func() int {
		return max(0, int(timeout)-int(time.Since(startTime).Seconds()))
	}

	// Start progress reporting goroutine before launching extraction workers
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Info().Msgf("🤖 Rendered %d out of %d applications (timeout in %d seconds)", renderedApps.Load(), totalApps, remainingTime())
			case <-progressDone:
				return
			}
		}
	}()

	for _, app := range apps {
		sem <- struct{}{} // Acquire semaphore
		wg.Add(1)         // Add to wait group
		go func(app argoapplication.ArgoResource) {
			defer wg.Done() // Signal completion when goroutine ends
			timeRemaining := remainingTime()

			// If timeout is reached, return an empty extracted app and an error
			if timeRemaining <= 0 {
				results <- struct {
					app ExtractedApp
					err error
				}{app: ExtractedApp{}, err: fmt.Errorf("timeout reached before starting to render application: %s", app.GetLongName())}
				<-sem
				return
			}

			// Get resources from application
			result, k8sName, err := getResourcesFromApp(argocd, app, timeRemaining, prefix, namespacedScopedResources)
			results <- struct {
				app ExtractedApp
				err error
			}{app: result, err: err}
			if err == nil {
				renderedApps.Add(1)
			}

			// Release semaphore
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

	for range len(apps) {
		result := <-results
		if result.err != nil {
			if firstError == nil {
				firstError = result.err
			}
			log.Error().Err(result.err).Msg("❌ Failed to extract application:")
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
	}

	// Signal progress reporting to stop
	close(progressDone)

	if firstError != nil {
		return nil, nil, firstError
	}

	log.Info().Msgf("🎉 Rendered all %d applications", renderedApps.Load())

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
	timeout int,
	prefix string,
	namespacedScopedResources map[schema.GroupKind]bool,
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

	triedRefreshing := false

	loopCount := -1 // -1 means we haven't started the loop yet
	for {
		loopCount++

		// Check if we've exceeded timeout
		if time.Since(startTime).Seconds() > float64(timeout) {
			return ExtractedApp{}, k8sName, fmt.Errorf("timed out waiting for application '%s'", app.GetLongName())
		}

		reconciled, isMarkedForRefresh, argoErrMessage, internalError, err := argoapplication.GetApplicationStatus(argocd, app)
		log.Debug().Str("loop", strconv.Itoa(loopCount)).Str("App", app.GetLongName()).Msgf("Application status: [reconciled: %v], [Marked for refresh: %v], [Argo CD Error: %v], [Internal Error: %v], [err: %v]", reconciled, isMarkedForRefresh, argoErrMessage, internalError, err != nil)
		if err != nil {
			return ExtractedApp{}, k8sName, err
		}

		// If Application is still marked for refresh, then we need to wait for it to be refreshed
		if isMarkedForRefresh {
			log.Debug().Str("loop", strconv.Itoa(loopCount)).Str("App", app.GetLongName()).Msg("Waiting for Application to be refreshed")
			time.Sleep(time.Second)
			continue
		}

		manifestsContent, err := getManifestsFromApp(argocd, app, namespacedScopedResources)

		// If we got manifests with no error, return the extracted app.Ignore all errors
		if err == nil && len(manifestsContent) > 0 {
			log.Debug().Str("loop", strconv.Itoa(loopCount)).Str("App", app.GetLongName()).Msgf("Successfully extracted %d manifests from application", len(manifestsContent))
			extractedApp := CreateExtractedApp(uniqueIdBeforeModifications, app.Name, app.FileName, manifestsContent, app.Branch)
			return extractedApp, k8sName, nil
		}

		// If we got no error and no manifests, check if we can get the error status from the application itself
		if err == nil {
			// Otherwise, the application might be empty, check the application status and update.
			if argoErrMessage != nil {
				if isExpected, reason := argocd.IsExpectedError(argoErrMessage.Error()); isExpected {
					log.Debug().Str("loop", strconv.Itoa(loopCount)).Str("App", app.GetLongName()).Err(argoErrMessage).Msgf("Expected error: %s", reason)
				} else {
					err = argoErrMessage
				}
			} else if internalError != nil {
				err = internalError
			} else if !reconciled {
				err = fmt.Errorf("application is not reconciled")
			}
		}

		// If still got no error anywhere, lets try to refresh the application and try to extract the manifests again.
		// Only refresh if we haven't tried refreshing yet.
		if err == nil && !triedRefreshing {
			log.Debug().Str("loop", strconv.Itoa(loopCount)).Str("App", app.GetLongName()).Msg("Application seems to be empty and with no error. Will try to refresh it just to be sure.")
			if err := argocd.RefreshApp(app.Id); err != nil {
				log.Debug().Err(err).Str("loop", strconv.Itoa(loopCount)).Str("App", app.GetLongName()).Msg("Failed to refresh application")
			} else {
				log.Debug().Str("loop", strconv.Itoa(loopCount)).Str("App", app.GetLongName()).Msg("Requested refresh of application because it was empty and with no error")
				triedRefreshing = true
				time.Sleep(time.Second)
				continue
			}
		}

		// If still got no error anywhere and already tried refreshing, return the extracted app. We assume the application was just empty.
		if err == nil {
			log.Warn().Str("App", app.GetLongName()).Msg("⚠️ No manifests found for application")
			extractedApp := CreateExtractedApp(uniqueIdBeforeModifications, app.Name, app.FileName, manifestsContent, app.Branch)
			return extractedApp, k8sName, nil
		}

		log.Debug().Str("loop", strconv.Itoa(loopCount)).Err(err).Str("App", app.GetLongName()).Msg("Failed to get manifests from application")

		// Check if the error is a known error
		errMsg := err.Error()
		if containsAny(errMsg, errorMessages) {
			log.Error().Str("App", app.GetLongName()).Msgf("❌ Application failed with error: %s", errMsg)
			return ExtractedApp{}, k8sName, err
		} else if containsAny(errMsg, timeoutMessages) {
			log.Warn().Str("App", app.GetLongName()).Msgf("⚠️ Application timed out with error: %s", errMsg)
			if err := argocd.RefreshApp(app.Id); err != nil {
				log.Error().Err(err).Str("App", app.GetLongName()).Msg("⚠️ Failed to refresh application")
			} else {
				log.Info().Str("App", app.GetLongName()).Msg("🔄 Requested refresh of application")
				triedRefreshing = true
			}
		}

		// Check if we've exceeded timeout
		if time.Since(startTime).Seconds() > float64(timeout) {
			return ExtractedApp{}, k8sName, fmt.Errorf("timed out waiting for application '%s'", app.GetLongName())
		}

		// Sleep before next iteration
		time.Sleep(3 * time.Second)
	}
}

func getManifestsFromApp(argocd *argocdPkg.ArgoCDInstallation, app argoapplication.ArgoResource, namespacedScopedResources map[schema.GroupKind]bool) ([]unstructured.Unstructured, error) {
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

	// Normalize namespaces and deduplicate resources (same logic as Argo CD controller)
	destNamespace, _, _ := unstructured.NestedString(app.Yaml.Object, "spec", "destination", "namespace")
	manifestsContent, err = normalizeNamespaces(manifestsContent, destNamespace, namespacedScopedResources, app.GetLongName())
	if err != nil {
		return nil, err
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

// normalizeNamespaces uses Argo CD's DeduplicateTargetObjects to normalize namespaces on manifests.
// This adds the destination namespace to namespaced resources that don't have one,
// clears namespace from cluster-scoped resources, and deduplicates resources with the same key.
// This matches the behavior of Argo CD's controller when processing target objects.
func normalizeNamespaces(
	manifests []unstructured.Unstructured,
	destNamespace string,
	namespacedResources map[schema.GroupKind]bool,
	appName string,
) ([]unstructured.Unstructured, error) {
	if destNamespace == "" {
		return manifests, nil
	}

	// Convert to pointer slice for DeduplicateTargetObjects
	ptrManifests := make([]*unstructured.Unstructured, len(manifests))
	for i := range manifests {
		ptrManifests[i] = &manifests[i]
	}

	provider := &resourceInfoProvider{namespacedByGk: namespacedResources}
	deduplicatedManifests, conditions, err := controller.DeduplicateTargetObjects(destNamespace, ptrManifests, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to normalize namespaces: %w", err)
	}

	// Log any duplicate resource warnings
	for _, cond := range conditions {
		log.Warn().Str("App", appName).Msgf("Duplicate resource warning: %s", cond.Message)
	}

	// Convert back to value slice
	result := make([]unstructured.Unstructured, len(deduplicatedManifests))
	for i, ptr := range deduplicatedManifests {
		result[i] = *ptr
	}

	return result, nil
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
			if key == common.AnnotationKeyAppInstance {
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
