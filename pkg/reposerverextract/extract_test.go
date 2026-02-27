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
	"os"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)
	require.Empty(t, refSources)
	assert.False(t, hasMultipleSources)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, "")
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

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, "")
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

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, "")
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
	// and path set is treated as a content source (not a ref source) by the
	// split logic in splitSources.
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

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, "")
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

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)
	require.Len(t, refSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, "")
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
	for i, cs := range contentSources {
		req, streamDir, cleanup, buildErr := buildManifestRequestForSource(app, cs, refSources, hasMultipleSources, branchFolder, nil, "")
		require.NoError(t, buildErr, "content source %d should not error", i)
		if cleanup != nil {
			defer cleanup()
		}

		// Both are local path sources with no refs → fast path (stream branchFolder).
		assert.Equal(t, branchFolder, streamDir, "content source %d should stream the branch folder", i)
		assert.True(t, req.HasMultipleSources, "HasMultipleSources must be true for both requests")
		assert.Equal(t, "argocd", req.Namespace)
	}

	// Verify the paths are correctly assigned to each request.
	req0, _, cleanup0, _ := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, "")
	if cleanup0 != nil {
		defer cleanup0()
	}
	req1, _, cleanup1, _ := buildManifestRequestForSource(app, contentSources[1], refSources, hasMultipleSources, branchFolder, nil, "")
	if cleanup1 != nil {
		defer cleanup1()
	}
	assert.Equal(t, "management-prod/applicationsets", req0.ApplicationSource.Path)
	assert.Equal(t, "management-prod/root", req1.ApplicationSource.Path)
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

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, prRepo)
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// CRITICAL: streamDir must be empty — the path lives in a foreign repo.
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

	req, streamDir, cleanup, err := buildManifestRequestForSource(app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil, prRepo)
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// Same repo — should stream the branch folder, not use remote RPC.
	assert.Equal(t, branchFolder, streamDir, "same-repo source must still stream locally even when prRepo is set")
	assert.Equal(t, "apps/my-app", req.ApplicationSource.Path)
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
