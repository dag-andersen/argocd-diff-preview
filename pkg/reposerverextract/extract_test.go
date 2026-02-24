package reposerverextract

// Tests for buildManifestRequestWithPackaging – the routing logic that decides
// how to call the Argo CD repo server for a given Application/ApplicationSet.
//
// Key regression: external Helm chart sources (spec.sources[].chart != "") that
// also have a $ref source used to fail with:
//
//   repo server returned error: error getting helm repos: error retrieving helm
//   dependency repos: error reading helm chart from /tmp/<uuid>/Chart.yaml:
//   open /tmp/<uuid>/Chart.yaml: no such file or directory
//
// because the code tried to stream a tarball of local files for a chart that
// lives in an external Helm registry.  The fix: when the primary source has a
// Chart field we use the unary GenerateManifest RPC (GenerateManifestsRemote)
// instead of streaming, and we populate RefSources so the repo server can
// resolve the $ref value files from its own git cache.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

// makeApp is a small helper that builds an ArgoResource from a YAML string.
func makeApp(t *testing.T, rawYAML string) argoapplication.ArgoResource {
	t.Helper()
	var obj unstructured.Unstructured
	require.NoError(t, yaml.Unmarshal([]byte(rawYAML), &obj))

	kind := argoapplication.Application
	if obj.GetKind() == "ApplicationSet" {
		kind = argoapplication.ApplicationSet
	}

	return argoapplication.ArgoResource{
		Yaml:     &obj,
		Kind:     kind,
		Id:       obj.GetName(),
		Name:     obj.GetName(),
		FileName: "test.yaml",
		Branch:   git.Base,
	}
}

// makeBranchFolder creates a temporary directory that acts as a checked-out
// branch folder, including a minimal file at the given path so copyDir succeeds.
func makeBranchFolder(t *testing.T, relPath string) string {
	t.Helper()
	dir := t.TempDir()
	if relPath != "" {
		full := filepath.Join(dir, relPath)
		require.NoError(t, os.MkdirAll(full, 0o755))
		// Write a dummy file so the directory is non-empty and copyDir works.
		require.NoError(t, os.WriteFile(filepath.Join(full, "values.yaml"), []byte("key: value\n"), 0o644))
	}
	return dir
}

// ─────────────────────────────────────────────────────────────────────────────
// 1.  Single-source, local chart (fast path, no refs)
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildManifestRequest_SingleSource_LocalChart(t *testing.T) {
	branchFolder := makeBranchFolder(t, "apps/my-app")

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  destination:
    namespace: production
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/my-app
    targetRevision: HEAD
`)

	req, streamDir, cleanup, err := buildManifestRequestWithPackaging(app, branchFolder, nil)
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// Fast path: streamDir == branchFolder, no temp dir created.
	assert.Equal(t, branchFolder, streamDir, "should stream the full branch folder for local charts")
	assert.Equal(t, "apps/my-app", req.ApplicationSource.Path)
	assert.Empty(t, req.ApplicationSource.Chart, "should not have a Chart field")
	assert.Equal(t, "production", req.Namespace)
	assert.Nil(t, req.RefSources)
}

// ─────────────────────────────────────────────────────────────────────────────
// 2.  Single-source, REMOTE/external Helm chart (no ref sources)
//     → must use GenerateManifestsRemote (streamDir == "")
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildManifestRequest_SingleSource_ExternalChart_NoRefs(t *testing.T) {
	branchFolder := t.TempDir() // contents don't matter – should not be streamed

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cert-manager
spec:
  destination:
    namespace: cert-manager
  source:
    repoURL: https://charts.jetstack.io
    chart: cert-manager
    targetRevision: v1.14.5
`)

	req, streamDir, cleanup, err := buildManifestRequestWithPackaging(app, branchFolder, nil)
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// Remote chart: streamDir must be empty so the caller uses GenerateManifestsRemote.
	assert.Empty(t, streamDir, "streamDir must be empty for an external Helm chart without refs")
	assert.Equal(t, "cert-manager", req.ApplicationSource.Chart)
	assert.Equal(t, "https://charts.jetstack.io", req.Repo.Repo)
	assert.Nil(t, req.RefSources)
}

