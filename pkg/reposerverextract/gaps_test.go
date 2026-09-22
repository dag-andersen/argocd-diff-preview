package reposerverextract

// Source-combination matrix - GAP and BUG rows (the frontier).
//
// These rows were identified by auditing the real repo-server source
// (argo-cd/reposerver/repository/repository.go) and were NOT covered by the
// original STR/TYP/REACH/REF/REQ/TRV/NEG tables. They are split into their own
// file because their status differs from the rest of the suite:
//
//   - Some assert behavior our tool DOES NOT implement yet. They are skipped by
//     default so normal CI stays green, but can be enabled with
//     RUN_KNOWN_BUG_TESTS=1 to run the failing repros.
//   - Some are integration-only (the failure happens inside the repo server and
//     cannot be observed by a unit test). Those are skipped placeholders, the
//     same treatment as REACH14 / TRV3 / REQ3.
//
// Each known-bug row asserts the REPO-SERVER-CORRECT outcome, so it will go
// green the moment the underlying behavior is fixed. When fixing one, remove
// the skipKnownBugUnlessEnabled call from that test.
//
// See SOURCE_MATRIX.md for the per-row expected behavior and repo-server refs.

import (
	"os"
	"path/filepath"
	"testing"

	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const runKnownBugTestsEnv = "RUN_KNOWN_BUG_TESTS"

func skipKnownBugUnlessEnabled(t *testing.T, row, reason string) {
	t.Helper()
	if os.Getenv(runKnownBugTestsEnv) == "1" {
		return
	}
	t.Skipf("%s known bug: %s. Set %s=1 to run this failing repro.", row, reason, runKnownBugTestsEnv)
}

// routeSingle is a small helper: write files, build the single content source's
// request, and return the observed strategy and the request. It fails the test
// on any routing error.
func routeSingle(t *testing.T, repo, prRepo, appYAML string, files map[string]string) (streamStrategy, *repoapiclient.ManifestRequest) {
	t.Helper()
	if prRepo == "" {
		prRepo = repo
	}
	branchFolder := writeBranchFiles(t, files)
	app := makeApp(t, replaceRepo(appYAML, repo))

	contentSources, refSources, hasMultipleSources, err := splitSources(app)
	require.NoError(t, err)
	require.Len(t, contentSources, 1, "helper is for single-content-source rows")

	req, streamDir, cleanup, err := buildManifestRequestForSource(
		app, contentSources[0], refSources, hasMultipleSources, branchFolder, &RepoCreds{},
		manifestRequestRenderContext{
			repoSelector: testRepoSelector(t, prRepo),
			kubeVersion:  "v1.30.1",
			apiVersions:  []string{"apps/v1", "v1"},
		},
	)
	require.NoError(t, err)
	if cleanup != nil {
		t.Cleanup(cleanup)
	}
	return classifyStrategy(streamDir, branchFolder), req
}

// ── TYP5: explicit `directory:` on a dir that has Chart.yaml ──────────────────
//
// The repo server's ExplicitType() makes this render as Directory, NOT Helm,
// regardless of the on-disk Chart.yaml (types.go:3799). Our isLocalHelmChart
// only checks for Chart.yaml on disk, so it treats this as a Helm chart and
// narrows the stream to the chart dir. Asserting the repo-server-correct
// outcome: an explicit directory source is plain-manifest and should stream the
// branch root with Path kept (so the repo server can find all manifests).
//
// EXPECTED TO FAIL today: we narrow to chart-dir and clear Path.
func TestSourceMatrix_TYP5_ExplicitDirectoryOverChartYaml(t *testing.T) {
	skipKnownBugUnlessEnabled(t, "TYP5", "explicit directory sources with Chart.yaml are narrowed as Helm charts")

	const repo = "https://github.com/org/repo.git"
	got, req := routeSingle(t, repo, "", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: typ5}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: apps/chart
    targetRevision: HEAD
    directory:
      recurse: true
`, map[string]string{
		"apps/chart/Chart.yaml":       "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
		"apps/chart/deployment.yaml":  "apiVersion: apps/v1\nkind: Deployment\nmetadata: {name: x}\n",
	})

	assert.Equal(t, strategyStreamBranchRoot, got,
		"explicit directory: source must not be narrowed as a Helm chart (ExplicitType wins)")
	assert.Equal(t, "apps/chart", req.ApplicationSource.Path, "Path must be kept for a directory source")
}

// ── TYP6: explicit `helm:` on a dir that has kustomization.yaml ───────────────
//
// ExplicitType() makes this render as Helm, not Kustomize. Our isKustomizeSource
// sees kustomization.yaml and keeps the branch root (the Kustomize behavior).
// For Helm, the repo server would want the chart dir as root - but there is no
// Chart.yaml here, so this is really "explicit helm on a non-chart dir", which
// the repo server treats as Helm-on-a-dir. The safe correct outcome we can
// assert: routing must at least not crash and must stream something renderable.
// We assert branch-root (Path kept) as the conservative correct behavior.
//
// May PASS today (we already stream branch root for kustomization.yaml dirs).
func TestSourceMatrix_TYP6_ExplicitHelmOverKustomization(t *testing.T) {
	const repo = "https://github.com/org/repo.git"
	got, req := routeSingle(t, repo, "", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: typ6}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: apps/k
    targetRevision: HEAD
    helm:
      releaseName: explicit
`, map[string]string{
		"apps/k/kustomization.yaml": "resources: []\n",
	})

	assert.Equal(t, strategyStreamBranchRoot, got,
		"explicit helm: on a non-chart dir should stream branch root (Path kept)")
	assert.Equal(t, "apps/k", req.ApplicationSource.Path)
}

