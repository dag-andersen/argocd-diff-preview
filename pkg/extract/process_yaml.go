package extract

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// processYamlOutput parses a YAML chunk into an unstructured.Unstructured
// A chunk is a single YAML object, e.g. a Deployment, Service, etc.
func processYamlOutput(chunk string) ([]unstructured.Unstructured, error) {

	// split
	documents := strings.Split(chunk, "---")

	manifests := make([]unstructured.Unstructured, 0)

	for _, doc := range documents {
		// Skip empty documents
		trimmedDoc := strings.TrimSpace(doc)

		if trimmedDoc == "" {
			continue
		}

		// Create a new map to hold the parsed YAML
		var yamlObj map[string]interface{}
		err := yaml.Unmarshal([]byte(trimmedDoc), &yamlObj)
		if err != nil {
			log.Debug().Msgf("Failed to parse YAML: \n%s", trimmedDoc)
			return nil, fmt.Errorf("failed to parse YAML: %w", err)
		}

		// Skip empty objects
		if len(yamlObj) == 0 {
			continue
		}

		// Check if this is a valid Kubernetes resource
		apiVersion, found, _ := unstructured.NestedString(yamlObj, "apiVersion")
		kind, kindFound, _ := unstructured.NestedString(yamlObj, "kind")

		if !found || !kindFound || apiVersion == "" || kind == "" {
			log.Debug().Msgf("Found manifest with no apiVersion or kind: %s", trimmedDoc)
			continue
		}

		manifests = append(manifests, unstructured.Unstructured{Object: yamlObj})
	}

	log.Debug().Msgf("Parsed %d manifests", len(manifests))

	return manifests, nil
}
