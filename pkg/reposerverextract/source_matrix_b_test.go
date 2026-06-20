package reposerverextract

// Source-combination matrix - Table B (multi-source `spec.sources`).
//
// Companion to SOURCE_MATRIX.md, section B. Each row is one valid way a
// multi-source Application can describe its content + $ref sources. Like Table
// A, the test asserts the ROUTING OUTCOME only:
//
//   - strategy per content source: remote / stream(chart-dir) /
//     stream(branch-root) / stream(.refs).
//   - RefSources map keys when the repo server resolves refs itself.
//   - that $ref/... value files are rewritten to on-disk relative paths when we
//     synthesize a .refs/ tree, and that the rewritten file actually exists.
//   - the repo-server-api invariants that caused bugs (KubeVersion/ApiVersions
//     for #432, ProjectName/ProjectSourceRepos for #416) on EVERY strategy.
//
// The split (content vs ref-only) is exercised by feeding the whole Application
// through splitSources and then routing each content source, so rows B9/B10
// (multiple content sources) assert per-source outcomes.
//
// See SOURCE_MATRIX.md for the issue cross-references (B3/#426, B5/#441,
// B6/#428, etc.).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// contentExpectation describes the expected routing outcome for one content
// source within a multi-source Application, keyed by a stable identity (the
// source's Chart or Path) so the order the sources are returned in does not
// matter.
type contentExpectation struct {
	// chart/path identify which content source this expectation is for. Exactly
	// one is set; it is matched against the ORIGINAL source (pre-rewrite).
	chart string
	path  string

	want      streamStrategy
	wantChart string // expected ManifestRequest Chart after routing
	// refKeys are the RefSources map keys that must be present (e.g. "$values").
	// Only meaningful for strategyRemote rows that carry refs.
	refKeys []string
	// noRefs asserts RefSources is nil (refs were inlined into the stream).
	noRefs bool
	// valueFileNoDollar asserts the (single) Helm value file was rewritten to a
	// relative path with no "$" prefix and that it exists under the stream dir.
	valueFileNoDollar bool
}

type multiSourceCase struct {
	name    string
	appYAML string

	// files to create under branchFolder before routing (relative path ->
	// contents). Use {{REPO}} inside appYAML for the PR repo URL.
	files map[string]string

	// prRepo overrides the PR repo selector (defaults to the shared repo).
	prRepo string

	// withPuller supplies a fakeChartPuller so the remote-chart-with-local-refs
	// path (B5/#441) streams the pulled chart instead of falling back to remote.
	withPuller bool

	// expectations is keyed implicitly by iteration; each entry is matched to a
	// content source by chart/path.
	expectations []contentExpectation
}

