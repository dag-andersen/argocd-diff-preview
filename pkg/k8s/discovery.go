package k8s

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GetServerVersion returns the Kubernetes server version string (e.g. "v1.29.2").
func (c *Client) GetServerVersion() (string, error) {
	v, err := c.discoveryClient.ServerVersion()
	if err != nil {
		return "", err
	}
	return v.GitVersion, nil
}

// GetAPIVersions returns all unique GroupVersion strings (e.g. "v1", "apps/v1",
// "monitoring.coreos.com/v1") available in the cluster. This is the format
// consumed by the Argo CD repo server's ManifestRequest.ApiVersions field and
// exposed to Helm templates via .Capabilities.APIVersions.
func (c *Client) GetAPIVersions() ([]string, error) {
	_, apiVersions, err := c.GetClusterScopedResourcesAndAPIVersions()
	if err != nil {
		return nil, err
	}
	return apiVersions, nil
}

// GetClusterScopedResourcesAndAPIVersions returns metadata about all cluster-scoped
// resource types and all unique GroupVersion strings in one discovery pass.
func (c *Client) GetClusterScopedResourcesAndAPIVersions() (map[schema.GroupKind]bool, []string, error) {
	_, apiResourceLists, err := c.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to discover API resources: %w", err)
	}

	clusterScopedResources := make(map[schema.GroupKind]bool)
	seen := make(map[string]bool)
	var apiVersions []string
	for _, apiResourceList := range apiResourceLists {
		if !seen[apiResourceList.GroupVersion] {
			seen[apiResourceList.GroupVersion] = true
			apiVersions = append(apiVersions, apiResourceList.GroupVersion)
		}
		// Parse GroupVersion to extract the group
		gv, err := schema.ParseGroupVersion(apiResourceList.GroupVersion)
		if err != nil {
			log.Warn().Err(err).Msgf("Failed to parse GroupVersion: %s", apiResourceList.GroupVersion)
			continue
		}

		// Check each resource in the API group
		for _, apiResource := range apiResourceList.APIResources {
			// Only retain cluster-scoped resources. A kind missing from this map is
			// treated as namespaced, which is the safe default for undiscovered CRDs.
			if apiResource.Namespaced {
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

			clusterScopedResources[gk] = true
		}
	}
	sort.Strings(apiVersions)
	return clusterScopedResources, apiVersions, nil
}

// GetListOfClusterScopedResources returns metadata about all cluster-scoped resource types.
// A GroupKind absent from the returned map should be treated as namespaced.
// This format matches the interface expected by Argo CD's kubeutil.ResourceInfoProvider
func (c *Client) GetListOfClusterScopedResources() (map[schema.GroupKind]bool, error) {
	clusterScopedResources, _, err := c.GetClusterScopedResourcesAndAPIVersions()
	if err != nil {
		return nil, err
	}
	return clusterScopedResources, nil
}
