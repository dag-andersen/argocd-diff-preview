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
func (a *ArgoResource) SetNamespace(namespace string) error {
	setYamlValue(a.Yaml, []string{"metadata", "namespace"}, namespace)
	return nil
}

// SetProjectToDefault sets the project to "default"
func (a *ArgoResource) SetProjectToDefault() error {
	spec := a.getSpec()
	if spec == nil {
		return fmt.Errorf("no 'spec' key found in Application: %s", a.Name)
	}

	project := GetYamlValue(spec, []string{"project"})
	if project == nil {
		return fmt.Errorf("no 'spec.project' key found in Application: %s (file: %s)",
			a.Name, a.FileName)
	}

	setYamlValue(spec, []string{"project"}, "default")
	return nil
}

// PointDestinationToInCluster updates the destination to point to the in-cluster service
func (a *ArgoResource) PointDestinationToInCluster() error {
	spec := a.getSpec()
	if spec == nil {
		return fmt.Errorf("no 'spec' key found in Application: %s", a.Name)
	}

	dest := GetYamlValue(spec, []string{"destination"})
	if dest == nil {
		return fmt.Errorf("no 'spec.destination' key found in Application: %s (file: %s)",
			a.Name, a.FileName)
	}

	setYamlValue(dest, []string{"name"}, "in-cluster")
	removeYamlValue(dest, []string{"server"})
	return nil
}

// RemoveSyncPolicy removes the syncPolicy from the resource
func (a *ArgoResource) RemoveSyncPolicy() error {
	if a.Yaml == nil {
		log.Printf("Can't remove 'syncPolicy' because YAML is nil in file: %s",
			a.FileName)
		return nil
	}

	if a.Yaml.Content == nil {
		log.Printf("Can't remove 'syncPolicy' because YAML content is nil in file: %s",
			a.FileName)
		return nil
	}

	spec := GetYamlValue(a.Yaml, []string{"spec"})
	if spec == nil {
		log.Printf("Can't remove 'syncPolicy' because 'spec' key not found in file: %s",
			a.FileName)
		return nil
	}

	removeYamlValue(spec, []string{"syncPolicy"})
	return nil
}

// RedirectSources updates the source/sources targetRevision to point to the specified branch
func (a *ArgoResource) RedirectSources(repo, branch string, redirectRevisions []string) error {
	spec := a.getSpec()
	if spec == nil {
		return fmt.Errorf("no 'spec' key found in Application: %s", a.Name)
	}

	// Handle single source
	source := GetYamlValue(spec, []string{"source"})
	if source != nil {
		if err := a.redirectSource(source, repo, branch, redirectRevisions); err != nil {
			return err
		}
		return nil
	}

	// Handle multiple sources
	sources := GetYamlValue(spec, []string{"sources"})
	if sources != nil {
		for _, src := range sources.Content {
			if err := a.redirectSource(src, repo, branch, redirectRevisions); err != nil {
				return err
			}
		}
		return nil
	}

	return fmt.Errorf("no 'spec.source' or 'spec.sources' key found in Application: %s",
		a.Name)
}

