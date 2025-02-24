package types

import (
	"fmt"
	"log"
	"regexp"
	"strings"

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
func (a *ArgoResource) SetNamespace(namespace string) *ArgoResource {
	setYamlValue(a.Yaml, []string{"metadata", "namespace"}, namespace)
	return a
}

// SetProjectToDefault sets the project to "default"
func (a *ArgoResource) SetProjectToDefault() (*ArgoResource, error) {
	spec := a.getSpec()
	if spec == nil {
		return nil, fmt.Errorf("no 'spec' key found in Application: %s", a.Name)
	}

	project := getYamlValue(spec, []string{"project"})
	if project == nil {
		return nil, fmt.Errorf("no 'spec.project' key found in Application: %s (file: %s)",
			a.Name, a.FileName)
	}

	setYamlValue(spec, []string{"project"}, "default")
	return a, nil
}

// PointDestinationToInCluster updates the destination to point to the in-cluster service
func (a *ArgoResource) PointDestinationToInCluster() (*ArgoResource, error) {
	spec := a.getSpec()
	if spec == nil {
		return nil, fmt.Errorf("no 'spec' key found in Application: %s", a.Name)
	}

	dest := getYamlValue(spec, []string{"destination"})
	if dest == nil {
		return nil, fmt.Errorf("no 'spec.destination' key found in Application: %s (file: %s)",
			a.Name, a.FileName)
	}

	setYamlValue(dest, []string{"name"}, "in-cluster")
	removeYamlValue(dest, []string{"server"})
	return a, nil
}

// RemoveSyncPolicy removes the syncPolicy from the resource
func (a *ArgoResource) RemoveSyncPolicy() (*ArgoResource, error) {
	if a.Yaml == nil {
		log.Printf("Can't remove 'syncPolicy' because YAML is nil in file: %s",
			a.FileName)
		return a, nil
	}

	if a.Yaml.Content == nil {
		log.Printf("Can't remove 'syncPolicy' because YAML content is nil in file: %s",
			a.FileName)
		return a, nil
	}

	// Debug logging
	log.Printf("YAML Content length: %d for file: %s", len(a.Yaml.Content), a.FileName)
	log.Printf("YAML Kind: %v", a.Yaml.Kind)

	// Print the first level keys
	for i := 0; i < len(a.Yaml.Content); i += 2 {
		if i+1 < len(a.Yaml.Content) {
			log.Printf("Key at %d: %s", i/2, a.Yaml.Content[i].Value)
		}
	}

	spec := getYamlValue(a.Yaml, []string{"spec"})
	if spec == nil {
		log.Printf("Can't remove 'syncPolicy' because 'spec' key not found in file: %s",
			a.FileName)
		return a, nil
	}

	removeYamlValue(spec, []string{"syncPolicy"})
	return a, nil
}

// RedirectSources updates the source/sources targetRevision to point to the specified branch
func (a *ArgoResource) RedirectSources(repo, branch string, redirectRevisions []string) (*ArgoResource, error) {
	spec := a.getSpec()
	if spec == nil {
		return nil, fmt.Errorf("no 'spec' key found in Application: %s", a.Name)
	}

	// Handle single source
	source := getYamlValue(spec, []string{"source"})
	if source != nil {
		if err := a.redirectSource(source, repo, branch, redirectRevisions); err != nil {
			return nil, err
		}
		return a, nil
	}

	// Handle multiple sources
	sources := getYamlValue(spec, []string{"sources"})
	if sources != nil {
		for _, src := range sources.Content {
			if err := a.redirectSource(src, repo, branch, redirectRevisions); err != nil {
				return nil, err
			}
		}
		return a, nil
	}

	return nil, fmt.Errorf("no 'spec.source' or 'spec.sources' key found in Application: %s",
		a.Name)
}

// RedirectGenerators updates the git generator targetRevision to point to the specified branch
func (a *ArgoResource) RedirectGenerators(repo, branch string, redirectRevisions []string) (*ArgoResource, error) {
	// Only process ApplicationSets
	if a.Kind != ApplicationSet {
		return a, nil
	}

	spec := a.getSpec()
	if spec == nil {
		return nil, fmt.Errorf("no 'spec' key found in ApplicationSet: %s", a.Name)
	}

	generators := getYamlValue(spec, []string{"generators"})
	if generators == nil {
		return a, nil
	}

	// Process each generator
	for _, generator := range generators.Content {
		if generator.Kind != yaml.MappingNode {
			continue
		}

		// Look for git generator
		gitGen := getYamlValue(generator, []string{"git"})
		if gitGen == nil {
			continue
		}

		// Check repoURL
		repoURL := getYamlValue(gitGen, []string{"repoURL"})
		if repoURL == nil || !containsIgnoreCase(repoURL.Value, repo) {
			continue
		}

		// Check targetRevision
		targetRevision := getYamlValue(gitGen, []string{"targetRevision"})
		if targetRevision == nil {
			continue
		}

		// Check if we should redirect this revision
		shouldRedirect := len(redirectRevisions) == 0 || contains(redirectRevisions, targetRevision.Value)
		if shouldRedirect {
			setYamlValue(gitGen, []string{"targetRevision"}, branch)
			log.Printf(
				"Patched git generators in ApplicationSet: %s in file: %s",
				a.Name,
				a.FileName,
			)
		}
	}

	return a, nil
}

// Filter checks if the application matches the given selectors and watches the given files
func (a *ArgoResource) Filter(
	selectors []Selector,
	filesChanged []string,
	ignoreInvalidWatchPattern bool,
) *ArgoResource {
	// Check selectors
	if len(selectors) > 0 {
		metadata := getYamlValue(a.Yaml, []string{"metadata"})
		if metadata == nil {
			return nil
		}

		labels := getYamlValue(metadata, []string{"labels"})
		if labels == nil {
			return nil
		}

		for _, selector := range selectors {
			labelValue := getYamlValue(labels, []string{selector.Key})
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
		metadata := getYamlValue(a.Yaml, []string{"metadata"})
		if metadata == nil {
			return nil
		}

		annotations := getYamlValue(metadata, []string{"annotations"})
		if annotations == nil {
			return nil
		}

		watchPattern := getYamlValue(annotations, []string{AnnotationWatchPattern})
		if watchPattern == nil {
			return nil
		}

		pattern := watchPattern.Value
		if pattern == "" {
			return nil
		}

		regex, err := regexp.Compile(pattern)
		if err != nil {
			if !ignoreInvalidWatchPattern {
				log.Printf("⚠️ Invalid watch pattern '%s' in file: %s", pattern, a.FileName)
				return nil
			}
			log.Printf("⚠️ Ignoring invalid watch pattern '%s' in file: %s", pattern, a.FileName)
			return a
		}

		for _, file := range filesChanged {
			if regex.MatchString(file) {
				return a
			}
		}
		return nil
	}

	return a
}

// Helper functions

func (a *ArgoResource) getSpec() *yaml.Node {
	switch a.Kind {
	case Application:
		return getYamlValue(a.Yaml, []string{"spec"})
	case ApplicationSet:
		return getYamlValue(a.Yaml, []string{"spec", "template", "spec"})
	default:
		return nil
	}
}

func (a *ArgoResource) redirectSource(source *yaml.Node, repo, branch string, redirectRevisions []string) error {
	// Skip if it's a helm chart
	if getYamlValue(source, []string{"chart"}) != nil {
		return nil
	}

	repoURL := getYamlValue(source, []string{"repoURL"})
	if repoURL == nil || !containsIgnoreCase(repoURL.Value, repo) {
		log.Printf("Found no 'repoURL' under spec.source in file: %s", a.FileName)
		return nil
	}

	targetRev := getYamlValue(source, []string{"targetRevision"})
	if targetRev == nil {
		targetRev = &yaml.Node{Value: "HEAD"}
	}

	shouldRedirect := len(redirectRevisions) == 0 ||
		contains(redirectRevisions, targetRev.Value)

	if shouldRedirect {
		setYamlValue(source, []string{"targetRevision"}, branch)
	}

	return nil
}

// Helper functions for YAML manipulation
func getYamlValue(node *yaml.Node, path []string) *yaml.Node {
	if node == nil || len(path) == 0 {
		return node
	}

	if node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == path[0] {
			if len(path) == 1 {
				return node.Content[i+1]
			}
			return getYamlValue(node.Content[i+1], path[1:])
		}
	}
	return nil
}

func setYamlValue(node *yaml.Node, path []string, value string) {
	if node == nil || len(path) == 0 {
		return
	}

	if node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == path[0] {
			if len(path) == 1 {
				node.Content[i+1].Value = value
				return
			}
			setYamlValue(node.Content[i+1], path[1:], value)
			return
		}
	}
}

func removeYamlValue(node *yaml.Node, path []string) {
	if node == nil || len(path) == 0 {
		return
	}

	if node.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == path[0] {
			if len(path) == 1 {
				// Only remove if we found the exact path
				node.Content = append(node.Content[:i], node.Content[i+2:]...)
				return
			}
			// Continue searching deeper
			removeYamlValue(node.Content[i+1], path[1:])
			return
		}
	}
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
		log.Printf("⚠️ No content found in file: %s", resource.FileName)
		return nil
	}

	kind := getYamlValue(resource.Yaml.Content[0], []string{"kind"})
	if kind == nil {
		log.Printf("⚠️ No 'kind' field found in file: %s", resource.FileName)
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

	name := getYamlValue(resource.Yaml.Content[0], []string{"metadata", "name"})
	if name == nil {
		log.Printf("⚠️ No 'metadata.name' field found in file: %s", resource.FileName)
		return nil
	}

	return &ArgoResource{
		Yaml:     resource.Yaml.Content[0],
		Kind:     appKind,
		Name:     name.Value,
		FileName: resource.FileName,
	}
}