// ─────────────────────────────────────────────────────────────────────────────
// 3.  REGRESSION: external Helm chart WITH a $ref source
//     This was the bug: the old code tried to stream a tarball that had no
//     Chart.yaml, causing the repo server to fail.
//
//     Fix: use GenerateManifestsRemote (streamDir == "") and populate RefSources
//     so the repo server can resolve the $ref value files from its git cache.
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildManifestRequest_ExternalChart_WithRef_UsesRemoteRPC(t *testing.T) {
	// This test captures the exact failure pattern from production:
	//
	//   sources:
	//     - repoURL: https://github.com/org/repo.git
	//       ref: local                               ← ref-only source
	//     - chart: cert-manager
	//       repoURL: https://charts.jetstack.io      ← primary (external chart)
	//       helm:
	//         valueFiles:
	//           - $local/clusters/prod/values.yaml   ← $ref path
	//
	// Before the fix, buildManifestRequestWithPackaging would try to stream a
	// tarball for the "cert-manager" chart source.  The repo server would then
	// look for Chart.yaml inside the tarball (at the temp dir root) and fail
	// with "no such file or directory".

	branchFolder := t.TempDir() // contents irrelevant – must NOT be streamed

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cert-manager-prod
spec:
  destination:
    namespace: cert-manager
  sources:
    - repoURL: https://github.com/org/repo.git
      ref: local
      targetRevision: HEAD
    - repoURL: https://charts.jetstack.io
      chart: cert-manager
      targetRevision: v1.14.5
      helm:
        valueFiles:
          - $local/clusters/prod/cert-manager-values.yaml
`)

	req, streamDir, cleanup, err := buildManifestRequestWithPackaging(app, branchFolder, nil)
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// CRITICAL: streamDir must be empty – we must NOT stream a tarball for an
	// external chart.  The caller will use GenerateManifestsRemote instead.
	assert.Empty(t, streamDir,
		"REGRESSION: external Helm chart with ref sources must use the remote RPC (streamDir must be empty), "+
			"not stream a tarball that has no Chart.yaml")

	// The primary source should be the chart source (not the ref source).
	assert.Equal(t, "cert-manager", req.ApplicationSource.Chart)
	assert.Equal(t, "https://charts.jetstack.io", req.Repo.Repo)
	assert.Equal(t, "v1.14.5", req.Revision)

	// RefSources must be populated so the repo server can resolve $local/…
	require.NotNil(t, req.RefSources, "RefSources must be populated for $ref value files")
	refTarget, ok := req.RefSources["$local"]
	require.True(t, ok, "RefSources must contain an entry for '$local'")
	assert.Equal(t, "https://github.com/org/repo.git", refTarget.Repo.Repo)
	assert.Equal(t, "HEAD", refTarget.TargetRevision)

	// Value file paths must NOT be rewritten – they stay as $local/… so the
	// repo server resolves them against the ref it fetched.
	require.NotNil(t, req.ApplicationSource.Helm)
	require.Len(t, req.ApplicationSource.Helm.ValueFiles, 1)
	assert.Equal(t, "$local/clusters/prod/cert-manager-values.yaml", req.ApplicationSource.Helm.ValueFiles[0],
		"value file path must remain as a $ref path for the remote RPC")
}

// ─────────────────────────────────────────────────────────────────────────────
// 4.  Multi-source with ref AND a local chart (slow path: temp dir + streaming)
//     Value file $ref/… paths must be rewritten to relative paths.
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildManifestRequest_MultiSource_LocalChart_WithRef_RewritesValueFiles(t *testing.T) {
	// Branch layout:
	//   <branchFolder>/
	//     apps/my-chart/           ← primary source path
	//       Chart.yaml
	//     config/                  ← directory the ref source points at (ref-only: no path field)
	//       values-prod.yaml
	//
	// A "ref-only" source has ref != "" and path == "".  A source with both ref
	// and path set is treated as a content source (not a ref source) by the
	// split logic in buildManifestRequestWithPackaging.
	branchFolder := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(branchFolder, "apps", "my-chart"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(branchFolder, "apps", "my-chart", "Chart.yaml"), []byte("name: my-chart\n"), 0o644))
	// The ref-only source has no path (points at repo root), so we put the
	// values file at the repo root level inside the branch folder.
	require.NoError(t, os.WriteFile(filepath.Join(branchFolder, "values-prod.yaml"), []byte("replicas: 3\n"), 0o644))

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-chart-prod
spec:
  destination:
    namespace: production
  sources:
    - repoURL: https://github.com/org/repo.git
      ref: cfg
      targetRevision: HEAD
    - repoURL: https://github.com/org/repo.git
      path: apps/my-chart
      targetRevision: HEAD
      helm:
        valueFiles:
          - $cfg/values-prod.yaml
`)

	req, streamDir, cleanup, err := buildManifestRequestWithPackaging(app, branchFolder, nil)
	require.NoError(t, err)
	require.NotEmpty(t, streamDir, "local chart with refs must stream a temp dir")
	defer cleanup()

	// Temp dir should not be the branchFolder itself.
	assert.NotEqual(t, branchFolder, streamDir)

	// The primary source path should be preserved in the request.
	assert.Equal(t, "apps/my-chart", req.ApplicationSource.Path)
	assert.Empty(t, req.ApplicationSource.Chart)

	// Value file must be rewritten from $cfg/values-prod.yaml to a relative path.
	require.NotNil(t, req.ApplicationSource.Helm)
	require.Len(t, req.ApplicationSource.Helm.ValueFiles, 1)
	vf := req.ApplicationSource.Helm.ValueFiles[0]
	assert.NotContains(t, vf, "$", "value file path must be rewritten to a relative path, not keep the $ref prefix")

	// The rewritten path must point to the correct file inside the temp dir.
	absValueFile := filepath.Join(streamDir, "apps", "my-chart", vf)
	absValueFile = filepath.Clean(absValueFile)
	_, statErr := os.Stat(absValueFile)
	assert.NoError(t, statErr, "rewritten value file path %q should exist on disk", absValueFile)
}