// RedirectGenerators updates the git generator targetRevision to point to the specified branch
func (a *ArgoResource) RedirectGenerators(repo, branch string, redirectRevisions []string) error {
	// Only process ApplicationSets
	if a.Kind != ApplicationSet {
		return nil
	}

	spec := a.getSpec()
	if spec == nil {
		return fmt.Errorf("no 'spec' key found in ApplicationSet: %s", a.Name)
	}

	generators := GetYamlValue(spec, []string{"generators"})
	if generators == nil {
		return nil
	}

	// Process each generator
	for _, generator := range generators.Content {
		if generator.Kind != yaml.MappingNode {
			continue
		}

		// Look for git generator
		gitGen := GetYamlValue(generator, []string{"git"})
		if gitGen == nil {
			continue
		}

		// Check repoURL
		repoURL := GetYamlValue(gitGen, []string{"repoURL"})
		if repoURL == nil || !containsIgnoreCase(repoURL.Value, repo) {
			continue
		}

		// Check targetRevision
		targetRevision := GetYamlValue(gitGen, []string{"targetRevision"})
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

	return nil
}

// Filter checks if the application matches the given selectors and watches the given files
func (a *ArgoResource) Filter(
	selectors []Selector,
	filesChanged []string,
	ignoreInvalidWatchPattern bool,
) *ArgoResource {
	// Check selectors
	if len(selectors) > 0 {
		metadata := GetYamlValue(a.Yaml, []string{"metadata"})
		if metadata == nil {
			return nil
		}

		labels := GetYamlValue(metadata, []string{"labels"})
		if labels == nil {
			return nil
		}

		for _, selector := range selectors {
			labelValue := GetYamlValue(labels, []string{selector.Key})
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
		metadata := GetYamlValue(a.Yaml, []string{"metadata"})
		if metadata == nil {
			return nil
		}

		annotations := GetYamlValue(metadata, []string{"annotations"})
		if annotations == nil {
			return nil
		}

		watchPattern := GetYamlValue(annotations, []string{AnnotationWatchPattern})
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
		return GetYamlValue(a.Yaml, []string{"spec"})
	case ApplicationSet:
		return GetYamlValue(a.Yaml, []string{"spec", "template", "spec"})
	default:
		return nil
	}
}

func (a *ArgoResource) redirectSource(source *yaml.Node, repo, branch string, redirectRevisions []string) error {

	if GetYamlValue(source, []string{"chart"}) != nil {
		log.Printf("Found helm chart in file: %s", a.FileName)
		return nil
	}

	repoURL := GetYamlValue(source, []string{"repoURL"})
	if repoURL == nil || !containsIgnoreCase(repoURL.Value, repo) {
		log.Printf("Found no 'repoURL' under spec.source in file: %s", a.FileName)
		return nil
	}

	targetRev := GetYamlValue(source, []string{"targetRevision"})
	if targetRev == nil {
		log.Printf("Found no 'targetRevision' under spec.source in file: %s", a.FileName)
		targetRev = &yaml.Node{Value: "HEAD"}
		setYamlValue(source, []string{"targetRevision"}, "HEAD")
	}

	shouldRedirect := len(redirectRevisions) == 0 ||
		contains(redirectRevisions, targetRev.Value)

	if shouldRedirect {
		log.Printf("Redirecting targetRevision from %s to %s in file: %s", targetRev.Value, branch, a.FileName)
		setYamlValue(source, []string{"targetRevision"}, branch)
	}

	return nil
}

// GetYamlValue gets a value from a YAML node by path
func GetYamlValue(node *yaml.Node, path []string) *yaml.Node {
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
			return GetYamlValue(node.Content[i+1], path[1:])
		}
	}
	return nil
}

func setYamlValue(node *yaml.Node, path []string, value string) {
	if node == nil || len(path) == 0 {
		log.Printf("Can't set value because node is nil or path is empty")
		return
	}

	if node.Kind != yaml.MappingNode {
		log.Printf("Can't set value because node is not a mapping node")
		return
	}

	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == path[0] {
			if len(path) == 1 {
				// Create new node if it doesn't exist
				if node.Content[i+1] == nil {
					node.Content[i+1] = &yaml.Node{
						Kind:  yaml.ScalarNode,
						Value: value,
					}
				} else {
					// Update existing node
					node.Content[i+1].Kind = yaml.ScalarNode
					node.Content[i+1].Value = value
					node.Content[i+1].Tag = "!!str"
				}
				return
			}
			setYamlValue(node.Content[i+1], path[1:], value)
			return
		}
	}

	// Key not found, create new key-value pair
	if len(path) == 1 {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: path[0]},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value},
		)
	} else {
		newMap := &yaml.Node{Kind: yaml.MappingNode}
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: path[0]},
			newMap,
		)
		setYamlValue(newMap, path[1:], value)
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

	kind := GetYamlValue(resource.Yaml.Content[0], []string{"kind"})
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

	name := GetYamlValue(resource.Yaml.Content[0], []string{"metadata", "name"})
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

func ApplicationsToString(apps []ArgoResource) string {
	var yamlStrings []string
	for _, app := range apps {
		yamlStr, err := app.AsString()
		if err != nil {
			log.Printf("Failed to convert app %s to YAML: %v", app.Name, err)
			continue
		}
		// add a comment with the name of the file
		yamlStr = fmt.Sprintf("# File: %s\n%s", app.FileName, yamlStr)

		yamlStrings = append(yamlStrings, yamlStr)
	}
	return strings.Join(yamlStrings, "---\n")
}