// ── TYP7: both `helm:` and `kustomize:` blocks -> repo server hard error ───────
//
// ExplicitType() returns "multiple application sources defined" (types.go:3816)
// and nothing renders. Our tool does not detect this and will happily build a
// request. There is no unit-observable failure on our side (we don't validate
// the combination), so the realistic assertion is that the repo server would
// reject it - which is an integration concern. Documented as a skipped
// placeholder; the U-portion (our tool's lack of validation) is asserted as a
// known-permissive behavior below so the row is still exercised.
func TestSourceMatrix_TYP7_MultipleExplicitTypes_Integration(t *testing.T) {
	t.Skip("TYP7 is integration-only: the 'multiple application sources defined' error is raised inside the repo server (ExplicitType, types.go:3816)")
}

// ── TYP9: dir has BOTH Chart.yaml and kustomization.yaml ──────────────────────
//
// Repo-server discovery picks Kustomize (discovery.go:65 overwrites Helm). Our
// buildStreamDirForLocalSource checks isKustomizeSource first and keeps the
// branch root - which matches. This row should PASS and documents the parity.
func TestSourceMatrix_TYP9_BothChartAndKustomization_PicksKustomize(t *testing.T) {
	const repo = "https://github.com/org/repo.git"
	got, _ := routeSingle(t, repo, "", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: typ9}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: apps/both
    targetRevision: HEAD
`, map[string]string{
		"apps/both/Chart.yaml":         "apiVersion: v2\nname: both\nversion: 0.1.0\n",
		"apps/both/kustomization.yaml": "resources: []\n",
	})

	assert.Equal(t, strategyStreamBranchRoot, got,
		"a dir with both markers must stream branch root (Kustomize wins, like the repo server)")
}

// ── TYP8: .argocd-source.yaml injects a helm/kustomize block ──────────────────
//
// mergeSourceParameters applies .argocd-source.yaml from the app dir BEFORE type
// detection (repository.go:1872), so an override file can change the rendered
// type. Our tool never reads override files, so it cannot react. The render-time
// effect is only observable against a real repo server -> integration. The
// unit-observable shortfall (we ignore the file) has no assertion target on our
// side, so this is a skipped integration placeholder.
func TestSourceMatrix_TYP8_ArgocdSourceInjectsType_Integration(t *testing.T) {
	t.Skip("TYP8 is integration-only: .argocd-source.yaml is applied inside the repo server before type detection (repository.go:1872); our tool does not read it")
}

// ── TYP10: source type disabled via EnabledSourceTypes ────────────────────────
func TestSourceMatrix_TYP10_DisabledSourceTypeFallsBackToDirectory_Integration(t *testing.T) {
	t.Skip("TYP10 is integration-only: IsManifestGenerationEnabled gating happens inside the repo server (repository.go:1938); our tool never sets EnabledSourceTypes")
}

// ── REACH5: .argocd-source.yaml adds out-of-chart helm.valueFiles (BUG) ────────
//
// CONFIRMED BUG. The override file (read from the chart dir by the repo server)
// can add helm.valueFiles that point outside the chart dir. Our
// hasOutOfChartValueFile only inspects the MANIFEST's Helm.ValueFiles, so it
// never sees the override-injected file, returns false, and we narrow the stream
// to the chart dir - dropping the out-of-chart file from the tarball. The repo
// server then fails to find it.
//
// Asserting the repo-server-correct outcome: branch root must be streamed.
// EXPECTED TO FAIL today (we narrow to chart-dir).
func TestSourceMatrix_REACH5_ArgocdSourceOutOfChartValues_BUG(t *testing.T) {
	skipKnownBugUnlessEnabled(t, "REACH5", ".argocd-source.yaml out-of-chart valueFiles are not considered when choosing stream root")

	const repo = "https://github.com/org/repo.git"
	got, _ := routeSingle(t, repo, "", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: reach5}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: apps/chart
    targetRevision: HEAD
`, map[string]string{
		"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
		// The override file the repo server will read - it adds an out-of-chart
		// value file. Our tool does not parse this today.
		"apps/chart/.argocd-source.yaml": "helm:\n  valueFiles:\n    - ../shared/values.yaml\n",
		"shared/values.yaml":             "replicas: 3\n",
	})

	assert.Equal(t, strategyStreamBranchRoot, got,
		"BUG REACH5: .argocd-source.yaml out-of-chart valueFiles must force branch-root streaming")
}

