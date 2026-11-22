package reposerverextract

// Source-combination matrix - Tables C, D, E.
//
// Companion to SOURCE_MATRIX.md sections C/D/E. These cover the axes that are
// orthogonal to the per-source routing in Tables A/B:
//
//   - Table C: cardinality / kind. Does an ApplicationSet route the same as the
//     equivalent Application, and do the degenerate source topologies (no
//     source, all-ref-only) fail loudly?
//   - Table D: app-of-apps traversal. A discovered child Application must route
//     through buildManifestRequestForSource exactly like a seed Application.
//     (The --redirect-target-revisions behavior for children, TRV2/TRV3 / #446, is
//     already covered by appofapps_test.go's BuildChildApplication_* tests and
//     is NOT duplicated here.)
//   - Table E: cross-cutting request-content correctness. The repo-server-api
//     request invariants (#432 KubeVersion/ApiVersions, #416
//     ProjectName/ProjectSourceRepos) must hold on ALL THREE strategies, not
//     just the streamed local-chart one that historically asserted them.
//
// REQ3 (transient helm-dep build error surfaced, #416) only fails inside a real
// repo server and is an integration concern; it is represented below as a
// skipped integration placeholder (TestSourceMatrix_RequestInvariants_HelmDepBuildErrorSurfaced_*) so every E-row
// has a named test.

import (
	"testing"

	"github.com/dag-andersen/argocd-diff-preview/pkg/git"
	"github.com/dag-andersen/argocd-diff-preview/pkg/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ── Table C: cardinality / kind ─────────────────────────────────────────────

// STR5: an ApplicationSet whose sources live under spec.template.spec must split
// and route identically to the equivalent Application. We build the same
// logical multi-source app in both shapes and assert the routing outcome
// matches source-for-source.
func TestSourceMatrix_Strategy_AppSetRoutesLikeApp(t *testing.T) {
	const repo = "https://github.com/org/repo.git"

	applicationYAML := `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: c1-app}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: https://github.com/other/values.git
      ref: ext
      targetRevision: HEAD
    - repoURL: ` + repo + `
      path: apps/chart
      targetRevision: HEAD
      helm:
        valueFiles:
          - $ext/values.yaml
`
	applicationSetYAML := `
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata: {name: c1-appset}
spec:
  generators:
    - list:
        elements: []
  template:
    metadata: {name: c1-child}
    spec:
      destination: {namespace: default}
      sources:
        - repoURL: https://github.com/other/values.git
          ref: ext
          targetRevision: HEAD
        - repoURL: ` + repo + `
          path: apps/chart
          targetRevision: HEAD
          helm:
            valueFiles:
              - $ext/values.yaml
`
	files := map[string]string{
		"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
	}

	route := func(t *testing.T, rawYAML string) (streamStrategy, []string) {
		t.Helper()
		branchFolder := writeBranchFiles(t, files)
		app := makeApp(t, rawYAML)

		contentSources, refSources, hasMultipleSources, err := splitSources(app)
		require.NoError(t, err)
		require.Len(t, contentSources, 1, "single content source expected")

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
		return classifyStrategy(streamDir, branchFolder), refKeysOf(req.RefSources)
	}

	appStrategy, appRefKeys := route(t, applicationYAML)
	setStrategy, setRefKeys := route(t, applicationSetYAML)

	assert.Equal(t, appStrategy, setStrategy,
		"ApplicationSet must route to the same strategy as the equivalent Application")
	assert.ElementsMatch(t, appRefKeys, setRefKeys,
		"ApplicationSet must produce the same RefSources keys as the equivalent Application")
	// External ref -> both must be remote with a $ext RefSource.
	assert.Equal(t, strategyRemote, appStrategy)
	assert.ElementsMatch(t, []string{"$ext"}, appRefKeys)
}

// NEG1/NEG2: degenerate source topologies must fail loudly in splitSources rather
// than silently producing an empty/invalid request.
func TestSourceMatrix_Negatives_DegenerateTopologies(t *testing.T) {
	cases := []struct {
		name    string
		appYAML string
	}{
		{
			name: "NEG1 neither source nor sources -> error",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: c2}
spec:
  destination: {namespace: default}
`,
		},
		{
			name: "NEG2 all sources are ref-only (no content) -> error",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: c3}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: https://github.com/org/repo.git
      ref: a
      targetRevision: HEAD
    - repoURL: https://github.com/org/repo.git
      ref: b
      targetRevision: HEAD
`,
		},
		{
			name: "NEG3 empty sources list -> error",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: c3b}
spec:
  destination: {namespace: default}
  sources: []
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := makeApp(t, tc.appYAML)
			_, _, _, err := splitSources(app)
			assert.Error(t, err, "degenerate source topology must return an error")
		})
	}
}

// ── Table D: app-of-apps traversal ──────────────────────────────────────────

