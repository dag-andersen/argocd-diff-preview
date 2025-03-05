package argoapplicaiton

import (
	"fmt"
	"strings"

	"github.com/argocd-diff-preview/argocd-diff-preview/pkg/types"
	yamlutil "github.com/argocd-diff-preview/argocd-diff-preview/pkg/yaml"
	"github.com/rs/zerolog/log"
)

// FromK8sResource creates an ArgoResource from a K8sResource
func FromK8sResource(resource *types.K8sResource) *ArgoResource {
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
		Kind:     ApplicationKind(appKind),
		Name:     name.Value,
		FileName: resource.FileName,
	}
}

// ApplicationsToString converts a slice of ArgoResource to a YAML string
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
