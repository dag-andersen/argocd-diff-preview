package argoapplication

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func patchApplications(
	argocdNamespace string,
	applications []ArgoResource,
	branch *git.Branch,
	repo string,
	redirectRevisions []string,
) ([]ArgoResource, error) {
	var patchedApps []ArgoResource

	for _, app := range applications {
		patchedApp, err := patchApplication(
			argocdNamespace,
			app,
			branch,
			repo,
			redirectRevisions,
		)
		if err != nil {
			return nil, err
		}
		patchedApps = append(patchedApps, *patchedApp)
	}

	return patchedApps, nil
}

// patchApplication patches a single ArgoResource
func patchApplication(
	argocdNamespace string,
	app ArgoResource,
	branch *git.Branch,
	repo string,
	redirectRevisions []string,
) (*ArgoResource, error) {

	// Chain the modifications
	app.SetNamespace(argocdNamespace)

	err := app.RemoveSyncPolicy()
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", app.GetLongName())
		return nil, fmt.Errorf("failed to remove sync policy: %w", err)
	}

	err = app.SetProjectToDefault()
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", app.GetLongName())
		return nil, fmt.Errorf("failed to set project to default: %w", err)
	}

	err = app.SetDestinationServerToLocal()
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", app.GetLongName())
		return nil, fmt.Errorf("failed to set destination server to local: %w", err)
	}

	err = app.RemoveArgoCDFinalizers()
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", app.GetLongName())
		return nil, fmt.Errorf("failed to remove Argo CD finalizers: %w", err)
	}

	err = app.RedirectSources(repo, branch.Name, redirectRevisions)
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", app.GetLongName())
		return nil, fmt.Errorf("failed to redirect sources: %w", err)
	}

	err = app.RedirectGenerators(repo, branch.Name, redirectRevisions)
	if err != nil {
		log.Info().Msgf("❌ Failed to patch application: %s", app.GetLongName())
		return nil, fmt.Errorf("failed to redirect generators: %w", err)
	}

	return &app, nil
}

// SetNamespace sets the namespace of the resource
func (a *ArgoResource) SetNamespace(namespace string) {
	a.Yaml.SetNamespace(namespace)
}

// SetProjectToDefault sets the project to "default"
func (a *ArgoResource) SetProjectToDefault() error {
	if a.Yaml == nil {
		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msg("Resource contains no YAML")
		return nil
	}

	switch a.Kind {
	case Application:
		if _, found, _ := unstructured.NestedString(a.Yaml.Object, "spec", "project"); !found {
			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no 'spec.project' key found in %s", a.Kind.ShortName())
		}
		if err := unstructured.SetNestedField(a.Yaml.Object, "default", "spec", "project"); err != nil {
			return fmt.Errorf("failed to set spec.project field: %w", err)
		}
	case ApplicationSet:
		if _, found, _ := unstructured.NestedString(a.Yaml.Object, "spec", "template", "spec", "project"); !found {
			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no 'spec.template.spec.project' key found in %s", a.Kind.ShortName())
		}
		if err := unstructured.SetNestedField(a.Yaml.Object, "default", "spec", "template", "spec", "project"); err != nil {
			return fmt.Errorf("failed to set spec.template.spec.project field: %w", err)
		}
	}

	return nil
}

// SetDestinationServerToLocal updates the destination to point to the in-cluster service
func (a *ArgoResource) SetDestinationServerToLocal() error {
	if a.Yaml == nil {
		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msg("Resource contains no YAML")
		return nil
	}

	var destPath []string
	switch a.Kind {
	case Application:
		destPath = []string{"spec", "destination"}
	default:
		return nil
	}

	// Check if destination exists
	destMap, found, _ := unstructured.NestedMap(a.Yaml.Object, destPath...)
	if !found {
		log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no '%s' key found in %s",
			strings.Join(destPath, "."), a.Kind.ShortName())
		return nil
	}

	// Update destination
	delete(destMap, "name")
	destMap["server"] = "https://kubernetes.default.svc"

	// Set it back
	if err := unstructured.SetNestedMap(a.Yaml.Object, destMap, destPath...); err != nil {
		return fmt.Errorf("failed to set destination field: %w", err)
	}

	return nil
}

// RemoveArgoCDFinalizers removes only the resources-finalizer.argocd.argoproj.io finalizer
func (a *ArgoResource) RemoveArgoCDFinalizers() error {
	finalizers := a.Yaml.GetFinalizers()
	if finalizers == nil {
		return nil
	}

	// Filter out Argo CD finalizer in a single operation
	filteredFinalizers := finalizers[:0]
	for _, f := range finalizers {
		if f != "resources-finalizer.argocd.argoproj.io" {
			filteredFinalizers = append(filteredFinalizers, f)
		}
	}

	a.Yaml.SetFinalizers(filteredFinalizers)

	return nil
}