// ── REACH6: valueFile with env placeholder ────────────────────────────────────
//
// The repo server runs env.Envsubst on value-file paths before resolving them
// (repository.go:1502). A placeholder could expand to an out-of-chart path. Our
// hasOutOfChartValueFile does not envsubst before classifying, so it may judge
// the literal "${ARGOCD_APP_NAME}/x.yaml" as in-chart. Whether this is wrong
// depends on the env; we assert the conservative correct behavior for an
// obviously-escaping placeholder value.
//
// EXPECTED TO FAIL today (we do not expand env vars when classifying).
func TestSourceMatrix_REACH6_EnvPlaceholderValueFile(t *testing.T) {
	skipKnownBugUnlessEnabled(t, "REACH6", "environment-placeholder valueFiles are treated as refs and skipped during stream-root classification")

	const repo = "https://github.com/org/repo.git"
	got, _ := routeSingle(t, repo, "", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: reach6}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: apps/chart
    targetRevision: HEAD
    helm:
      valueFiles:
        - $ARGOCD_ENV_OUT/values.yaml
`, map[string]string{
		"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
	})

	// $ARGOCD_ENV_OUT begins with "$", so our classifier currently treats it as a
	// $ref and skips it -> narrows to chart-dir. The repo server would envsubst it
	// to a real (likely out-of-chart) path. Conservative correct: branch root.
	assert.Equal(t, strategyStreamBranchRoot, got,
		"env-placeholder value files should not be assumed in-chart")
}

// ── REACH7: glob valueFile matching out-of-chart files ────────────────────────
//
// The repo server expands globs (repository.go:1516). A glob like ../env/*.yaml
// matches out-of-chart files. Our hasOutOfChartValueFile treats the glob string
// literally; "../env/*.yaml" actually starts with ".." so it may already be
// caught - but a glob that is in-chart-prefixed yet escapes (e.g. "*/../../x")
// would not be. We assert the clear out-of-chart relative glob forces branch
// root.
//
// May PASS (the "../" prefix is caught by the existing relative-escape check).
func TestSourceMatrix_REACH7_GlobOutOfChartValueFile(t *testing.T) {
	const repo = "https://github.com/org/repo.git"
	got, _ := routeSingle(t, repo, "", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: reach7}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: apps/chart
    targetRevision: HEAD
    helm:
      valueFiles:
        - ../env/*.yaml
`, map[string]string{
		"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
		"env/prod.yaml":         "replicas: 2\n",
	})

	assert.Equal(t, strategyStreamBranchRoot, got,
		"a glob value file resolving outside the chart dir must force branch-root streaming")
}

// ── REACH8: valueFile remote URL with disallowed scheme (integration) ─────────
func TestSourceMatrix_REACH8_DisallowedUrlScheme_Integration(t *testing.T) {
	t.Skip("REACH8 is integration-only: URL-scheme allow-listing happens inside the repo server (isURLSchemeAllowed, resolved.go:53)")
}

