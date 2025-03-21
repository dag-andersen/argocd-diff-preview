package argoapplicaiton

import (
	"fmt"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// ArgoResource represents an Argo CD Application or ApplicationSet
type ArgoResource struct {
	Yaml     *unstructured.Unstructured
	Kind     ApplicationKind
	Name     string
	FileName string
}

// AsString returns the YAML representation of the resource
func (a *ArgoResource) AsString() (string, error) {
	bytes, err := yaml.Marshal(a.Yaml)
	if err != nil {
		return "", fmt.Errorf("failed to marshal yaml: %w", err)
	}
	return string(bytes), nil
}

// SetNamespace sets the namespace of the resource
func (a *ArgoResource) SetNamespace(namespace string) error {
	a.Yaml.SetNamespace(namespace)
	return nil
}

// SetProjectToDefault sets the project to "default"
func (a *ArgoResource) SetProjectToDefault() error {
	if a.Yaml == nil {
		log.Debug().Msgf("no YAML for Application: %s", a.Name)
		return nil
	}

	switch a.Kind {
	case Application:
		if _, found, _ := unstructured.NestedString(a.Yaml.Object, "spec", "project"); !found {
			log.Debug().Msgf("no 'spec.project' key found in Application: %s (file: %s)",
				a.Name, a.FileName)
		}
		unstructured.SetNestedField(a.Yaml.Object, "default", "spec", "project")
	case ApplicationSet:
		if _, found, _ := unstructured.NestedString(a.Yaml.Object, "spec", "template", "spec", "project"); !found {
			log.Debug().Msgf("no 'spec.template.spec.project' key found in ApplicationSet: %s (file: %s)",
				a.Name, a.FileName)
		}
		unstructured.SetNestedField(a.Yaml.Object, "default", "spec", "template", "spec", "project")
	}

	return nil
}

// PointDestinationToInCluster updates the destination to point to the in-cluster service
func (a *ArgoResource) PointDestinationToInCluster() error {
	if a.Yaml == nil {
		log.Debug().Msgf("no YAML for Application: %s", a.Name)
		return nil
	}

	var destPath []string
	switch a.Kind {
	case Application:
		destPath = []string{"spec", "destination"}
	case ApplicationSet:
		destPath = []string{"spec", "template", "spec", "destination"}
	default:
		return nil
	}

	// Check if destination exists
	destMap, found, _ := unstructured.NestedMap(a.Yaml.Object, destPath...)
	if !found {
		log.Debug().Msgf("no '%s' key found in %s: %s (file: %s)",
			strings.Join(destPath, "."), a.Kind, a.Name, a.FileName)
		return nil
	}

	// Update destination
	destMap["name"] = "in-cluster"
	delete(destMap, "server")

	// Set it back
	unstructured.SetNestedMap(a.Yaml.Object, destMap, destPath...)

	return nil
}

// RemoveSyncPolicy removes the syncPolicy from the resource
func (a *ArgoResource) RemoveSyncPolicy() error {
	if a.Yaml == nil {
		log.Warn().Str("patchType", "removeSyncPolicy").Str("file", a.FileName).Msgf("⚠️ Can't remove 'syncPolicy' because YAML is nil")
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
		log.Warn().Str("patchType", "removeSyncPolicy").Str("file", a.FileName).Msgf("⚠️ Can't remove 'syncPolicy' because spec not found")
		return nil
	}

	// Remove syncPolicy
	delete(specMap, "syncPolicy")

	// Set it back
	unstructured.SetNestedMap(a.Yaml.Object, specMap, specPath...)

	return nil
}

// RedirectSources updates the source/sources targetRevision to point to the specified branch
func (a *ArgoResource) RedirectSources(repo, branch string, redirectRevisions []string) error {
	if a.Yaml == nil {
		log.Warn().Str("patchType", "redirectSources").Str("file", a.FileName).Msgf("⚠️ No YAML for Application: %s", a.Name)
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
		log.Warn().Str("patchType", "redirectSources").Str("file", a.FileName).Msgf("⚠️ No spec found in %s: %s", a.Kind, a.Name)
		return nil
	}

	// Handle single source
	if source, ok := specMap["source"].(map[string]interface{}); ok {
		if err := a.redirectSourceMap(source, repo, branch, redirectRevisions); err != nil {
			return err
		}
	}

	// Handle multiple sources
	if sourcesInterface, ok := specMap["sources"]; ok {
		if sources, ok := sourcesInterface.([]interface{}); ok {
			for _, sourceInterface := range sources {
				if source, ok := sourceInterface.(map[string]interface{}); ok {
					if err := a.redirectSourceMap(source, repo, branch, redirectRevisions); err != nil {
						return err
					}
				}
			}
		}
	}

	// Set updated spec back
	unstructured.SetNestedMap(a.Yaml.Object, specMap, specPath...)

	return nil
}

// Helper function to redirect a single source
func (a *ArgoResource) redirectSourceMap(source map[string]interface{}, repo, branch string, redirectRevisions []string) error {
	// Skip helm charts
	if _, hasChart := source["chart"]; hasChart {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msg("Found helm chart")
		return nil
	}

	// Check repoURL
	repoURL, ok := source["repoURL"].(string)
	if !ok {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msg("Found no 'repoURL' under source")
		return nil
	}

	if !containsIgnoreCase(repoURL, repo) {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msgf("Skipping source: %s (repoURL does not match %s)", repoURL, repo)
		return nil
	}

	// Get or set targetRevision
	targetRev, ok := source["targetRevision"].(string)
	if !ok {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msg("Found no 'targetRevision' under source")
		targetRev = "HEAD"
		source["targetRevision"] = targetRev
	}

	shouldRedirect := len(redirectRevisions) == 0 || contains(redirectRevisions, targetRev)

	if shouldRedirect {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msgf("Redirecting targetRevision from %s to %s", targetRev, branch)
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
		log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("no 'spec.generators' key found in ApplicationSet: %s", a.Name)
		return nil
	}

	// Process generators
	if err := a.processGenerators(generators, repo, branch, redirectRevisions, "spec.generators", 0); err != nil {
		log.Error().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Err(err).Msg("error processing generators")
		return err
	}

	// Set back updated generators
	return unstructured.SetNestedSlice(a.Yaml.Object, generators, "spec", "generators")
}

// processGenerators processes a slice of generators recursively
func (a *ArgoResource) processGenerators(generators []interface{}, repo, branch string, redirectRevisions []string, parent string, level int) error {
	// Limit nesting level to prevent infinite recursion
	if level > 2 {
		return fmt.Errorf("too many levels of nested matrix generators in ApplicationSet: %s", a.Name)
	}

	// Process each generator
	for i, genInterface := range generators {
		gen, ok := genInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Check for matrix generator
		if matrixGen, hasMatrix := gen["matrix"]; hasMatrix {
			matrixMap, ok := matrixGen.(map[string]interface{})
			if !ok {
				continue
			}

			log.Debug().Str("file", a.FileName).Str("patchType", "redirectGenerators").Msg("Matrix generator found")

			// Get nested generators
			nestedGens, hasNestedGens := matrixMap["generators"]
			if !hasNestedGens {
				continue
			}

			log.Debug().Str("file", a.FileName).Str("patchType", "redirectGenerators").Msg("Nested generators found")

			nestedGenSlice, ok := nestedGens.([]interface{})
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

		// Check for git generator
		if gitGen, hasGit := gen["git"]; hasGit {
			gitMap, ok := gitGen.(map[string]interface{})
			if !ok {
				continue
			}

			log.Debug().Str("file", a.FileName).Str("patchType", "redirectGenerators").Msg("Git generator found")

			// Check repoURL
			repoURL, ok := gitMap["repoURL"].(string)
			if !ok || !containsIgnoreCase(repoURL, repo) {
				log.Debug().Str("file", a.FileName).Str("patchType", "redirectGenerators").Msgf("Skipping source: %s (repoURL does not match %s)", repoURL, repo)
				continue
			}

			// Check revision
			revision, ok := gitMap["revision"].(string)
			if !ok {
				continue
			}

			// Check if we should redirect this revision
			shouldRedirect := len(redirectRevisions) == 0 || contains(redirectRevisions, revision)
			if shouldRedirect {
				gitMap["revision"] = branch
				log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).
					Msgf("Redirecting revision from %s to %s in %s[%d].git", revision, branch, parent, i)
			}
		}
	}

	return nil
}

func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// WriteApplications writes applications to YAML files in the specified folder
func WriteApplications(
	apps []ArgoResource,
	branch *git.Branch,
	folder string,
) error {
	filePath := fmt.Sprintf("%s/%s.yaml", folder, branch.FolderName())
	log.Info().Msgf("💾 Writing %d Applications from '%s' to ./%s",
		len(apps), branch.Name, filePath)

	yaml := ApplicationsToString(apps)
	if err := utils.WriteFile(filePath, yaml); err != nil {
		return fmt.Errorf("failed to write %s apps: %w", branch.Type(), err)
	}

	return nil
}