// RemoveSyncPolicy removes the syncPolicy from the resource
func (a *ArgoResource) RemoveSyncPolicy() error {
	if a.Yaml == nil {
		log.Warn().Str("patchType", "removeSyncPolicy").Str(a.Kind.ShortName(), a.GetLongName()).Msg("⚠️ Can't remove 'syncPolicy' because YAML is nil")
		return nil
	}

	var specPath []string
	switch a.Kind {
	case Application:
		specPath = []string{"spec"}
	case ApplicationSet:
		specPath = []string{"spec", "template", "spec"}
	default:
		return nil
	}

	// Check if spec exists
	specMap, found, _ := unstructured.NestedMap(a.Yaml.Object, specPath...)
	if !found {
		log.Warn().Str("patchType", "removeSyncPolicy").Str(a.Kind.ShortName(), a.GetLongName()).Msg("⚠️ Can't remove 'syncPolicy' because spec not found")
		return nil
	}

	// Remove syncPolicy
	delete(specMap, "syncPolicy")

	// Set it back
	if err := unstructured.SetNestedMap(a.Yaml.Object, specMap, specPath...); err != nil {
		return fmt.Errorf("failed to set %s map: %w", strings.Join(specPath, "."), err)
	}

	return nil
}

// RedirectSources updates the source/sources targetRevision to point to the specified branch
func (a *ArgoResource) RedirectSources(repo, branch string, redirectRevisions []string) error {
	if a.Yaml == nil {
		log.Warn().Str("patchType", "redirectSources").Str(a.Kind.ShortName(), a.GetLongName()).Msg("⚠️ No YAML for Application")
		return nil
	}

	var specPath []string
	switch a.Kind {
	case Application:
		specPath = []string{"spec"}
	case ApplicationSet:
		specPath = []string{"spec", "template", "spec"}
	default:
		return nil
	}

	// Get spec
	specMap, found, _ := unstructured.NestedMap(a.Yaml.Object, specPath...)
	if !found {
		log.Warn().Str("patchType", "redirectSources").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("⚠️ No spec found in %s", a.Kind.ShortName())
		return nil
	}

	// Handle single source
	if source, ok := specMap["source"].(map[string]any); ok {
		if err := a.redirectSourceMap(source, repo, branch, redirectRevisions); err != nil {
			return err
		}
	}

	// Handle multiple sources
	if sourcesInterface, ok := specMap["sources"]; ok {
		if sources, ok := sourcesInterface.([]any); ok {
			for _, sourceInterface := range sources {
				if source, ok := sourceInterface.(map[string]any); ok {
					if err := a.redirectSourceMap(source, repo, branch, redirectRevisions); err != nil {
						return err
					}
				}
			}
		}
	}

	// Set updated spec back
	if err := unstructured.SetNestedMap(a.Yaml.Object, specMap, specPath...); err != nil {
		return fmt.Errorf("failed to set %s map: %w", strings.Join(specPath, "."), err)
	}

	return nil
}

// Helper function to redirect a single source
func (a *ArgoResource) redirectSourceMap(source map[string]any, repo, branch string, redirectRevisions []string) error {
	// Skip helm charts
	if _, hasChart := source["chart"]; hasChart {
		log.Debug().Str("patchType", "redirectSource").Str(a.Kind.ShortName(), a.GetLongName()).Msg("Found helm chart")
		return nil
	}

	// Check repoURL
	repoURL, ok := source["repoURL"].(string)
	if !ok {
		log.Debug().Str("patchType", "redirectSource").Str(a.Kind.ShortName(), a.GetLongName()).Msg("Found no 'repoURL' under source")
		return nil
	}

	if !containsIgnoreCase(repoURL, repo) {
		log.Debug().Str("patchType", "redirectSource").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("Skipping source: %s (repoURL does not match %s)", repoURL, repo)
		return nil
	}

	// Get or set targetRevision
	targetRev, ok := source["targetRevision"].(string)
	if !ok {
		defaultTargetRev := "HEAD"
		log.Debug().Str("patchType", "redirectSource").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("Found no 'targetRevision' under source. Defaulting to '%s'", defaultTargetRev)
		targetRev = defaultTargetRev
		source["targetRevision"] = targetRev
	}

	if targetRev == branch {
		log.Debug().Str("patchType", "redirectSource").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("Target revision is already '%s'. Skipping redirect.", branch)
		return nil
	}

	shouldRedirect := len(redirectRevisions) == 0 || slices.Contains(redirectRevisions, targetRev)

	if shouldRedirect {
		log.Debug().Str("patchType", "redirectSource").Str(a.Kind.ShortName(), a.GetLongName()).Msgf("Redirecting targetRevision from '%s' to '%s'", targetRev, branch)
		source["targetRevision"] = branch
	}

	return nil
}