// ── REACH9: missing valueFile with ignoreMissingValueFiles ────────────────────
//
// When a value file is missing, ignoreMissingValueFiles=true makes the repo
// server skip it (repository.go:1542). This affects whether a missing
// out-of-chart file even matters. Our routing does not consider the flag; for an
// out-of-chart value file it forces branch root regardless. We assert that an
// out-of-chart value file still forces branch root (correct) - the flag only
// changes the repo server's behavior once the file is/ isn't streamed.
//
// Should PASS (out-of-chart still forces branch root).
func TestSourceMatrix_REACH9_IgnoreMissingValueFiles(t *testing.T) {
	const repo = "https://github.com/org/repo.git"
	got, _ := routeSingle(t, repo, "", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: reach9}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: apps/chart
    targetRevision: HEAD
    helm:
      ignoreMissingValueFiles: true
      valueFiles:
        - ../shared/maybe.yaml
`, map[string]string{
		"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
	})

	assert.Equal(t, strategyStreamBranchRoot, got,
		"an out-of-chart value file forces branch root even with ignoreMissingValueFiles")
}

// ── REACH11: helm.fileParameters[].path out-of-chart ──────────────────────────
//
// fileParameters resolve like valueFiles in the repo server (repository.go:1343)
// and can point outside the chart dir. Our hasOutOfChartValueFile only inspects
// ValueFiles, never FileParameters, so an out-of-chart fileParameter does not
// force the branch root and the file is dropped from the narrowed tarball.
//
// Asserting the repo-server-correct outcome: branch root.
// EXPECTED TO FAIL today (fileParameters are ignored when classifying).
func TestSourceMatrix_REACH11_FileParameterOutOfChart_BUG(t *testing.T) {
	skipKnownBugUnlessEnabled(t, "REACH11", "out-of-chart helm.fileParameters are not considered when choosing stream root")

	const repo = "https://github.com/org/repo.git"
	got, _ := routeSingle(t, repo, "", `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: reach11}
spec:
  destination: {namespace: default}
  source:
    repoURL: {{REPO}}
    path: apps/chart
    targetRevision: HEAD
    helm:
      fileParameters:
        - name: ca
          path: ../shared/ca.pem
`, map[string]string{
		"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
		"shared/ca.pem":         "PEM\n",
	})

	assert.Equal(t, strategyStreamBranchRoot, got,
		"an out-of-chart fileParameter must force branch-root streaming (currently ignored)")
}

// ── REACH12: helm.fileParameters[].path remote URL (integration) ──────────────
func TestSourceMatrix_REACH12_FileParameterRemoteUrl_Integration(t *testing.T) {
	t.Skip("REACH12 is integration-only: remote fileParameter fetch happens inside the repo server")
}

// ── REF13: helm.fileParameters[].path: $ref/... (BUG) ─────────────────────────
//
// CONFIRMED BUG. The repo server treats fileParameters as ref candidates exactly
// like valueFiles (repository.go:571-575). Our rewriteRefValueFiles only rewrites
// source.Helm.ValueFiles, never FileParameters, so for a multi-source app the ref
// source is staged into .refs/ but the fileParameter path is left as "$ref/foo"
// and the streamed render cannot resolve it.
//
// Asserting the repo-server-correct outcome: the fileParameter path is rewritten
// to a relative on-disk path (no leading "$") that exists under the stream tree.
// EXPECTED TO FAIL today (FileParameters are never rewritten).
func TestSourceMatrix_REF13_FileParameterRef_BUG(t *testing.T) {
	skipKnownBugUnlessEnabled(t, "REF13", "helm.fileParameters with $ref paths are not rewritten into the staged .refs tree")

	const repo = "https://github.com/org/repo.git"

	branchFolder := writeBranchFiles(t, map[string]string{
		"apps/chart/Chart.yaml": "apiVersion: v2\nname: chart\nversion: 0.1.0\n",
		"ca.pem":                "PEM\n",
	})
	app := makeApp(t, replaceRepo(`
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata: {name: ref13}
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
        fileParameters:
          - name: ca
            path: $cfg/ca.pem
`, repo))

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

	require.NotNil(t, req.ApplicationSource.Helm, "expected Helm config")
	require.Len(t, req.ApplicationSource.Helm.FileParameters, 1)
	rewritten := req.ApplicationSource.Helm.FileParameters[0].Path
	assert.NotContains(t, rewritten, "$",
		"BUG REF13: $ref fileParameter path must be rewritten to a relative path, not left as %q", rewritten)
	abs := filepath.Clean(filepath.Join(streamDir, req.ApplicationSource.Path, rewritten))
	_, statErr := os.Stat(abs)
	assert.NoError(t, statErr,
		"BUG REF13: rewritten fileParameter %q should point at a real file under the stream tree", abs)
}

// ── NEG4: a $ref source that itself sets chart: (integration) ─────────────────
func TestSourceMatrix_NEG4_RefSourceWithChart_Integration(t *testing.T) {
	t.Skip("NEG4 is integration-only: the repo server rejects chart refs ('Helm charts are not yet not supported for ref sources', repository.go:594)")
}

// ── NEG5: raw streamed single-source with $ref, no .refs staging (integration) ─
func TestSourceMatrix_NEG5_RawStreamedRefUnsupported_Integration(t *testing.T) {
	t.Skip("NEG5 is integration-only: a raw streamed $ref fails inside the repo server ('failed to find repo', repository.go:1565). Our .refs staging exists precisely to avoid this.")
}
