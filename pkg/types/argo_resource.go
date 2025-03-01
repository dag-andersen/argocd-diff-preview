package types

import (
	"fmt"
	"regexp"
	"strings"

	yamlutil "github.com/argocd-diff-preview/argocd-diff-preview/pkg/yaml"
	"github.com/rs/zerolog/log"

	"gopkg.in/yaml.v3"
)

const (
	AnnotationWatchPattern = "argocd-diff-preview/watch-pattern"
	AnnotationIgnore       = "argocd-diff-preview/ignore"
)

// ApplicationKind represents the type of Argo CD application
type ApplicationKind int

const (
	Application ApplicationKind = iota
	ApplicationSet
)

// String returns the string representation of ApplicationKind
func (k ApplicationKind) String() string {
	switch k {
	case Application:
		return "Application"
	case ApplicationSet:
		return "ApplicationSet"
	default:
		return "Unknown"
	}
}

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
		log.Warn().Str("patchType", "removeSyncPolicy").Str("file", a.FileName).Msgf("Can't remove 'syncPolicy' because YAML is nil")
		return nil
	}

	if a.Yaml.Content == nil {
		log.Warn().Str("patchType", "removeSyncPolicy").Str("file", a.FileName).Msgf("Can't remove 'syncPolicy' because YAML content is nil")
		return nil
	}

	spec := yamlutil.GetYamlValue(a.Yaml, []string{"spec"})
	if spec == nil {
		log.Warn().Str("patchType", "removeSyncPolicy").Str("file", a.FileName).Msgf("Can't remove 'syncPolicy' because 'spec' key not found")
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

	// Process each generator
	for index, generator := range generators.Content {
		if generator.Kind != yaml.MappingNode {
			continue
		}

		// Look for git generator
		gitGen := yamlutil.GetYamlValue(generator, []string{"git"})
		if gitGen == nil {
			log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("no 'spec.generators[%d].git' key found in ApplicationSet: %s", index, a.Name)
			continue
		}

		// Check repoURL
		repoURL := yamlutil.GetYamlValue(gitGen, []string{"repoURL"})
		if repoURL == nil || !containsIgnoreCase(repoURL.Value, repo) {
			log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("no 'spec.generators[%d].git.repoURL' key found in ApplicationSet: %s", index, a.Name)
			continue
		}

		// Check targetRevision
		revision := yamlutil.GetYamlValue(gitGen, []string{"revision"})
		if revision == nil {
			log.Debug().Str("patchType", "redirectGenerators").Str("file", a.FileName).Str("branch", branch).Msgf("no 'spec.generators[%d].git.revision' key found in ApplicationSet: %s", index, a.Name)
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

// TODO: should ignoreInvalidWatchPattern throw error or return nil?

// Filter checks if the application matches the given selectors and watches the given files
func (a *ArgoResource) Filter(
	selectors []Selector,
	filesChanged []string,
	ignoreInvalidWatchPattern bool,
) *ArgoResource {
	// Check selectors
	if len(selectors) > 0 {
		metadata := yamlutil.GetYamlValue(a.Yaml, []string{"metadata"})
		if metadata == nil {
			return nil
		}

		labels := yamlutil.GetYamlValue(metadata, []string{"labels"})
		if labels == nil {
			return nil
		}

		for _, selector := range selectors {
			labelValue := yamlutil.GetYamlValue(labels, []string{selector.Key})
			if labelValue == nil {
				return nil
			}

			matches := labelValue.Value == selector.Value
			if (selector.Operator == Eq && !matches) || (selector.Operator == Ne && matches) {
				return nil
			}
		}
	}

	// Check files changed
	if len(filesChanged) > 0 {

		log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("checking files changed: %v", filesChanged)

		metadata := yamlutil.GetYamlValue(a.Yaml, []string{"metadata"})
		if metadata == nil {
			return nil
		}

		annotations := yamlutil.GetYamlValue(metadata, []string{"annotations"})
		if annotations == nil {
			return nil
		}

		watchPattern := yamlutil.GetYamlValue(annotations, []string{AnnotationWatchPattern})
		if watchPattern == nil {
			log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("no watch pattern annotation found. Skipping")
			return nil
		}

		patternList := strings.TrimSpace(watchPattern.Value)
		if patternList == "" {
			log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("no watch pattern value found. Skipping")
			return nil
		}

		patterns := strings.Split(patternList, ",")

		for _, pattern := range patterns {
			pattern = strings.TrimSpace(pattern)

			log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("checking watch pattern: %s", pattern)

			if pattern == "" {
				log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("empty watch pattern found. Continuing")
				continue
			}

			regex, err := regexp.Compile(pattern)
			if err != nil {
				if !ignoreInvalidWatchPattern {
					log.Warn().Msgf("⚠️ Invalid watch pattern '%s' in file: %s", pattern, a.FileName)
					return nil
				}
				log.Warn().Msgf("⚠️ Ignoring invalid watch pattern '%s' in file: %s", pattern, a.FileName)
				return a
			}

			log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("watch pattern '%s' is valid. Checking files changed", pattern)

			for _, file := range filesChanged {
				if regex.MatchString(file) {
					log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("file '%s' matches watch pattern. Returning application", file)
					return a
				}
			}
		}

		log.Debug().Str("patchType", "filter").Str("file", a.FileName).Msgf("no files changed match watch pattern. Skipping")
		return nil
	}

	return a
}

// Helper functions

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

func (a *ArgoResource) redirectSource(source *yaml.Node, repo, branch string, redirectRevisions []string) error {

	if yamlutil.GetYamlValue(source, []string{"chart"}) != nil {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msg("Found helm chart")
		return nil
	}

	repoURL := yamlutil.GetYamlValue(source, []string{"repoURL"})
	if repoURL == nil || !containsIgnoreCase(repoURL.Value, repo) {
		log.Debug().Str("patchType", "redirectSource").Str("file", a.FileName).Msg("Found no 'repoURL' under spec.source")
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

// FromK8sResource creates an ArgoResource from a K8sResource
func FromK8sResource(resource *K8sResource) *ArgoResource {
	// Get the kind

	if resource.Yaml.Content == nil {
		log.Debug().Str("file", resource.FileName).Msg("No content found in file")
		return nil
	}

	kind := yamlutil.GetYamlValue(resource.Yaml.Content[0], []string{"kind"})
	if kind == nil {
		log.Debug().Str("file", resource.FileName).Msg("No 'kind' field found in file")
		return nil
	}

	// Check if it's an Argo CD resource
	var appKind ApplicationKind
	switch kind.Value {
	case "Application":
		appKind = Application
	case "ApplicationSet":
		appKind = ApplicationSet
	default:
		return nil
	}

	name := yamlutil.GetYamlValue(resource.Yaml.Content[0], []string{"metadata", "name"})
	if name == nil {
		log.Debug().Str("file", resource.FileName).Msg("No 'metadata.name' field found in file")
		return nil
	}

	return &ArgoResource{
		Yaml:     resource.Yaml.Content[0],
		Kind:     appKind,
		Name:     name.Value,
		FileName: resource.FileName,
	}
}

func ApplicationsToString(apps []ArgoResource) string {
	var yamlStrings []string
	for _, app := range apps {
		yamlStr, err := app.AsString()
		if err != nil {
			log.Debug().Err(err).Str("file", app.FileName).Msgf("Failed to convert app %s to YAML", app.Name)
			continue
		}
		// add a comment with the name of the file
		yamlStr = fmt.Sprintf("# File: %s\n%s", app.FileName, yamlStr)

		yamlStrings = append(yamlStrings, yamlStr)
	}
	return strings.Join(yamlStrings, "---\n")
}
