package reposerverextract

// Source-combination matrix - Table A (single-source `spec.source`).
//
// This is the table-driven companion to SOURCE_MATRIX.md. Each row below is one
// valid way a single-source Application can describe its source. The test feeds
// the Application through buildManifestRequestForSource and asserts the ROUTING
// OUTCOME only:
//
//   - strategy: which RPC the caller will use (remote vs streamed tarball),
//     expressed via streamDir ("" => remote GenerateManifest, non-"" => stream).
//   - streamRoot: for streamed sources, whether we stream just the chart dir or
//     the whole branch root (and therefore whether Path is cleared or kept).
//   - request fields that are repo-server-api specific and have caused bugs
//     (Chart, KubeVersion/ApiVersions, ProjectName/ProjectSourceRepos).
//
// We deliberately assert OUTCOME, not call order or internal helpers, so this
// table stays valid if the routing is ever simplified (e.g. "always stream the
// branch root"). See SOURCE_MATRIX.md for the issue cross-references.
//
// Rows A9/A10/A11 (symlink behavior) are split out into their own tests at the
// bottom of this file (TestSourceMatrix_TableA9/A10/A11_*) because they need
// on-disk symlinks and tarball/copyDir-level assertions rather than the plain
// routing table. A10 is integration-only (skipped placeholder); A9 and A11 are
// unit-testable. Every A-row therefore has a named test in this file.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// streamStrategy is the observable routing outcome for a content source.
type streamStrategy int

const (
	// strategyRemote: streamDir == "" -> caller uses the unary GenerateManifest
	// RPC and the repo server fetches the content itself.
	strategyRemote streamStrategy = iota
	// strategyStreamChartDir: stream only the chart directory; request Path is
	// cleared because the chart dir is the tarball root.
	strategyStreamChartDir
	// strategyStreamBranchRoot: stream the whole branch folder; request Path is
	// kept so the repo server resolves it relative to the repo root.
	strategyStreamBranchRoot
	// strategyStreamRefs: stream a synthesized temp tree that holds the content
	// source plus a .refs/<refName>/ directory per ref source, with $ref/...
	// value files rewritten to relative paths. streamDir is a temp dir that is
	// neither "" nor the branch folder and contains a .refs/ subdirectory.
	strategyStreamRefs
)

func (s streamStrategy) String() string {
	switch s {
	case strategyRemote:
		return "remote"
	case strategyStreamChartDir:
		return "stream(chart-dir)"
	case strategyStreamBranchRoot:
		return "stream(branch-root)"
	case strategyStreamRefs:
		return "stream(.refs)"
	default:
		return "unknown"
	}
}

// singleSourceCase is one row of Table A: a single-source Application plus the
// on-disk files it needs and the routing outcome we expect.
type singleSourceCase struct {
	name string // matrix id + description, e.g. "A2 local chart, no values"

	// appYAML is the Application manifest under test. Use {{REPO}} as a
	// placeholder for the PR repo URL so cross-repo rows are explicit.
	appYAML string

	// files to create under branchFolder before routing (relative paths ->
	// contents). The chart/kustomize marker files live here.
	files map[string]string

	// prRepo is what --repo resolves to. Defaults to the same repo the source
	// uses; set to a different value to model a cross-repo source (A15).
	prRepo string

	want       streamStrategy
	wantChart  string // expected request.ApplicationSource.Chart
	wantPath   string // expected request.ApplicationSource.Path ("" => cleared)
	wantNoRefs bool   // RefSources must be nil for single-source rows
}

