package reposerverextract

// Tests for the manifest-request building logic – the routing that decides
// how to call the Argo CD repo server for a given Application/ApplicationSet.
//
// Key regression: external Helm chart sources (spec.sources[].chart != "") that
// also have a $ref source used to fail with:
//
//	repo server returned error: error getting helm repos: error retrieving helm
//	dependency repos: error reading helm chart from /tmp/<uuid>/Chart.yaml:
//	open /tmp/<uuid>/Chart.yaml: no such file or directory
//
// because the code tried to stream a tarball of local files for a chart that
// lives in an external Helm registry.  The fix: when the primary source has a
// Chart field we use the unary GenerateManifest RPC (GenerateManifestsRemote)
// instead of streaming, and we populate RefSources so the repo server can
// resolve the $ref value files from its own git cache.

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/tgzstream"
	"github.com/dag-andersen/argocd-diff-preview/pkg/argoapplication"
	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/repository"
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
		require.NoError(t, os.WriteFile(filepath.Join(full, "Chart.yaml"), []byte("apiVersion: v2\nname: test\nversion: 0.1.0\n"), 0o644))
		// Write a dummy file so the directory is non-empty and copyDir works.
		require.NoError(t, os.WriteFile(filepath.Join(full, "values.yaml"), []byte("key: value\n"), 0o644))
	}
	return dir
}

func assertDefaultProjectFields(t *testing.T, req *repoapiclient.ManifestRequest) {
	t.Helper()
	assert.Equal(t, "default", req.ProjectName, "project name must match the patched application project")
	assert.Equal(t, []string{"*"}, req.ProjectSourceRepos, "source repos must be permissive so helm build errors are not masked as permission errors")
}

func testRepoSelector(t *testing.T, repo string) *repository.Selector {
	t.Helper()
	selector, err := repository.NewSelector(repo, "")
	require.NoError(t, err)
	return selector
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

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)
	require.Empty(t, refSources)
	assert.False(t, hasMultipleSources)

	kubeVersion := "v1.30.1"
	apiVersions := []string{"apps/v1", "v1"}
	req, streamDir, cleanup, err := buildManifestRequestForSource(
		app,
		contentSources[0],
		refSources,
		hasMultipleSources,
		branchFolder,
		nil,
		manifestRequestRenderContext{
			repoSelector: testRepoSelector(t, ""),
			kubeVersion:  kubeVersion,
			apiVersions:  apiVersions,
		},
	)
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// Fast path: stream only the source directory and clear Path so unrelated
	// monorepo files are not included in the streamed tarball.
	assert.Equal(t, filepath.Join(branchFolder, "apps", "my-app"), streamDir, "should stream only the local source directory")
	assert.Empty(t, req.ApplicationSource.Path)
	assert.Empty(t, req.ApplicationSource.Chart, "should not have a Chart field")
	assert.Equal(t, "production", req.Namespace)
	assert.Equal(t, kubeVersion, req.KubeVersion)
	assert.Equal(t, apiVersions, req.ApiVersions)
	assert.Nil(t, req.RefSources)
	assertDefaultProjectFields(t, req)
}

// ─────────────────────────────────────────────────────────────────────────────
// 2.  Single-source, REMOTE/external Helm chart (no ref sources)
//
//	→ must use GenerateManifestsRemote (streamDir == "")
//
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

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
		repoSelector: testRepoSelector(t, "")})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// Remote chart: streamDir must be empty so the caller uses GenerateManifestsRemote.
	assert.Empty(t, streamDir, "streamDir must be empty for an external Helm chart without refs")
	assert.Equal(t, "cert-manager", req.ApplicationSource.Chart)
	assert.Equal(t, "https://charts.jetstack.io", req.Repo.Repo)
	assert.Nil(t, req.RefSources)
	assertDefaultProjectFields(t, req)
}

