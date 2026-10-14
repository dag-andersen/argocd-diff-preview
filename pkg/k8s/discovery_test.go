package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
)

func newFakeDiscoveryClient(resources []*metav1.APIResourceList) *Client {
	return &Client{
		discoveryClient: &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{Resources: resources}},
	}
}

func TestGetClusterScopedResourcesAndAPIVersions(t *testing.T) {
	client := newFakeDiscoveryClient([]*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "pods", Kind: "Pod", Namespaced: true},
				{Name: "pods/log", Kind: "Pod", Namespaced: true},
				{Name: "namespaces", Kind: "Namespace", Namespaced: false},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Kind: "Deployment", Namespaced: true},
				{Name: "deployments/scale", Kind: "Scale", Namespaced: true},
			},
		},
		{
			GroupVersion: "monitoring.coreos.com/v1",
			APIResources: []metav1.APIResource{
				{Name: "servicemonitors", Kind: "ServiceMonitor", Namespaced: true},
			},
		},
	})

	clusterScoped, apiVersions, err := client.GetClusterScopedResourcesAndAPIVersions()
	require.NoError(t, err)

	assert.Equal(t, []string{
		"apps/v1",
		"apps/v1/Deployment",
		"monitoring.coreos.com/v1",
		"monitoring.coreos.com/v1/ServiceMonitor",
		"v1",
		"v1/Namespace",
		"v1/Pod",
	}, apiVersions)
	assert.Equal(t, map[schema.GroupKind]bool{{Group: "", Kind: "Namespace"}: true}, clusterScoped)
}