func TestSourceMatrix_TableA_SingleSource_Routing(t *testing.T) {
	const repo = "https://github.com/org/repo.git"

	cases := []singleSourceCase{
		{
			name: "A1 plain local dir (manifests) -> stream branch root",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a1}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: apps/plain
    targetRevision: HEAD
`,
			files:      map[string]string{"apps/plain/deployment.yaml": "kind: Deployment\n"},
			want:       strategyStreamBranchRoot,
			wantPath:   "apps/plain",
			wantNoRefs: true,
		},
		{
			name: "A2 local Helm chart, no values -> stream chart dir",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a2}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: charts/app
    targetRevision: HEAD
`,
			files:      map[string]string{"charts/app/Chart.yaml": "apiVersion: v2\nname: app\nversion: 0.1.0\n"},
			want:       strategyStreamChartDir,
			wantPath:   "",
			wantNoRefs: true,
		},
		{
			name: "A3 local Helm chart, in-chart values -> stream chart dir",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a3}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: charts/app
    targetRevision: HEAD
    helm:
      valueFiles:
        - values.yaml
        - overrides/prod.yaml
`,
			files: map[string]string{
				"charts/app/Chart.yaml":          "apiVersion: v2\nname: app\nversion: 0.1.0\n",
				"charts/app/values.yaml":         "k: v\n",
				"charts/app/overrides/prod.yaml": "k: v\n",
			},
			want:       strategyStreamChartDir,
			wantPath:   "",
			wantNoRefs: true,
		},
		{
			name: "A4 local Helm chart, out-of-chart relative values -> stream branch root",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a4}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: charts/app
    targetRevision: HEAD
    helm:
      valueFiles:
        - ../shared/values.yaml
`,
			files: map[string]string{
				"charts/app/Chart.yaml":    "apiVersion: v2\nname: app\nversion: 0.1.0\n",
				"charts/shared/values.yaml": "k: v\n",
			},
			want:       strategyStreamBranchRoot,
			wantPath:   "charts/app",
			wantNoRefs: true,
		},
		{
			name: "A5 local Helm chart, out-of-chart absolute values -> stream branch root",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a5}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: charts/app
    targetRevision: HEAD
    helm:
      valueFiles:
        - /env/prod/values.yaml
`,
			files: map[string]string{
				"charts/app/Chart.yaml":   "apiVersion: v2\nname: app\nversion: 0.1.0\n",
				"env/prod/values.yaml":    "k: v\n",
			},
			want:       strategyStreamBranchRoot,
			wantPath:   "charts/app",
			wantNoRefs: true,
		},
		{
			name: "A6 local Helm chart, remote-URL values -> stream chart dir (URL ignored)",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a6}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: charts/app
    targetRevision: HEAD
    helm:
      valueFiles:
        - https://example.com/values.yaml
`,
			files:      map[string]string{"charts/app/Chart.yaml": "apiVersion: v2\nname: app\nversion: 0.1.0\n"},
			want:       strategyStreamChartDir,
			wantPath:   "",
			wantNoRefs: true,
		},
		{
			name: "A7 local Kustomize -> stream branch root",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a7}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: overlays/prod
    targetRevision: HEAD
`,
			files:      map[string]string{"overlays/prod/kustomization.yaml": "resources: []\n"},
			want:       strategyStreamBranchRoot,
			wantPath:   "overlays/prod",
			wantNoRefs: true,
		},
		{
			name: "A8 local Kustomize with helmCharts (Chart.yaml present) -> stream branch root",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a8}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: overlays/prod
    targetRevision: HEAD
`,
			files: map[string]string{
				"overlays/prod/kustomization.yaml": "helmCharts:\n  - name: x\n",
				"overlays/prod/Chart.yaml":         "apiVersion: v2\nname: x\nversion: 0.1.0\n",
			},
			want:       strategyStreamBranchRoot,
			wantPath:   "overlays/prod",
			wantNoRefs: true,
		},
		{
			name: "A12 local plugin (CMP) path -> stream branch root",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a12}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: apps/plugin
    targetRevision: HEAD
    plugin:
      name: my-plugin
`,
			files:      map[string]string{"apps/plugin/manifest.yaml": "kind: ConfigMap\n"},
			want:       strategyStreamBranchRoot,
			wantPath:   "apps/plugin",
			wantNoRefs: true,
		},
		{
			name: "A13 remote Helm chart -> remote RPC",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a13}
spec:
  destination: {namespace: cert-manager}
  source:
    repoURL: https://charts.jetstack.io
    chart: cert-manager
    targetRevision: v1.14.5
`,
			files:      nil,
			want:       strategyRemote,
			wantChart:  "cert-manager",
			wantNoRefs: true,
		},
		{
			name: "A14 remote Helm chart (OCI) -> remote RPC",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a14}
spec:
  destination: {namespace: default}
  source:
    repoURL: oci://registry-1.docker.io/bitnamicharts
    chart: nginx
    targetRevision: "15.0.0"
