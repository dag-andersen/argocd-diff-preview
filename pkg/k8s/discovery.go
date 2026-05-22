package k8s

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetServerVersion returns the Kubernetes server version string (e.g. "v1.29.2").
func (c *Client) GetServerVersion() (string, error) {
	v, err := c.discoveryClient.ServerVersion()
	if err != nil {
		return "", fmt.Errorf("failed to get server version: %w", err)
	}
	return v.GitVersion, nil
}

// GetAPIVersions returns all unique GroupVersion strings (e.g. "v1", "apps/v1",
// "monitoring.coreos.com/v1") available in the cluster. This is the format
// consumed by the Argo CD repo server's ManifestRequest.ApiVersions field and
// exposed to Helm templates via .Capabilities.APIVersions.
func (c *Client) GetAPIVersions() ([]string, error) {
	_, apiResourceLists, err := c.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return nil, fmt.Errorf("failed to discover API resources: %w", err)
	}

	seen := make(map[string]bool)
	var apiVersions []string
	for _, apiResourceList := range apiResourceLists {
		if seen[apiResourceList.GroupVersion] {
			continue
		}
		seen[apiResourceList.GroupVersion] = true
		apiVersions = append(apiVersions, apiResourceList.GroupVersion)
	}
	return apiVersions, nil
}

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