// RedirectGenerators updates the git generator targetRevision to point to the specified branch
func (a *ArgoResource) RedirectGenerators(repo, branch string, redirectRevisions []string) error {
	// Only process ApplicationSets
	if a.Kind != ApplicationSet || a.Yaml == nil {
		return nil
	}

	// Get generators
	generators, found, err := unstructured.NestedSlice(a.Yaml.Object, "spec", "generators")
	if err != nil || !found {
		log.Debug().Str("patchType", "redirectGenerators").Str("branch", branch).Str(a.Kind.ShortName(), a.GetLongName()).Msgf("no 'spec.generators' key found in ApplicationSet: %s", a.Name)
		return nil
	}

	// Process generators
	if err := a.processGenerators(generators, repo, branch, redirectRevisions, "spec.generators", 0); err != nil {
		log.Error().Str("patchType", "redirectGenerators").Str("branch", branch).Str(a.Kind.ShortName(), a.GetLongName()).Err(err).Msg("error processing generators")
		return err
	}

	// Set back updated generators
	return unstructured.SetNestedSlice(a.Yaml.Object, generators, "spec", "generators")
}

// processGenerators processes a slice of generators recursively
func (a *ArgoResource) processGenerators(generators []any, repo, branch string, redirectRevisions []string, parent string, level int) error {
	// Limit nesting level to prevent infinite recursion
	if level > 2 {
		return fmt.Errorf("too many levels of nested matrix generators in ApplicationSet: %s", a.Name)
	}

	// Process each generator
	for i, genInterface := range generators {
		gen, ok := genInterface.(map[string]any)
		if !ok {
			continue
		}

		// Check for matrix generator
		if matrixGen, hasMatrix := gen["matrix"]; hasMatrix {
			matrixMap, ok := matrixGen.(map[string]any)
			if !ok {
				continue
			}

			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Str("patchType", "redirectGenerators").Msg("Matrix generator found")

			// Get nested generators
			nestedGens, hasNestedGens := matrixMap["generators"]
			if !hasNestedGens {
				continue
			}

			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Str("patchType", "redirectGenerators").Msg("Nested generators found")

			nestedGenSlice, ok := nestedGens.([]any)
			if !ok {
				continue
			}

			// Make sure there are at most 2 child generators
			if len(nestedGenSlice) > 2 {
				return fmt.Errorf("only 2 child generators are allowed for matrix generator '%s' in ApplicationSet: %s",
					fmt.Sprintf("%s[%d].matrix", parent, i), a.Name)
			}

			// Process nested generators
			matrixParent := fmt.Sprintf("%s[%d].matrix.generators", parent, i)
			if err := a.processGenerators(nestedGenSlice, repo, branch, redirectRevisions, matrixParent, level+1); err != nil {
				return err
			}

			continue
		}

		// Check for merge generator
		if mergeGen, hasMerge := gen["merge"]; hasMerge {
			mergeMap, ok := mergeGen.(map[string]any)
			if !ok {
				continue
			}

			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Str("patchType", "redirectGenerators").Msg("Merge generator found")

			// Get nested generators
			nestedGens, hasNestedGens := mergeMap["generators"]
			if !hasNestedGens {
				continue
			}

			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Str("patchType", "redirectGenerators").Msg("Nested generators found in merge")

			nestedGenSlice, ok := nestedGens.([]any)
			if !ok {
				continue
			}

			// Process nested generators
			mergeParent := fmt.Sprintf("%s[%d].merge.generators", parent, i)
			if err := a.processGenerators(nestedGenSlice, repo, branch, redirectRevisions, mergeParent, level+1); err != nil {
				return err
			}

			continue
		}

		// Check for git generator
		if gitGen, hasGit := gen["git"]; hasGit {
			gitMap, ok := gitGen.(map[string]any)
			if !ok {
				continue
			}

			log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Str("patchType", "redirectGenerators").Msg("Git generator found")

			// Check repoURL
			repoURL, ok := gitMap["repoURL"].(string)
			if !ok || !containsIgnoreCase(repoURL, repo) {
				log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Str("patchType", "redirectGenerators").Msgf("Skipping source: %s (repoURL does not match %s)", repoURL, repo)
				continue
			}

			// Check revision
			revision, ok := gitMap["revision"].(string)
			if !ok {
				continue
			}

			// Check if we should redirect this revision
			shouldRedirect := len(redirectRevisions) == 0 || slices.Contains(redirectRevisions, revision)
			if shouldRedirect {
				gitMap["revision"] = branch
				log.Debug().Str(a.Kind.ShortName(), a.GetLongName()).Str("patchType", "redirectGenerators").Str("branch", branch).
					Msgf("Redirecting revision from '%s' to '%s' in %s[%d].git", revision, branch, parent, i)
			}
		}
	}

	return nil
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