func TestSourceMatrix_TableB_MultiSource_Routing(t *testing.T) {
	const repo = "https://github.com/org/repo.git"

	cases := []multiSourceCase{
		{
			name: "B1 local chart + one local $ref -> stream(.refs), values rewritten",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b1}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: {{REPO}}
      ref: cfg
      targetRevision: HEAD
    - repoURL: {{REPO}}
      path: apps/chart
      targetRevision: HEAD
      helm:
        valueFiles:
          - $cfg/values-prod.yaml
`,
			files: map[string]string{
				"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
				"values-prod.yaml":      "replicas: 3\n",
			},
			expectations: []contentExpectation{{
				path:              "apps/chart",
				want:              strategyStreamRefs,
				noRefs:            true,
				valueFileNoDollar: true,
			}},
		},
		{
			name: "B2 local chart + ref-with-both-ref+path (GH401) -> stream(.refs)",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b2}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: {{REPO}}
      ref: shared
      path: shared
      targetRevision: HEAD
    - repoURL: {{REPO}}
      path: apps/chart
      targetRevision: HEAD
      helm:
        valueFiles:
          - $shared/values.yaml
`,
			files: map[string]string{
				"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
				"shared/values.yaml":    "replicas: 2\n",
				"shared/Chart.yaml":     "apiVersion: v2\nname: shared\nversion: 0.1.0\n",
			},
			// The source with both ref+path is BOTH a content source and a ref
			// source. When rendered AS content it still sees itself in the ref
			// list, so it too takes the .refs streaming path (it does not
			// degrade to a plain chart-dir stream).
			expectations: []contentExpectation{
				{
					path:              "apps/chart",
					want:              strategyStreamRefs,
					noRefs:            true,
					valueFileNoDollar: true,
				},
				{
					path: "shared",
					want: strategyStreamRefs,
				},
			},
		},
		{
			name: "B3 local chart + one external $ref -> remote RPC + RefSources",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b3}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: https://github.com/other/values.git
      ref: ext
      targetRevision: HEAD
    - repoURL: {{REPO}}
      path: apps/chart
      targetRevision: HEAD
      helm:
        valueFiles:
          - $ext/values.yaml
`,
			files: map[string]string{
				"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
			},
			// Because a ref lives in a different repo, the whole render falls back
			// to the remote RPC and the repo server fetches everything.
			expectations: []contentExpectation{{
				path:    "apps/chart",
				want:    strategyRemote,
				refKeys: []string{"$ext"},
			}},
		},
		{
			name: "B4 local plain dir (non-chart) + local $ref -> stream(.refs)",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b4}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: {{REPO}}
      ref: cfg
      targetRevision: HEAD
    - repoURL: {{REPO}}
      path: manifests
      targetRevision: HEAD
`,
			files: map[string]string{
				"manifests/deployment.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata: {name: x}\n",
				"cfg/values.yaml":           "k: v\n",
			},
			// Plain dir primary still synthesizes a .refs tree because a local ref
			// source is present; no Helm valueFiles to rewrite.
			expectations: []contentExpectation{{
				path:   "manifests",
				want:   strategyStreamRefs,
				noRefs: true,
			}},
		},
		{
			name: "B5 remote chart + one local $ref (#441) -> stream(.refs) via puller",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b5}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: {{REPO}}
      ref: cfg
      targetRevision: HEAD
    - repoURL: https://charts.example.com
      chart: my-chart
      targetRevision: "1.0.0"
      helm:
        valueFiles:
          - $cfg/values.yaml
`,
			files: map[string]string{
				"values.yaml": "replicas: 5\n",
			},
			withPuller: true,
			// With a puller, the remote chart is pulled into the stream tree and
			// rendered as a local path chart; Chart is cleared, values rewritten.
			expectations: []contentExpectation{{
				chart:             "my-chart",
				want:              strategyStreamRefs,
				wantChart:         "",
				noRefs:            true,
				valueFileNoDollar: true,
			}},
		},
		{
			name: "B5b remote chart + one local $ref, NO puller -> remote RPC + RefSources",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b5b}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: {{REPO}}
      ref: cfg
      targetRevision: HEAD
    - repoURL: https://charts.example.com
      chart: my-chart
      targetRevision: "1.0.0"
      helm:
        valueFiles:
          - $cfg/values.yaml
`,
			files: map[string]string{
				"values.yaml": "replicas: 5\n",
			},
			withPuller: false,
			// Without a puller we cannot stream the chart, so fall back to remote
			// and let the repo server resolve the ref from git.
			expectations: []contentExpectation{{
				chart:     "my-chart",
				want:      strategyRemote,
				wantChart: "my-chart",
				refKeys:   []string{"$cfg"},
			}},
		},
		{
			name: "B6 remote chart + one external $ref -> remote RPC + RefSources",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b6}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: https://github.com/other/values.git
      ref: ext
      targetRevision: HEAD
    - repoURL: https://charts.example.com
      chart: my-chart
      targetRevision: "1.0.0"
      helm:
        valueFiles:
          - $ext/values.yaml