// ─────────────────────────────────────────────────────────────────────────────
// 3.  REGRESSION: external Helm chart WITH a $ref source
//
//	This was the bug: the old code tried to stream a tarball that had no
//	Chart.yaml, causing the repo server to fail.
//
//	Fix: use GenerateManifestsRemote (streamDir == "") and populate RefSources
//	so the repo server can resolve the $ref value files from its git cache.
//
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
	// Before the fix, buildManifestRequestForSource would try to stream a
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

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1, "only the chart source is a content source")
	require.Len(t, refSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
		repoSelector: testRepoSelector(t, "")})
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
	assertDefaultProjectFields(t, req)
}

// ─────────────────────────────────────────────────────────────────────────────
// 4.  Multi-source with ref AND a local chart (slow path: temp dir + streaming)
//
//	Value file $ref/… paths must be rewritten to relative paths.
//
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
	// and path set is treated as both a content source AND a ref source by the
	// split logic in splitSources (see GH #401 fix).
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

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)
	require.Len(t, refSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
		repoSelector: testRepoSelector(t, "")})
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
	assertDefaultProjectFields(t, req)
}

// ─────────────────────────────────────────────────────────────────────────────
// 4b. Multi-source: external chart with a ref+path dual-purpose source (GH #401)
//
//	When a source has BOTH ref and path set, the source serves a dual purpose:
//	it produces content (from path) AND it provides a $ref namespace for other
//	sources' value files. The current splitSources logic classifies it only as
//	a content source, causing the chart source's $ref lookup to fail with:
//	  "source referenced "$values", but no source has a 'ref' field defined"
//
// ─────────────────────────────────────────────────────────────────────────────
func TestBuildManifestRequest_ExternalChart_WithRefAndPath_GH401(t *testing.T) {
	// Exact reproduction of https://github.com/dag-andersen/argocd-diff-preview/issues/401
	//
	// This application has a single source that sets BOTH ref and path:
	//   sources:
	//     - ref: values
	//       repoURL: https://github.com/dominik-th/argocd-diff-preview-bug.git
	//       targetRevision: HEAD
	//       path: manifests          ← ref AND path on the same source
	//     - chart: cert-manager
	//       repoURL: https://charts.jetstack.io
	//       targetRevision: v1.20.1
	//       helm:
	//         valueFiles:
	//           - $values/values.yaml
	//
	// splitSources currently treats this first source as content-only (because
	// path != ""), so refSources is empty, and the chart source's request has
	// no RefSources - causing the repo server to error.

	branchFolder := t.TempDir()

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: demo
  namespace: argocd
spec:
  destination:
    namespace: demo
    server: https://kubernetes.default.svc
  project: default
  sources:
    - ref: values
      repoURL: https://github.com/dominik-th/argocd-diff-preview-bug.git
      targetRevision: HEAD
      path: manifests
    - chart: cert-manager
      helm:
        valueFiles:
          - $values/values.yaml
      repoURL: https://charts.jetstack.io
      targetRevision: v1.20.1
`)

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)

	// The first source has both ref and path. It should be a content source
	// (it has a path that produces manifests), but its ref information must
	// also be available for sibling sources.
	require.True(t, hasMultipleSources)

	// BUG ASSERTION: The dual-purpose source (ref+path) must appear in
	// refSources so that the chart source's $values/... value file can be
	// resolved. Currently splitSources puts it only in contentSources.
	require.Len(t, contentSources, 2, "both sources are content sources (the dual-purpose one has a path)")

	// This is the key assertion that currently FAILS - the dual-purpose source
	// must also be present in refSources.
	require.NotEmpty(t, refSources,
		"BUG GH#401: a source with both ref and path must also appear in refSources")

	// Now verify the chart source gets a proper RefSources map.
	// Find the chart content source.
	var chartSource v1alpha1.ApplicationSource
	for _, cs := range contentSources {
		if cs.Chart != "" {
			chartSource = cs
			break
		}
	}
	require.NotEmpty(t, chartSource.Chart, "should find the chart content source")

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, chartSource, refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
		repoSelector: testRepoSelector(t, "")})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// External chart - must use remote RPC (no streaming).
	assert.Empty(t, streamDir,
		"external Helm chart with ref sources must use the remote RPC (streamDir must be empty)")

	// RefSources must be populated so the repo server can resolve $values/…
	require.NotNil(t, req.RefSources,
		"RefSources must be populated for $ref value files")
	refTarget, ok := req.RefSources["$values"]
	require.True(t, ok, "RefSources must contain an entry for '$values'")
	assert.Equal(t, "https://github.com/dominik-th/argocd-diff-preview-bug.git", refTarget.Repo.Repo)
	assert.Equal(t, "HEAD", refTarget.TargetRevision)

	// Value file paths must stay as $values/… for the remote RPC.
	require.NotNil(t, req.ApplicationSource.Helm)
	require.Len(t, req.ApplicationSource.Helm.ValueFiles, 1)
	assert.Equal(t, "$values/values.yaml", req.ApplicationSource.Helm.ValueFiles[0],
		"value file path must remain as a $ref path for the remote RPC")
	assertDefaultProjectFields(t, req)
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

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)
	require.Len(t, refSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
		repoSelector: testRepoSelector(t, "")})
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
	assertDefaultProjectFields(t, req)
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

	_, _, _, err := splitSources(app)
	assert.Error(t, err, "application with no source should return an error")
}

// ─────────────────────────────────────────────────────────────────────────────
// 7.  Multiple content sources → one request per source, all succeed
//
//	This is the pattern from the real-world failure report:
//
//	  sources:
//	    - path: management-prod/applicationsets
//	    - path: management-prod/root
//
//	Previously this returned an error; now we build a request for each.
//
// ─────────────────────────────────────────────────────────────────────────────
func TestBuildManifestRequest_MultipleContentSources_BuildsOneRequestEach(t *testing.T) {
	branchFolder := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(branchFolder, "management-prod", "applicationsets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(branchFolder, "management-prod", "applicationsets", "app.yaml"), []byte("kind: Application\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(branchFolder, "management-prod", "root"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(branchFolder, "management-prod", "root", "app.yaml"), []byte("kind: Application\n"), 0o644))

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: root
  namespace: argocd
spec:
  project: in-cluster
  sources:
    - repoURL: https://github.com/egmontadministration/argo-management-cluster.git
      path: management-prod/applicationsets
    - repoURL: https://github.com/egmontadministration/argo-management-cluster.git
      path: management-prod/root
  destination:
    name: in-cluster
    namespace: argocd
`)

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 2, "both path sources are content sources")
	require.Empty(t, refSources)
	assert.True(t, hasMultipleSources)

	// Build a request for each content source – this must not error.
	// Capture requests so we can verify per-source paths without duplicate calls.
	for i, cs := range contentSources {
		req, streamDir, cleanup, buildErr := buildManifestRequestForSource(app, cs, refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
			repoSelector: testRepoSelector(t, "")})
		require.NoError(t, buildErr, "content source %d should not error", i)
		if cleanup != nil {
			defer cleanup()
		}

		// Multiple local path sources without Helm charts keep the branch root.
		assert.Equal(t, branchFolder, streamDir, "content source %d should stream the branch root", i)
		assert.Equal(t, cs.Path, req.ApplicationSource.Path)
		assert.True(t, req.HasMultipleSources, "HasMultipleSources must be true for both requests")
		assert.Equal(t, "argocd", req.Namespace)
		assertDefaultProjectFields(t, req)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 8.  Cross-repo source → must use remote RPC (streamDir == "")