`,
			files:      nil,
			want:       strategyRemote,
			wantChart:  "nginx",
			wantNoRefs: true,
		},
		{
			name: "A15 local path source but cross-repo -> remote RPC",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a15}
spec:
  destination: {namespace: default}
  source:
    repoURL: https://github.com/other/foreign.git
    path: charts/app
    targetRevision: HEAD
`,
			// Files exist locally, but the source repo != --repo, so they must
			// NOT be streamed; the repo server fetches the foreign repo itself.
			// Path is kept (not cleared) because the repo server resolves it
			// inside the fetched foreign repo - only the stream-chart-dir
			// strategy clears Path.
			files:      map[string]string{"charts/app/Chart.yaml": "apiVersion: v2\nname: app\nversion: 0.1.0\n"},
			prRepo:     repo,
			want:       strategyRemote,
			wantPath:   "charts/app",
			wantNoRefs: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			branchFolder := writeBranchFiles(t, tc.files)

			prRepo := tc.prRepo
			if prRepo == "" {
				prRepo = repo
			}
			app := makeApp(t, replaceRepo(tc.appYAML, repo))

			contentSources, refSources, hasMultipleSources, err := splitSources(app)
			require.NoError(t, err)
			require.Len(t, contentSources, 1, "table A rows are single content source")

			req, streamDir, cleanup, err := buildManifestRequestForSource(
				app, contentSources[0], refSources, hasMultipleSources, branchFolder, nil,
				manifestRequestRenderContext{
					repoSelector: testRepoSelector(t, prRepo),
					kubeVersion:  "v1.30.1",
					apiVersions:  []string{"apps/v1", "v1"},
				},
			)
			require.NoError(t, err)
			if cleanup != nil {
				defer cleanup()
			}

			gotStrategy := classifyStrategy(streamDir, branchFolder)
			assert.Equalf(t, tc.want, gotStrategy,
				"strategy mismatch: want %s, got %s (streamDir=%q)", tc.want, gotStrategy, streamDir)

			assert.Equal(t, tc.wantChart, req.ApplicationSource.Chart, "Chart field")
			assert.Equal(t, tc.wantPath, req.ApplicationSource.Path, "Path field")
			if tc.wantNoRefs {
				assert.Nil(t, req.RefSources, "single-source rows must not set RefSources")
			}

			// Repo-server-api request invariants that must hold on EVERY strategy
			// (these are the roots of #432 and #416).
			assert.Equal(t, "v1.30.1", req.KubeVersion, "KubeVersion must be set (#432)")
			assert.Equal(t, []string{"apps/v1", "v1"}, req.ApiVersions, "ApiVersions must be set (#432)")
			assertDefaultProjectFields(t, req)
		})
	}
}

// classifyStrategy maps the (streamDir, branchFolder) outcome back to a
// streamStrategy so the table can assert intent rather than raw paths.
func classifyStrategy(streamDir, branchFolder string) streamStrategy {
	if streamDir == "" {
		return strategyRemote
	}
	if streamDir == branchFolder {
		return strategyStreamBranchRoot
	}
	// A synthesized temp tree for ref handling contains a .refs/ directory; a
	// narrowed chart-dir stream does not.
	if info, err := os.Stat(filepath.Join(streamDir, ".refs")); err == nil && info.IsDir() {
		return strategyStreamRefs
	}
	return strategyStreamChartDir
}

// writeBranchFiles creates a temp branch folder and writes the given relative
// files into it, returning the folder path.
func writeBranchFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}
	return dir
}

func replaceRepo(yaml, repo string) string {
	return strings.ReplaceAll(yaml, "{{REPO}}", repo)
}