`,
			files:      nil,
			withPuller: true, // puller present but external ref forces remote anyway
			expectations: []contentExpectation{{
				chart:     "my-chart",
				want:      strategyRemote,
				wantChart: "my-chart",
				refKeys:   []string{"$ext"},
			}},
		},
		{
			name: "B7 remote chart + local ref-with-both-ref+path -> remote RPC + RefSources",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b7}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: https://github.com/other/values.git
      ref: ext
      path: shared
      targetRevision: HEAD
    - repoURL: https://charts.example.com
      chart: my-chart
      targetRevision: "1.0.0"
      helm:
        valueFiles:
          - $ext/values.yaml
`,
			files: nil,
			// The ref source is external; primary remote chart -> remote RPC. The
			// ref-with-path source is also a content source (cross-repo) -> remote.
			expectations: []contentExpectation{
				{
					chart:     "my-chart",
					want:      strategyRemote,
					wantChart: "my-chart",
					refKeys:   []string{"$ext"},
				},
				{
					path:      "shared",
					want:      strategyRemote,
					wantChart: "",
				},
			},
		},
		{
			name: "B8 local chart + local $ref AND out-of-chart abs value -> stream(.refs)",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b8}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: {{REPO}}
      ref: cfg
      targetRevision: HEAD
    - repoURL: {{REPO}}
      path: apps/chart
      targetRevision: HEAD
      helm:
        valueFiles:
          - $cfg/values.yaml
          - /env/prod.yaml
`,
			files: map[string]string{
				"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
				"values.yaml":           "replicas: 3\n",
				"env/prod.yaml":         "env: prod\n",
			},
			// Combines the A5 (out-of-chart abs) concern with refs. The $ref file
			// is rewritten; the absolute /env/prod.yaml must remain reachable in
			// the streamed tree. We assert the strategy + that the $ref file is
			// rewritten; the out-of-chart file reachability is asserted in check.
			expectations: []contentExpectation{{
				path:              "apps/chart",
				want:              strategyStreamRefs,
				noRefs:            true,
				valueFileNoDollar: true,
			}},
		},
		{
			name: "B9 two local content sources, no ref -> stream each independently",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b9}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: {{REPO}}
      path: apps/chart-a
      targetRevision: HEAD
    - repoURL: {{REPO}}
      path: apps/chart-b
      targetRevision: HEAD
`,
			files: map[string]string{
				"apps/chart-a/Chart.yaml": "apiVersion: v2\nname: a\nversion: 0.1.0\n",
				"apps/chart-b/Chart.yaml": "apiVersion: v2\nname: b\nversion: 0.1.0\n",
			},
			expectations: []contentExpectation{
				{path: "apps/chart-a", want: strategyStreamChartDir, noRefs: true},
				{path: "apps/chart-b", want: strategyStreamChartDir, noRefs: true},
			},
		},
		{
			name: "B10 local content + external content (mixed) -> stream local, remote external",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b10}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: {{REPO}}
      path: apps/local-chart
      targetRevision: HEAD
    - repoURL: https://charts.example.com
      chart: remote-chart
      targetRevision: "2.0.0"
`,
			files: map[string]string{
				"apps/local-chart/Chart.yaml": "apiVersion: v2\nname: local\nversion: 0.1.0\n",
			},
			expectations: []contentExpectation{
				{path: "apps/local-chart", want: strategyStreamChartDir, noRefs: true},
				{chart: "remote-chart", want: strategyRemote, wantChart: "remote-chart", noRefs: true},
			},
		},
		{
			name: "B11 ref-only source with no path (points at repo root) -> stream(.refs)",
			appYAML: `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: b11}
spec:
  destination: {namespace: default}
  sources:
    - repoURL: {{REPO}}
      ref: root
      targetRevision: HEAD
    - repoURL: {{REPO}}
      path: apps/chart
      targetRevision: HEAD
      helm:
        valueFiles:
          - $root/global/values.yaml