//
//	This is the failure pattern from production: an application whose
//	spec.source.repoURL points at a DIFFERENT repository than the PR repo.
//	The source path does not exist locally, so streaming would fail with
//	"app path does not exist".
//
//	Fix: when prRepo is set and the source repoURL doesn't match, return
//	streamDir="" so the caller uses the remote GenerateManifest RPC and
//	the repo server fetches the content itself.
//
// ─────────────────────────────────────────────────────────────────────────────
func TestBuildManifestRequest_CrossRepoSource_UsesRemoteRPC(t *testing.T) {
	// The PR repo is "argo-management-cluster", but the app points at "argo-apps".
	// "argo-apps" is NOT checked out locally.
	prRepo := "https://github.com/egmontadministration/argo-management-cluster.git"
	branchFolder := t.TempDir() // does NOT contain the argo-apps path

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cloud-services
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/egmontadministration/argo-apps.git
    path: eks-platformservices-nonprod/apps/service-cloud-services/cloud-services
    targetRevision: HEAD
  destination:
    server: https://kubernetes.default.svc
    namespace: cloud-services
`)

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)
	require.Empty(t, refSources)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
		repoSelector: testRepoSelector(t, prRepo)})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// CRITICAL: streamDir must be empty - the path lives in a foreign repo.
	// Streaming local files would fail with "app path does not exist".
	assert.Empty(t, streamDir,
		"REGRESSION: source in a foreign repo must use the remote RPC (streamDir must be empty) "+
			"not stream local files that don't exist")

	// The request should still be properly constructed.
	assert.Equal(t, "https://github.com/egmontadministration/argo-apps.git", req.Repo.Repo)
	assert.Equal(t, "eks-platformservices-nonprod/apps/service-cloud-services/cloud-services", req.ApplicationSource.Path)
	assert.Equal(t, "HEAD", req.Revision)
	assert.Equal(t, "cloud-services", req.Namespace)
	assert.False(t, hasMultipleSources)
	assertDefaultProjectFields(t, req)
}

// ─────────────────────────────────────────────────────────────────────────────
// 9.  Same-repo source with prRepo set → still uses local streaming
//
//	Verifies the happy path is not broken when prRepo is set: sources that
//	belong to the PR repo should still be streamed from local disk.
//
// ─────────────────────────────────────────────────────────────────────────────
func TestBuildManifestRequest_SameRepoSource_WithPrRepo_StreamsLocally(t *testing.T) {
	prRepo := "https://github.com/org/repo.git"
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

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
		repoSelector: testRepoSelector(t, prRepo)})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// Same repo - should stream the source directory, not use remote RPC.
	assert.Equal(t, filepath.Join(branchFolder, "apps", "my-app"), streamDir, "same-repo source must still stream locally even when prRepo is set")
	assert.Empty(t, req.ApplicationSource.Path)
	assertDefaultProjectFields(t, req)
}

func TestBuildManifestRequest_SameRepoPrimaryWithExternalRef_UsesRemoteRPC(t *testing.T) {
	prRepo := "org/helm-charts"
	branchFolder := makeBranchFolder(t, "charts/my-chart")

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  destination:
    namespace: production
  sources:
    - repoURL: https://github.com/org/helm-charts.git
      path: charts/my-chart
      targetRevision: main
      helm:
        valueFiles:
          - $helm-values-repo/values.yaml
    - repoURL: https://github.com/org/app-values.git
      targetRevision: main
      ref: helm-values-repo
`)

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)
	require.Len(t, refSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
		repoSelector: testRepoSelector(t, prRepo)})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	assert.Empty(t, streamDir,
		"external ref source must use remote RPC so repo server fetches ref repository instead of copying PR repo root")
	assert.Equal(t, []string{"$helm-values-repo/values.yaml"}, req.ApplicationSource.Helm.ValueFiles,
		"remote RPC must keep $ref value files unchanged")
	require.NotNil(t, req.RefSources)
	refTarget, ok := req.RefSources["$helm-values-repo"]
	require.True(t, ok)
	assert.Equal(t, "https://github.com/org/app-values.git", refTarget.Repo.Repo)
	assert.Equal(t, "main", refTarget.TargetRevision)
	assertDefaultProjectFields(t, req)
}

