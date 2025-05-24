package argoapplication

import (
	"fmt"
	"strings"

	k8s "github.com/dag-andersen/argocd-diff-preview/pkg/fileparsing"
	"github.com/rs/zerolog/log"
)

// FromResourceToApplication converts K8sResources to ArgoResources with filtering
func FromResourceToApplication(
	k8sResources []k8s.Resource,
) []ArgoResource {
	var apps []ArgoResource

	// Convert K8sResources to ArgoResources
	for _, r := range k8sResources {
		if app := fromK8sResource(r); app != nil {
			apps = append(apps, *app)
		}
	}

	return apps
}

// fromK8sResource creates an ArgoResource from a K8sResource
func fromK8sResource(resource k8s.Resource) *ArgoResource {

	kind := resource.Yaml.GetKind()
	if kind == "" {
		log.Debug().Str("file", resource.FileName).Msg("No 'kind' field found in file")
		return nil
	}

	// Check if it's an Argo CD resource
	var appKind ApplicationKind
	switch kind {
	case "Application":
		appKind = Application
	case "ApplicationSet":
		appKind = ApplicationSet
	default:
		return nil
	}

	name := resource.Yaml.GetName()
	if name == "" {
		log.Debug().Str("file", resource.FileName).Msg("No 'metadata.name' field found in file")
		return nil
	}

	return NewArgoResource(&resource.Yaml, ApplicationKind(appKind), name, name, resource.FileName, resource.Branch)
}

// ApplicationsToString converts a slice of ArgoResource to a YAML string
func ApplicationsToString(apps []ArgoResource) string {
	var yamlStrings []string
	for _, app := range apps {
		yamlStr, err := app.AsString()
		if err != nil {
			log.Debug().Err(err).Str(app.Kind.ShortName(), app.GetLongName()).Msg("Failed to convert app to YAML")
			continue
		}
		// add a comment with the name of the file
		yamlStr = fmt.Sprintf("# File: %s\n%s", app.FileName, yamlStr)

		yamlStrings = append(yamlStrings, yamlStr)
	}
	return strings.Join(yamlStrings, "---\n")
}
