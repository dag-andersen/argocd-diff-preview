package argoapplicaiton

import (
	"fmt"
	"strings"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	yamlutil "github.com/dag-andersen/argocd-diff-preview/pkg/yaml"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

// ArgoResource represents an Argo CD Application or ApplicationSet
type ArgoResource struct {
	Yaml     *yaml.Node
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
	yamlutil.SetYamlValue(a.Yaml, []string{"metadata", "namespace"}, namespace)
	return nil
}

// SetProjectToDefault sets the project to "default"
func (a *ArgoResource) SetProjectToDefault() error {
	spec := a.getAppSpec()
	if spec == nil {
		log.Debug().Msgf("no 'spec' key found in Application: %s", a.Name)
	}

	project := yamlutil.GetYamlValue(spec, []string{"project"})
	if project == nil {
		log.Debug().Msgf("no 'spec.project' key found in Application: %s (file: %s)",
			a.Name, a.FileName)
	}

	yamlutil.SetYamlValue(spec, []string{"project"}, "default")
	return nil
}

// PointDestinationToInCluster updates the destination to point to the in-cluster service
func (a *ArgoResource) PointDestinationToInCluster() error {
	spec := a.getAppSpec()
	if spec == nil {
		log.Debug().Msgf("no 'spec' key found in Application: %s", a.Name)
	}

	dest := yamlutil.GetYamlValue(spec, []string{"destination"})
	if dest == nil {
		log.Debug().Msgf("no 'spec.destination' key found in Application: %s (file: %s)",
			a.Name, a.FileName)
	}

	yamlutil.SetYamlValue(dest, []string{"name"}, "in-cluster")
	yamlutil.RemoveYamlValue(dest, []string{"server"})
	return nil
}

// RemoveSyncPolicy removes the syncPolicy from the resource
func (a *ArgoResource) RemoveSyncPolicy() error {
	if a.Yaml == nil {
		log.Warn().Str("patchType", "removeSyncPolicy").Str("file", a.FileName).Msgf("⚠️ Can't remove 'syncPolicy' because YAML is nil")
		return nil
	}

	if a.Yaml.Content == nil {
		log.Warn().Str("patchType", "removeSyncPolicy").Str("file", a.FileName).Msgf("⚠️ Can't remove 'syncPolicy' because YAML content is nil")
		return nil
	}

	spec := yamlutil.GetYamlValue(a.Yaml, []string{"spec"})
	if spec == nil {
		log.Warn().Str("patchType", "removeSyncPolicy").Str("file", a.FileName).Msgf("⚠️ Can't remove 'syncPolicy' because 'spec' key not found")
		return nil
	}

	yamlutil.RemoveYamlValue(spec, []string{"syncPolicy"})
	return nil
}

// RedirectSources updates the source/sources targetRevision to point to the specified branch
func (a *ArgoResource) RedirectSources(repo, branch string, redirectRevisions []string) error {
	spec := a.getAppSpec()
	if spec == nil {
		log.Warn().Str("patchType", "redirectSources").Str("file", a.FileName).Msgf("⚠️ No 'spec' key found in Application: %s", a.Name)
	}

	// Handle single source
	source := yamlutil.GetYamlValue(spec, []string{"source"})
	if source != nil {
		if err := a.redirectSource(source, repo, branch, redirectRevisions); err != nil {
			return err
		}
		return nil
	}

	// Handle multiple sources
	sources := yamlutil.GetYamlValue(spec, []string{"sources"})
	if sources != nil {
		for _, src := range sources.Content {
			if err := a.redirectSource(src, repo, branch, redirectRevisions); err != nil {
				return err
			}
		}
		return nil
	}

	log.Debug().Str("patchType", "redirectSources").Str("file", a.FileName).Msgf("no 'spec.source' or 'spec.sources' key found in Application: %s",
		a.Name)

	return nil
}

// RedirectGenerators updates the git generator targetRevision to point to the specified branch
func (a *ArgoResource) RedirectGenerators(repo, branch string, redirectRevisions []string) error {
	// Only process ApplicationSets
	if a.Kind != ApplicationSet {
		return nil
	}

	spec := a.getRootSpec()
	if spec == nil {
		log.Warn().Str("patchType", "redirectGenerators").Str("file", a.FileName).Msgf("⚠️ No 'spec' key found in ApplicationSet: %s", a.Name)
		return nil
	}

	generators := yamlutil.GetYamlValue(spec, []string{"generators"})
	if generators == nil {
		log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("no 'spec.generators' key found in ApplicationSet: %s", a.Name)
		return nil
	}

	log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("found %d generators in ApplicationSet: %s", len(generators.Content), a.Name)

	parent := "spec"
	if err := a.redirectGenerators(generators, repo, branch, redirectRevisions, parent, 0); err != nil {
		return err
	}

	return nil
}

// Helper functions
func (a *ArgoResource) redirectGenerators(generators *yaml.Node, repo, branch string, redirectRevisions []string, parent string, level int) error {
	// Process each generator
	for index, generator := range generators.Content {
		if generator.Kind != yaml.MappingNode {
			continue
		}

		// A restriction of ArgoCD Matrix generators, only 2 child generators are allowed
		if level > 0 && index > 1 {
			return fmt.Errorf("only 2 child generators are allowed for matrix generator '%s' in ApplicationSet: %s", parent, a.Name)
		}

		// Look for matrix generator
		matrixGen := yamlutil.GetYamlValue(generator, []string{"matrix"})
		if matrixGen != nil {
			matrixParent := fmt.Sprintf("%s.generators[%d].matrix", parent, index)
			log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("'%s' key found in ApplicationSet: %s", matrixParent, a.Name)

			if err := a.redirectMatrixGenerators(matrixGen, repo, branch, redirectRevisions, matrixParent, level+1); err != nil {
				return err
			}
			continue
		}

		// Look for git generator
		gitGen := yamlutil.GetYamlValue(generator, []string{"git"})
		if gitGen == nil {
			log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("no '%s.generators[%d].git' key found in ApplicationSet: %s", parent, index, a.Name)
			continue
		}

		// Check repoURL
		repoURL := yamlutil.GetYamlValue(gitGen, []string{"repoURL"})
		if repoURL == nil || !containsIgnoreCase(repoURL.Value, repo) {
			log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("no '%s.generators[%d].git.repoURL' key found in ApplicationSet: %s", parent, index, a.Name)
			continue
		}

		// Check targetRevision
		revision := yamlutil.GetYamlValue(gitGen, []string{"revision"})
		if revision == nil {
			log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("no '%s.generators[%d].git.revision' key found in ApplicationSet: %s", parent, index, a.Name)
			continue
		}

		// Check if we should redirect this revision
		shouldRedirect := len(redirectRevisions) == 0 || contains(redirectRevisions, revision.Value)
		if shouldRedirect {
			yamlutil.SetYamlValue(gitGen, []string{"revision"}, branch)
			log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf(
				"Patched git generators in ApplicationSet: %s",
				a.Name,
			)
		}
	}

	return nil
}

// Helper functions
func (a *ArgoResource) redirectMatrixGenerators(matrix *yaml.Node, repo, branch string, redirectRevisions []string, parent string, level int) error {
	// A restriction of ArgoCD Matrix gnenerators, only 2 levels of nested matrix generators are allowed
	if level > 2 {
		return fmt.Errorf("too many levels of nested matrix generators in ApplicationSet: %s", a.Name)
	}

	// Look for child generators
	childGen := yamlutil.GetYamlValue(matrix, []string{"generators"})
	if childGen == nil {
		log.Debug().Str("patchType", "redirectMatrixGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("no '%s.generators' key found in ApplicationSet: %s", parent, a.Name)
		return nil
	}

	if err := a.redirectGenerators(childGen, repo, branch, redirectRevisions, parent, level); err != nil {
		return err
	}

	return nil
}

// Helper functions
func (a *ArgoResource) redirectSource(source *yaml.Node, repo, branch string, redirectRevisions []string) error {
	if yamlutil.GetYamlValue(source, []string{"chart"}) != nil {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msg("Found helm chart")
		return nil
	}

	repoURL := yamlutil.GetYamlValue(source, []string{"repoURL"})
	if repoURL == nil {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msg("Found no 'repoURL' under spec.source")
		return nil
	}

	if !containsIgnoreCase(repoURL.Value, repo) {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msgf("Skipping source: %s (repoURL does not match %s)", repoURL.Value, repo)
		return nil
	}

	targetRev := yamlutil.GetYamlValue(source, []string{"targetRevision"})
	if targetRev == nil {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msg("Found no 'targetRevision' under spec.source")
		targetRev = &yaml.Node{Value: "HEAD"}
		yamlutil.SetYamlValue(source, []string{"targetRevision"}, "HEAD")
	}

	shouldRedirect := len(redirectRevisions) == 0 ||
		contains(redirectRevisions, targetRev.Value)

	if shouldRedirect {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msgf("Redirecting targetRevision from %s to %s", targetRev.Value, branch)
		yamlutil.SetYamlValue(source, []string{"targetRevision"}, branch)
	}

	return nil
}

// Helper functions for YAML manipulation
func (a *ArgoResource) getAppSpec() *yaml.Node {
	switch a.Kind {
	case Application:
		return yamlutil.GetYamlValue(a.Yaml, []string{"spec"})
	case ApplicationSet:
		return yamlutil.GetYamlValue(a.Yaml, []string{"spec", "template", "spec"})
	default:
		return nil
	}
}

func (a *ArgoResource) getRootSpec() *yaml.Node {
	return yamlutil.GetYamlValue(a.Yaml, []string{"spec"})
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