// TRV1: a discovered child Application must route through
// buildManifestRequestForSource exactly like a seed Application. This ties the
// app-of-apps path back into the source matrix: whatever a child's source is,
// the same routing rules (stream local chart dir, remote chart -> remote RPC,
// etc.) apply.
//
// (TRV2 and TRV3 below cover the --redirect-target-revisions behavior for children,
// #446. There is overlapping coverage in appofapps_test.go's
// TestBuildChildApplication_* tests, but the matrix rows are kept here so every
// row in SOURCE_MATRIX.md has a corresponding, named test in the matrix suite.)
func TestSourceMatrix_Traversal_ChildRoutesLikeSeed(t *testing.T) {
	const repo = "https://github.com/org/repo.git"

	cases := []struct {
		name      string
		childYAML string
		files     map[string]string
		want      streamStrategy
		wantChart string
	}{
		{
			name: "TRV1a child local chart -> stream(chart-dir)",
			childYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: child-local}
spec:
  destination: {namespace: default}
  source:
    repoURL: ` + repo + `
    path: apps/child
    targetRevision: HEAD
`,
			files: map[string]string{
				"apps/child/Chart.yaml": "apiVersion: v2\nname: child\nversion: 0.1.0\n",
			},
			want:      strategyStreamChartDir,
			wantChart: "",
		},
		{
			name: "TRV1b child remote chart -> remote RPC",
			childYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: child-remote}
spec:
  destination: {namespace: default}
  source:
    repoURL: https://charts.example.com
    chart: child-chart
    targetRevision: "1.0.0"
`,
			files:     nil,
			want:      strategyRemote,
			wantChart: "child-chart",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			branchFolder := writeBranchFiles(t, tc.files)
			// A discovered child is just an Application manifest; feed it through
			// the same split + route path a seed app takes.
			child := makeApp(t, tc.childYAML)

			contentSources, refSources, hasMultipleSources, err := splitSources(child)
			require.NoError(t, err)
			require.Len(t, contentSources, 1)

			req, streamDir, cleanup, err := buildManifestRequestForSource(
				child, contentSources[0], refSources, hasMultipleSources, branchFolder, &RepoCreds{},
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

			assert.Equal(t, tc.want, classifyStrategy(streamDir, branchFolder),
				"child must route like a seed app")
			assert.Equal(t, tc.wantChart, req.ApplicationSource.Chart, "Chart field")
			assertDefaultProjectFields(t, req)
		})
	}
}

// TRV2: a discovered child Application must honor --redirect-target-revisions
// exactly like a seed Application (#446, fixed by #447). A child source whose
// targetRevision is in the redirect list is redirected to the target branch; a
// source pinned to a revision outside the list is left untouched; an empty list
// preserves the "redirect everything" default.
//
// childAppManifestTRV2 builds a minimal discovered child whose single source is
// pinned to targetRevision. (Named with a TRV2 suffix to avoid colliding with the
// childAppManifest helper in appofapps_test.go.)
func childAppManifestTRV2(repoURL, targetRevision string) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"name": "d2-child"},
		"spec": map[string]any{
			"source": map[string]any{
				"repoURL":        repoURL,
				"path":           "apps/child",
				"targetRevision": targetRevision,
			},
		},
	}}
}

func TestSourceMatrix_Traversal_ChildHonorsRedirectTargetRevisions(t *testing.T) {
	const repoURL = "https://github.com/example/repo"

	cases := []struct {
		name              string
		pinnedRevision    string
		redirectRevisions []string
		wantRevision      string
	}{
		{
			name:              "TRV2a revision in list -> redirected to target branch",
			pinnedRevision:    "main",
			redirectRevisions: []string{"main", "HEAD"},
			wantRevision:      "target",
		},
		{
			name:              "TRV2b revision NOT in list -> left untouched",
			pinnedRevision:    "feature-branch",
			redirectRevisions: []string{"main", "HEAD"},
			wantRevision:      "feature-branch",
		},
		{
			name:              "TRV2c empty list -> redirect everything (default)",
			pinnedRevision:    "feature-branch",
			redirectRevisions: nil,
			wantRevision:      "target",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			selector, err := repository.NewSelector(repoURL, "")
			require.NoError(t, err)
			targetBranch := git.NewBranch("target", git.Target)

			manifest := childAppManifestTRV2(repoURL, tc.pinnedRevision)
			child, err := buildChildApplication(
				"argocd", manifest, "parent: root", git.Target, targetBranch, *selector, tc.redirectRevisions)
			require.NoError(t, err)
			require.NotNil(t, child)

			got, found, err := unstructured.NestedString(child.Yaml.Object, "spec", "source", "targetRevision")
			require.NoError(t, err)
			require.True(t, found, "child source must keep a targetRevision")
			assert.Equal(t, tc.wantRevision, got,
				"child targetRevision after applying redirect list %v", tc.redirectRevisions)
		})
	}
}

