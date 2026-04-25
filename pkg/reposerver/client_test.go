package reposerver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClientWithAddress_SetsAddress(t *testing.T) {
	addr := "argocd-repo-server.argocd.svc.cluster.local:8081"
	c := NewClientWithAddress(addr, false, true)

	assert.Equal(t, addr, c.address)
	assert.False(t, c.disableTLS)
	assert.True(t, c.insecureSkipVerify)
	assert.Nil(t, c.k8sClient, "NewClientWithAddress should not set a k8s client")
}

func TestNewClientWithAddress_PlaintextMode(t *testing.T) {
	c := NewClientWithAddress("localhost:8081", true, false)

	assert.True(t, c.disableTLS)
	assert.False(t, c.insecureSkipVerify)
	assert.Nil(t, c.k8sClient)
}

func TestEnsurePortForward_NilK8sClient_IsNoop(t *testing.T) {
	c := NewClientWithAddress("argocd-repo-server.argocd.svc.cluster.local:8081", false, true)
	require.Nil(t, c.k8sClient)

	err := c.EnsurePortForward()
	assert.NoError(t, err, "EnsurePortForward should be a no-op when k8sClient is nil")
}

func TestEnsurePortForward_NilK8sClient_Idempotent(t *testing.T) {
	c := NewClientWithAddress("localhost:8081", true, false)

	for range 3 {
		assert.NoError(t, c.EnsurePortForward())
	}
}

func TestCleanup_NilK8sClient_DoesNotPanic(t *testing.T) {
	c := NewClientWithAddress("localhost:8081", false, true)
	assert.NotPanics(t, func() { c.Cleanup() })
}

func TestNewClient_SetsLocalAddress(t *testing.T) {
	// NewClient always binds to localhost:<repoServerLocalPort> for the port-forward.
	c := NewClient(nil, "argocd") // k8sClient nil is fine for this field-check
	assert.Contains(t, c.address, "localhost:")
	assert.True(t, c.insecureSkipVerify, "should skip verify for cluster-internal self-signed cert")
}
