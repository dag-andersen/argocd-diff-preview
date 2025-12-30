package extract

import (
	"fmt"

	"github.com/dag-andersen/argocd-diff-preview/pkg/utils"
	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// processYamlOutput parses a YAML chunk into an unstructured.Unstructured
// A chunk is a single YAML object, e.g. a Deployment, Service, etc.
func processYamlOutput(chunk string, skipResourceRules []SkipResourceRule) ([]unstructured.Unstructured, error) {

	documents := utils.SplitYAMLDocuments(chunk)

	manifests := make([]unstructured.Unstructured, 0)

	for _, doc := range documents {

		// Create a new map to hold the parsed YAML
		var yamlObj map[string]any
		err := yaml.Unmarshal([]byte(doc), &yamlObj)
		if err != nil {
			log.Debug().Msgf("Failed to parse YAML: \n%s", doc)
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}

		// Skip empty objects
		if len(yamlObj) == 0 {
			continue
		}

		// Check if this is a valid Kubernetes resource
		apiVersion, found, _ := unstructured.NestedString(yamlObj, "apiVersion")
		kind, kindFound, _ := unstructured.NestedString(yamlObj, "kind")
		name, nameFound, _ := unstructured.NestedString(yamlObj, "metadata", "name")
		if !found || !kindFound || !nameFound || apiVersion == "" || kind == "" || name == "" {
			log.Debug().Msgf("Found manifest with no apiVersion or kind: %s", doc)
			continue
		}

		// Check if the manifest matches the skip resource rules
		if len(skipResourceRules) > 0 {

			group := groupFromAPIVersion(apiVersion)
			for _, skipResourceRule := range skipResourceRules {
				if skipResourceRule.Matches(group, kind, name) {
					log.Debug().Msgf("Skipping manifest %s because it matches the skip resource rule [%s]", doc, skipResourceRule.String())
					break
				}
			}
		}

		manifests = append(manifests, unstructured.Unstructured{Object: yamlObj})
	}

	log.Debug().Msgf("Parsed %d manifests", len(manifests))

	return manifests, nil
}