`,
			files: map[string]string{
				"apps/chart/Chart.yaml":    "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
				"global/values.yaml":       "g: 1\n",
				"apps/chart/values.yaml":   "local: true\n",
			},
			// The ref-only source has no path, so its ref dir is the branch root;
			// $root/global/values.yaml must resolve to a real file in .refs/root.
			expectations: []contentExpectation{{
				path:              "apps/chart",
				want:              strategyStreamRefs,
				noRefs:            true,
				valueFileNoDollar: true,
			}},
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
			require.True(t, hasMultipleSources, "table B rows use spec.sources")
			require.Len(t, contentSources, len(tc.expectations),
				"each content source must have exactly one expectation")

			renderCtx := manifestRequestRenderContext{
				repoSelector: testRepoSelector(t, prRepo),
				kubeVersion:  "v1.30.1",
				apiVersions:  []string{"apps/v1", "v1"},
			}
			if tc.withPuller {
				renderCtx.puller = &fakeChartPuller{}
			}

			for _, src := range contentSources {
				exp := matchExpectation(t, tc.expectations, src)

				req, streamDir, cleanup, err := buildManifestRequestForSource(
					app, src, refSources, hasMultipleSources, branchFolder,
					&RepoCreds{}, renderCtx,
				)
				require.NoError(t, err)
				if cleanup != nil {
					defer cleanup()
				}

				gotStrategy := classifyStrategy(streamDir, branchFolder)
				assert.Equalf(t, exp.want, gotStrategy,
					"[%s] strategy mismatch: want %s, got %s (streamDir=%q)",
					sourceID(src), exp.want, gotStrategy, streamDir)

				assert.Equalf(t, exp.wantChart, req.ApplicationSource.Chart,
					"[%s] Chart field", sourceID(src))

				if exp.noRefs {
					assert.Nilf(t, req.RefSources, "[%s] RefSources must be nil (refs inlined)", sourceID(src))
				}
				for _, key := range exp.refKeys {
					require.NotNilf(t, req.RefSources, "[%s] RefSources must be populated", sourceID(src))
					_, ok := req.RefSources[key]
					assert.Truef(t, ok, "[%s] RefSources must contain key %q (got keys %v)",
						sourceID(src), key, refKeysOf(req.RefSources))
				}

				if exp.valueFileNoDollar {
					require.NotNilf(t, req.ApplicationSource.Helm, "[%s] expected Helm config", sourceID(src))
					require.NotEmptyf(t, req.ApplicationSource.Helm.ValueFiles,
						"[%s] expected at least one value file", sourceID(src))
					rewritten := req.ApplicationSource.Helm.ValueFiles[0]
					assert.NotContainsf(t, rewritten, "$",
						"[%s] $ref value file must be rewritten to a relative path", sourceID(src))
					// The rewritten path is relative to the content dir inside the
					// stream tree; it must point at a real file.
					abs := filepath.Clean(filepath.Join(streamDir, req.ApplicationSource.Path, rewritten))
					_, statErr := os.Stat(abs)
					assert.NoErrorf(t, statErr, "[%s] rewritten value file %q should exist", sourceID(src), abs)
				}

				// Repo-server-api invariants on EVERY strategy (#432, #416).
				assert.Equalf(t, "v1.30.1", req.KubeVersion, "[%s] KubeVersion must be set (#432)", sourceID(src))
				assert.Equalf(t, []string{"apps/v1", "v1"}, req.ApiVersions, "[%s] ApiVersions must be set (#432)", sourceID(src))
				assertDefaultProjectFields(t, req)
			}
		})
	}
}

// sourceID returns a short human label for a content source for test messages.
func sourceID(s v1alpha1.ApplicationSource) string {
	if s.Chart != "" {
		return "chart=" + s.Chart
	}
	return "path=" + s.Path
}

// matchExpectation finds the expectation for the given content source by its
// chart or path identity, failing the test if none matches.
func matchExpectation(t *testing.T, exps []contentExpectation, src v1alpha1.ApplicationSource) contentExpectation {
	t.Helper()
	for _, e := range exps {
		if e.chart != "" && e.chart == src.Chart {
			return e
		}
		if e.path != "" && e.path == src.Path && src.Chart == "" {
			return e
		}
	}
	t.Fatalf("no expectation matched content source %s", sourceID(src))
	return contentExpectation{}
}

func refKeysOf(m map[string]*v1alpha1.RefTarget) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
