package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// baseRawOptions returns a RawOptions with the minimum fields set so that
// ToConfig() reaches our validation logic without failing on unrelated checks.
func baseRawOptions() *RawOptions {
	return &RawOptions{
		BaseBranch:   "main",
		TargetBranch: "feature",
		Repo:         "owner/repo",
		RenderMethod: string(RenderMethodRepoServerAPI),
	}
}

func TestToConfig_RepoServerAddress_ValidWithRepoServerAPI(t *testing.T) {
	o := baseRawOptions()
	o.RepoServerAddress = "argocd-repo-server.argocd.svc.cluster.local:8081"

	cfg, err := o.ToConfig()
	require.NoError(t, err)
	assert.Equal(t, o.RepoServerAddress, cfg.RepoServerAddress)
}

func TestToConfig_RepoServerAddress_EmptyIsValid(t *testing.T) {
	o := baseRawOptions()
	o.RepoServerAddress = ""

	cfg, err := o.ToConfig()
	require.NoError(t, err)
	assert.Empty(t, cfg.RepoServerAddress)
}

func TestToConfig_RepoServerAddress_RequiresRepoServerAPIMode(t *testing.T) {
	tests := []struct {
		name         string
		renderMethod string
	}{
		{"server-api", string(RenderMethodServerAPI)},
		{"cli", string(RenderMethodCLI)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := baseRawOptions()
			o.RenderMethod = tt.renderMethod
			o.RepoServerAddress = "argocd-repo-server.argocd.svc.cluster.local:8081"
			o.DryRun = true // skip argocd CLI binary check for cli mode

			_, err := o.ToConfig()
			assert.ErrorContains(t, err, "--repo-server-address requires --render-method=repo-server-api")
		})
	}
}