func TestBuildManifestRequest_SameRepoPrimaryWithPrefixExternalRef_UsesRemoteRPC(t *testing.T) {
	prRepo := "org/helm-charts"
	branchFolder := makeBranchFolder(t, "charts/my-chart")

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  destination:
    namespace: production
  sources:
    - repoURL: https://github.com/org/helm-charts.git
      path: charts/my-chart
      targetRevision: main
      helm:
        valueFiles:
          - $helm-values-repo/imageTag.yaml
    - repoURL: https://github.com/org/helm-charts-deploy.git
      targetRevision: main
      ref: helm-values-repo
`)

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)
	require.Len(t, refSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
		repoSelector: testRepoSelector(t, prRepo),
	})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	assert.Empty(t, streamDir,
		"ref repo sharing the PR repo prefix must still be treated as external")
	assert.Equal(t, []string{"$helm-values-repo/imageTag.yaml"}, req.ApplicationSource.Helm.ValueFiles)
	require.NotNil(t, req.RefSources)
	refTarget, ok := req.RefSources["$helm-values-repo"]
	require.True(t, ok)
	assert.Equal(t, "https://github.com/org/helm-charts-deploy.git", refTarget.Repo.Repo)
	assertDefaultProjectFields(t, req)
}

// ─────────────────────────────────────────────────────────────────────────────
// repocreds: GetRepo normalises .git suffix on lookup
//
// Secrets are often registered WITHOUT a trailing ".git" while app repoURLs
// include one (or vice versa). GetRepo must find credentials in both cases.
// ─────────────────────────────────────────────────────────────────────────────

func TestGetRepo_NormalizesGitSuffix(t *testing.T) {
	withGit := "https://github.com/org/repo.git"
	withoutGit := "https://github.com/org/repo"

	fakeRepo := &v1alpha1.Repository{
		Repo:     withoutGit,
		Username: "robot",
		Password: "secret",
	}

	// Build a RepoCreds as if the secret was stored without ".git".
	rc := &RepoCreds{
		reposByURL: map[string]*v1alpha1.Repository{
			normalizeRepoURL(withoutGit): fakeRepo,
		},
	}

	// Lookup with the ".git" form must succeed.
	got := rc.GetRepo(withGit)
	assert.Equal(t, "robot", got.Username,
		"GetRepo with .git suffix must find credentials stored without .git")

	// Lookup with the exact stored form must also succeed.
	got2 := rc.GetRepo(withoutGit)
	assert.Equal(t, "robot", got2.Username,
		"GetRepo without .git suffix must find credentials stored without .git")

	// Unknown URL must return a bare stub (no panic, no credentials).
	unknown := rc.GetRepo("https://github.com/other-org/other-repo.git")
	assert.Equal(t, "https://github.com/other-org/other-repo.git", unknown.Repo)
	assert.Empty(t, unknown.Username)
}

func TestNormalizeRepoURL(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"https://github.com/org/repo.git", "https://github.com/org/repo"},
		{"https://github.com/org/repo", "https://github.com/org/repo"},
		{"HTTPS://GitHub.com/Org/Repo.git", "https://github.com/org/repo"},
		{"https://github.com/org/repo.git.git", "https://github.com/org/repo.git"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, normalizeRepoURL(tc.input), "input: %s", tc.input)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// 10. Same-repo source with prRepo as owner/repo slug → streams locally
//
//	When the user passes --repo=owner/repo (the documented format), the
//	source repoURL is a full URL like "https://github.com/owner/repo.git".
//	The comparison must recognise the slug as belonging to the same repository.
//
// ─────────────────────────────────────────────────────────────────────────────
func TestBuildManifestRequest_SameRepoSource_WithSlugPrRepo_StreamsLocally(t *testing.T) {
	// prRepo is the short "owner/repo" slug — the documented format for --repo.
	prRepo := "aaronshifman/my-private-repo"
	branchFolder := makeBranchFolder(t, "apps/debezium/debezium")

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: debezium-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/aaronshifman/my-private-repo.git
    path: apps/debezium/debezium
    plugin:
      name: kustomize-build-with-helm-oci
    targetRevision: main
  destination:
    namespace: debezium-operator
    server: https://kubernetes.default.svc
`)

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, manifestRequestRenderContext{
		repoSelector: testRepoSelector(t, prRepo)})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// CRITICAL: must stream locally — the source is in the same repo.
	// Before the fix, the slug format caused a mismatch and fell into the
	// remote RPC path, which failed with "authentication required".
	assert.Equal(t, filepath.Join(branchFolder, "apps", "debezium", "debezium"), streamDir,
		"REGRESSION: same-repo source with slug-format prRepo must stream locally, not use remote RPC")
	assert.Empty(t, req.ApplicationSource.Path)
	assertDefaultProjectFields(t, req)
}