// TRV3: a child Application whose source is pinned to a ref that only exists on
// the remote (not in the local checkout) must render from the remote rather
// than being redirected to the local branch. This is a consequence of TRV2/#446,
// but the failure mode (the repo server cloning a remote-only ref) is only
// observable against a real repo server, so it is an INTEGRATION row, not a
// unit row. It is recorded here as a skipped placeholder so the matrix has a
// named entry for every row in SOURCE_MATRIX.md; the real coverage lives in the
// integration suite (branch-N).
func TestSourceMatrix_Traversal_ChildRemoteOnlyRef_Integration(t *testing.T) {
	t.Skip("TRV3 is integration-only: a remote-only child ref must be rendered by a real repo server (branch-N), not a unit test")
}

// ── Table E: cross-cutting request-content correctness ───────────────────────

// REQ1/REQ2: the repo-server-api request invariants must hold on EVERY strategy.
// Historically only the streamed local-chart path asserted KubeVersion/
// ApiVersions (#432) and ProjectName/ProjectSourceRepos (#416); this test
// pins them across remote, stream(chart-dir) and stream(.refs) so a future
// routing change cannot drop them on one path.
func TestSourceMatrix_RequestInvariants_AllStrategies(t *testing.T) {
	const repo = "https://github.com/org/repo.git"
	const kubeVersion = "v1.31.2"
	apiVersions := []string{"apps/v1", "networking.k8s.io/v1", "v1"}

	cases := []struct {
		name    string
		appYAML string
		files   map[string]string
		want    streamStrategy
	}{
		{
			name: "E remote chart (strategy R)",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: e-remote}
spec:
  destination: {namespace: default}
  source:
    repoURL: https://charts.example.com
    chart: nginx
    targetRevision: "1.0.0"
`,
			files: nil,
			want:  strategyRemote,
		},
		{
			name: "E local chart (strategy S, chart dir)",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: e-local}
spec:
  destination: {namespace: default}
  source:
    repoURL: ` + repo + `
    path: apps/chart
    targetRevision: HEAD
`,
			files: map[string]string{
				"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
			},
			want: strategyStreamChartDir,
		},
		{
			name: "E local chart + local ref (strategy S+refs)",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: e-refs}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: ` + repo + `
      ref: cfg
      targetRevision: HEAD
    - repoURL: ` + repo + `
      path: apps/chart
      targetRevision: HEAD
      helm:
        valueFiles:
          - $cfg/values.yaml
`,
			files: map[string]string{
				"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
				"values.yaml":           "k: v\n",
			},
			want: strategyStreamRefs,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			branchFolder := writeBranchFiles(t, tc.files)
			app := makeApp(t, tc.appYAML)

			contentSources, refSources, hasMultipleSources, err := splitSources(app)
			require.NoError(t, err)
			require.Len(t, contentSources, 1)

			req, streamDir, cleanup, err := buildManifestRequestForSource(
				app, contentSources[0], refSources, hasMultipleSources, branchFolder, &RepoCreds{},
				manifestRequestRenderContext{
					repoSelector: testRepoSelector(t, repo),
					kubeVersion:  kubeVersion,
					apiVersions:  apiVersions,
				},
			)
			require.NoError(t, err)
			if cleanup != nil {
				defer cleanup()
			}

			// Sanity: this row really exercises the strategy it claims to.
			require.Equal(t, tc.want, classifyStrategy(streamDir, branchFolder),
				"row must exercise its declared strategy")

			// REQ1 (#432): cluster version info present on every strategy.
			assert.Equal(t, kubeVersion, req.KubeVersion, "KubeVersion must be set (#432)")
			assert.Equal(t, apiVersions, req.ApiVersions, "ApiVersions must be set (#432)")

			// REQ2 (#416): permissive project so helm-build errors are not masked
			// as permission errors. assertDefaultProjectFields checks both
			// ProjectName=default and ProjectSourceRepos=["*"].
			assertDefaultProjectFields(t, req)
		})
	}
}

// REQ3: a transient Helm dependency build error must be surfaced with its original
// message rather than swallowed or masked (#416). The error originates inside
// the repo server while it builds chart dependencies, so it is only observable
// against a real repo server - the permissive ProjectSourceRepos asserted in
// REQ2 is the unit-level precondition that keeps the repo server from replacing it
// with a misleading permission error, but verifying the surfaced message is an
// integration concern. Recorded as a skipped placeholder so the matrix has a
// named entry for REQ3; real coverage belongs in the integration suite (branch-N).
func TestSourceMatrix_RequestInvariants_HelmDepBuildErrorSurfaced_Integration(t *testing.T) {
	t.Skip("REQ3 is integration-only (#416): a swallowed helm-dependency build error is only observable against a real repo server (branch-N)")
}
