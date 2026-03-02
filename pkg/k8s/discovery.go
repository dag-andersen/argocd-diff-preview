package k8s

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetListOfNamespacedScopedResources returns metadata about all namespaced resource types
// Returns a map where the key is schema.GroupKind and the value is true (indicating the resource is namespaced)
// This format matches the interface expected by Argo CD's kubeutil.ResourceInfoProvider
func (c *Client) GetListOfNamespacedScopedResources() (map[schema.GroupKind]bool, error) {
	namespacedScopedResources := make(map[schema.GroupKind]bool)

	// Get all API resources from the cluster
	_, apiResourceLists, err := c.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return nil, fmt.Errorf("failed to discover API resources: %w", err)
	}

	// Iterate through all resource groups and versions
	for _, apiResourceList := range apiResourceLists {
		// Parse GroupVersion to extract the group
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to parse GroupVersion: %s", apiResourceList.GroupVersion)
			continue
		}

		// Check each resource in the API group
		for _, apiResource := range apiResourceList.APIResources {
			// Skip if this is a cluster-scoped resource (not namespaced)
			if !apiResource.Namespaced {
				continue
			}

			// Skip subresources (e.g., "pods/log", "deployments/scale")
			if strings.Contains(apiResource.Name, "/") {
				continue
			}

			// Create key as schema.GroupKind
			gk := schema.GroupKind{
				Group: gv.Group,
				Kind:  apiResource.Kind,
			}

			// Store with value true (indicating this resource is namespaced)
			namespacedScopedResources[gk] = true
		}
	}

	return namespacedScopedResources, nil
}