func TestBuildManifestRequest_SingleSource_Kustomize_StreamsBranchRoot(t *testing.T) {
	branchFolder := makeBranchFolder(t, "apps/my-app")
	require.NoError(t, os.Remove(filepath.Join(branchFolder, "apps", "my-app", "Chart.yaml")))
	require.NoError(t, os.WriteFile(filepath.Join(branchFolder, "apps", "my-app", "kustomization.yaml"), []byte("resources: []\n"), 0o644))

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

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil,
		manifestRequestRenderContext{repoSelector: testRepoSelector(t, "https://github.com/org/repo.git")})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	assert.Equal(t, branchFolder, streamDir, "kustomize sources should keep branch root so sibling references still work")
	assert.Equal(t, "apps/my-app", req.ApplicationSource.Path)
	assertDefaultProjectFields(t, req)
}

func TestBuildManifestRequest_SingleSource_KustomizeWithHelm_StreamsBranchRoot(t *testing.T) {
	branchFolder := makeBranchFolder(t, "apps/my-app")
	require.NoError(t, os.WriteFile(filepath.Join(branchFolder, "apps", "my-app", "kustomization.yaml"), []byte(`
helmCharts:
  - name: nginx
    repo: https://charts.bitnami.com/bitnami
    version: 15.0.0
`), 0o644))

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
    kustomize:
      version: v5.0.0
`)

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil,
		manifestRequestRenderContext{repoSelector: testRepoSelector(t, "https://github.com/org/repo.git")})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	assert.Equal(t, branchFolder, streamDir, "kustomize sources with helmCharts should not be mistaken for local Helm charts")
	assert.Equal(t, "apps/my-app", req.ApplicationSource.Path)
	assertDefaultProjectFields(t, req)
}

func TestBuildManifestRequest_SingleSource_LocalChart_DoesNotStreamUnrelatedSymlinks(t *testing.T) {
	branchFolder := makeBranchFolder(t, "infra/charts/argocd")
	relatedAssetDir := filepath.Join(branchFolder, "assets", "co")
	relatedAppDir := filepath.Join(branchFolder, "src", "apps", "web", "public", "avatars")
	require.NoError(t, os.MkdirAll(relatedAssetDir, 0o755))
	require.NoError(t, os.MkdirAll(relatedAppDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(relatedAssetDir, "avatar.png"), []byte("png"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join("..", "..", "..", "..", "..", "assets", "co", "avatar.png"), filepath.Join(relatedAppDir, "avatar.png")))

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: argocd
spec:
  destination:
    namespace: argocd
  source:
    repoURL: https://github.com/org/repo.git
    path: infra/charts/argocd
    targetRevision: HEAD
`)

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)
	require.Empty(t, refSources)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil,
		manifestRequestRenderContext{repoSelector: testRepoSelector(t, "https://github.com/org/repo.git")})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	assert.Equal(t, filepath.Join(branchFolder, "infra", "charts", "argocd"), streamDir)
	assert.Empty(t, req.ApplicationSource.Path)
	assert.NoDirExists(t, filepath.Join(streamDir, "src"), "unrelated top-level repo paths must not be streamed")
	assertDefaultProjectFields(t, req)
}