// ── A9: local chart with an unrelated in-bounds symlink elsewhere in the repo ──
//
// Routing a local chart must succeed even when the surrounding repo contains
// symlinks, and narrowing the stream to the chart dir must keep those unrelated
// files (and the symlinks pointing at them) out of the tarball. The unit-
// observable part - the tarball contents - is asserted here. The repo server's
// own symlink safety gate (#438) is the part that only a real render exercises;
// that is the I half of this U+I row and lives in the integration suite (see
// also TestBuildManifestRequest_LocalHelmChart_TarballExcludesUnrelatedRepoSymlinks).
func TestSourceMatrix_TableA9_InBoundsSymlink_RoutesAndExcludesUnrelated(t *testing.T) {
	const repo = "https://github.com/org/repo.git"

	branchFolder := t.TempDir()
	chartDir := filepath.Join(branchFolder, "apps", "chart")
	assetDir := filepath.Join(branchFolder, "assets")
	require.NoError(t, os.MkdirAll(chartDir, 0o755))
	require.NoError(t, os.MkdirAll(assetDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"),
		[]byte("apiVersion: v2\nname: chart\nversion: 0.1.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte("replicas: 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(assetDir, "logo.png"), []byte("PNG"), 0o644))
	// An unrelated symlink elsewhere in the repo, pointing in-bounds at the asset.
	require.NoError(t, os.Symlink(filepath.Join(assetDir, "logo.png"), filepath.Join(branchFolder, "logo-link.png")))

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a9}
spec:
  destination: {namespace: default}
  source:
    repoURL: `+repo+`
    path: apps/chart
    targetRevision: HEAD
`)

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(
		app, contentSources[0], refSources, hasMultipleSources, branchFolder, &RepoCreds{},
		manifestRequestRenderContext{
			repoSelector: testRepoSelector(t, repo),
			kubeVersion:  "v1.30.1",
			apiVersions:  []string{"apps/v1", "v1"},
		},
	)
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}

	// Routes to the chart dir (the unrelated symlink does not force the branch root).
	require.Equal(t, strategyStreamChartDir, classifyStrategy(streamDir, branchFolder))
	assert.Empty(t, req.ApplicationSource.Path)

	entries := compressAndListEntries(t, streamDir)
	assert.Contains(t, entries, "Chart.yaml")
	assert.Contains(t, entries, "values.yaml")
	assert.NotContains(t, entries, "logo-link.png", "unrelated repo symlink must not be in the chart tarball")
	assert.NotContains(t, entries, "assets/logo.png", "unrelated repo asset must not be in the chart tarball")
}

// ── A10: local chart with an OUT-OF-BOUNDS file symlink (#438) ────────────────
//
// A symlink that escapes the repository (e.g. /etc/passwd or ../../outside) is
// rejected by the repo server's symlink safety check during render. That
// rejection happens INSIDE the repo server, so it cannot be observed by a unit
// test that only inspects the request/tarball we hand off. This row is recorded
// as a skipped integration placeholder so the matrix has a named entry for it;
// the real coverage belongs in the integration suite (branch-N) with an
// out-of-bounds symlink fixture.
func TestSourceMatrix_TableA10_OutOfBoundsSymlink_Integration(t *testing.T) {
	t.Skip("A10 is integration-only (#438): the repo server's out-of-bounds symlink gate is only exercised by a real render (branch-N)")
}

// ── A11: local chart containing a DIRECTORY symlink (#448, fixed by #449) ──────
//
// A chart that contains a directory symlink must route successfully (it used to
// fail at the copyDir level with EISDIR because filepath.Walk+Lstat treated the
// dir symlink as a file). At the single-source matrix level the chart dir is
// streamed via the tarball compressor, which preserves the directory symlink as
// a symlink entry; the important assertion is that routing SUCCEEDS rather than
// erroring. The copyDir directory-symlink-following behavior used on the ref
// slow path is additionally covered by TestCopyDir_FollowsDirectorySymlink.
func TestSourceMatrix_TableA11_DirectorySymlinkInChart_RoutesSuccessfully(t *testing.T) {
	const repo = "https://github.com/org/repo.git"

	branchFolder := t.TempDir()
	chartDir := filepath.Join(branchFolder, "apps", "chart")
	externalDir := filepath.Join(branchFolder, "shared", "sql")
	require.NoError(t, os.MkdirAll(filepath.Join(chartDir, "files"), 0o755))
	require.NoError(t, os.MkdirAll(externalDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"),
		[]byte("apiVersion: v2\nname: chart\nversion: 0.1.0\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(externalDir, "V1__init.sql"), []byte("SELECT 1;"), 0o644))
	// A DIRECTORY symlink inside the chart, pointing at an in-repo directory.
	require.NoError(t, os.Symlink(externalDir, filepath.Join(chartDir, "files", "sql")))

	app := makeApp(t, `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: a11}
spec:
  destination: {namespace: default}
  source:
    repoURL: `+repo+`
    path: apps/chart
    targetRevision: HEAD
`)

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1)

	req, streamDir, cleanup, err := buildManifestRequestForSource(
		app, contentSources[0], refSources, hasMultipleSources, branchFolder, &RepoCreds{},
		manifestRequestRenderContext{
			repoSelector: testRepoSelector(t, repo),
			kubeVersion:  "v1.30.1",
			apiVersions:  []string{"apps/v1", "v1"},
		},
	)
	// The key regression guard: routing must NOT error on a directory symlink.
	require.NoError(t, err, "a chart containing a directory symlink must route without error (#448)")
	if cleanup != nil {
		defer cleanup()
	}

	require.Equal(t, strategyStreamChartDir, classifyStrategy(streamDir, branchFolder))
	assert.Empty(t, req.ApplicationSource.Path)

	// The chart streams successfully and the directory symlink is present.
	entries := compressAndListEntries(t, streamDir)
	assert.Contains(t, entries, "Chart.yaml")
	assert.Contains(t, entries, "files/sql", "directory symlink must be present in the streamed chart")
}