// ─────────────────────────────────────────────────────────────────────────────
// 5.  ApplicationSet: sources live under spec.template.spec
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildManifestRequest_ApplicationSet_ExternalChart_WithRef(t *testing.T) {
	branchFolder := t.TempDir()

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: cluster-common-charts
spec:
  template:
    spec:
      destination:
        namespace: monitoring
      sources:
        - repoURL: https://github.com/org/repo.git
          ref: local
          targetRevision: HEAD
        - repoURL: https://prometheus-community.github.io/helm-charts
          chart: kube-prometheus-stack
          targetRevision: 58.0.0
          helm:
            valueFiles:
              - $local/charts/prometheus/values.yaml
`)

	req, streamDir, cleanup, err := buildManifestRequestWithPackaging(app, branchFolder, nil)
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// Must use remote RPC (same regression guard as test 3, but for ApplicationSets).
	assert.Empty(t, streamDir,
		"REGRESSION: ApplicationSet with external chart + ref source must use remote RPC")

	assert.Equal(t, "kube-prometheus-stack", req.ApplicationSource.Chart)
	assert.Equal(t, "monitoring", req.Namespace)

	require.NotNil(t, req.RefSources)
	_, hasLocal := req.RefSources["$local"]
	assert.True(t, hasLocal, "RefSources must contain '$local'")
}

// ─────────────────────────────────────────────────────────────────────────────
// 6.  No source at all → error
// ─────────────────────────────────────────────────────────────────────────────

func TestBuildManifestRequest_NoSource_ReturnsError(t *testing.T) {
	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: broken-app
spec:
  destination:
    namespace: default
`)

	_, _, cleanup, err := buildManifestRequestWithPackaging(app, t.TempDir(), nil)
	if cleanup != nil {
		defer cleanup()
	}
	assert.Error(t, err, "application with no source should return an error")
}