func TestBuildManifestRequest_LocalHelmChart_TarballExcludesUnrelatedRepoSymlinks(t *testing.T) {
	baseFolder := createIssue438BranchFolder(t, "base")
	targetFolder := createIssue438BranchFolder(t, "target")

	for _, tc := range []struct {
		name         string
		branchFolder string
	}{
		{name: "base", branchFolder: baseFolder},
		{name: "target", branchFolder: targetFolder},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: argocd
spec:
  destination:
    namespace: argocd
  source:
    repoURL: https://github.com/org/repo.git
    path: infra/charts/argocd
    targetRevision: HEAD
`)

			contentSources, refSources, hasMultipleSources, err := splitSources(app)
			require.NoError(t, err)
			require.Len(t, contentSources, 1)

			req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, tc.branchFolder, nil,
				manifestRequestRenderContext{repoSelector: testRepoSelector(t, "https://github.com/org/repo.git")})
			require.NoError(t, err)
			if cleanup != nil {
				defer cleanup()
			}

			assert.Equal(t, filepath.Join(tc.branchFolder, "infra", "charts", "argocd"), streamDir)
			assert.Empty(t, req.ApplicationSource.Path)

			tarEntries := compressAndListEntries(t, streamDir)
			assert.Contains(t, tarEntries, "Chart.yaml")
			assert.Contains(t, tarEntries, "values.yaml")
			assert.NotContains(t, tarEntries, "src/apps/web/public/avatars/avatar.png")
			assert.NotContains(t, tarEntries, "assets/co/avatar.png")
		})
	}
}

func createIssue438BranchFolder(t *testing.T, name string) string {
	t.Helper()
	branchFolder := filepath.Join(t.TempDir(), name)
	chartDir := filepath.Join(branchFolder, "infra", "charts", "argocd")
	assetDir := filepath.Join(branchFolder, "assets", "co")
	avatarDir := filepath.Join(branchFolder, "src", "apps", "web", "public", "avatars")
	require.NoError(t, os.MkdirAll(chartDir, 0o755))
	require.NoError(t, os.MkdirAll(assetDir, 0o755))
	require.NoError(t, os.MkdirAll(avatarDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte("apiVersion: v2\nname: argocd\nversion: 0.1.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("replicas: 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetDir, "avatar.png"), []byte("png"), 0o644))
	require.NoError(t, os.Symlink(filepath.Join("..", "..", "..", "..", "..", "assets", "co", "avatar.png"), filepath.Join(avatarDir, "avatar.png")))
	return branchFolder
}

func compressAndListEntries(t *testing.T, dir string) []string {
	t.Helper()
	tgzFile, _, _, err := tgzstream.CompressFiles(dir, []string{"*"}, []string{".git"})
	require.NoError(t, err)
	defer tgzstream.CloseAndDelete(tgzFile)

	_, err = tgzFile.Seek(0, io.SeekStart)
	require.NoError(t, err)
	gzipReader, err := gzip.NewReader(tgzFile)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, gzipReader.Close())
	}()

	tarReader := tar.NewReader(gzipReader)
	var entries []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		entries = append(entries, header.Name)
	}
	sort.Strings(entries)
	return entries
}

// ─────────────────────────────────────────────────────────────────────────────
// collectRepoURLs
// ─────────────────────────────────────────────────────────────────────────────

func TestCollectRepoURLs_SingleSource(t *testing.T) {
	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/my-app
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`)
	urls := collectRepoURLs([]argoapplication.ArgoResource{app})
	assert.Equal(t, []string{"https://github.com/org/repo.git"}, urls)
}

func TestCollectRepoURLs_MultiSource(t *testing.T) {
	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  sources:
    - repoURL: https://github.com/org/repo.git
      path: apps/my-app
      ref: values
    - repoURL: https://charts.jetstack.io
      chart: cert-manager
      targetRevision: "1.12.0"
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`)
	urls := collectRepoURLs([]argoapplication.ArgoResource{app})
	assert.Len(t, urls, 2)
	assert.Contains(t, urls, "https://github.com/org/repo.git")
	assert.Contains(t, urls, "https://charts.jetstack.io")
}

func TestCollectRepoURLs_Deduplication(t *testing.T) {
	app1 := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app1
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/org/repo.git
    path: apps/app1
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`)
	app2 := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: app2
  namespace: argocd
spec:
  source:
    repoURL: https://github.com/org/repo
    path: apps/app2
  destination:
    server: https://kubernetes.default.svc
    namespace: default
`)
	// Same URL with and without .git — should deduplicate to one entry
	urls := collectRepoURLs(
		[]argoapplication.ArgoResource{app1},
		[]argoapplication.ArgoResource{app2},
	)
	assert.Len(t, urls, 1)
}

func TestCollectRepoURLs_Empty(t *testing.T) {
	urls := collectRepoURLs([]argoapplication.ArgoResource{})
	assert.Empty(t, urls)
}
