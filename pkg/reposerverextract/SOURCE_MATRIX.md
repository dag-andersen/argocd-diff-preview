# Repo-server-api source-combination matrix (test coverage spec)

> Status: **PARTIAL - gaps found.** Tables A-E (35 rows) are implemented and
> backed by named tests. However, a subsequent audit of the **actual repo-server
> source** (`argo-cd/reposerver/repository/repository.go`) found several input
> dimensions the original axes missed - including **two confirmed latent bugs**
> in our own routing. Those are captured in tables **F-H** and the
> "Repo-server audit findings" section below; they are **not yet implemented**.
>
> Goal: enumerate every *valid* way an Argo CD Application can describe its
> `source` / `sources`, decide the expected behaviour of the repo-server-api
> render path for each, and mark which cases are **unit-testable** vs which
> **need a real repo server** (integration). The output is a checklist of
> combinations so we stop discovering broken-but-valid cases one GitHub issue
> at a time.
>
> Test-name convention: each implemented row maps to `TestSourceMatrix_Table<N>...`,
> keyed by the row id (e.g. A11 -> `TestSourceMatrix_TableA11_*`, the B-table rows
> are subtests named `B1`/`B2`/... inside `TestSourceMatrix_TableB_MultiSource_Routing`).

## Why this exists

7 of the last 10 issues were repo-server-api bugs (#416, #426, #432, #438,
#441, #446, #448). They are all the same shape: `buildManifestRequestForSource`
makes a **routing decision** across several independent dimensions, and we only
ever verified the exact points users happened to hit. This doc makes the whole
input space explicit.

## How the code routes today (the thing under test)

Entry point: `renderApp` -> `splitSources` -> `buildManifestRequestForSource`
(in `pkg/reposerverextract/extract.go`).

There are three possible outcomes ("strategies") for each **content source**:

| Strategy | How we call the repo server | When |
| --- | --- | --- |
| **R = Remote RPC** | `GenerateManifest` (unary). Repo server git-fetches everything itself. `RefSources` populated if there are refs. `streamDir == ""`. | Files are not available locally (remote chart, cross-repo source, external ref). |
| **S = Stream** | `GenerateManifestWithFiles` (streaming a tarball). `streamDir` = chart dir or branch root. | Same-repo local source, no refs (or refs handled by streaming). |
| **S+refs = Stream with .refs** | Stream a temp tree: content dir + `.refs/<ref>/`, with `$ref/...` valueFiles rewritten to relative paths. | Same-repo local `path:` source whose same-repo `$ref` sources are also local. |

The tests observe **four** distinct outcomes via the returned `streamDir`:
`remote` (`streamDir == ""`), `stream(chart-dir)` (narrowed, `Path` cleared),
`stream(branch-root)` (`streamDir == branchFolder`, `Path` kept), and
`stream(.refs)` (temp tree containing a `.refs/` dir). See `streamStrategy` /
`classifyStrategy` in `source_matrix_a_test.go`.

`splitSources` first splits the source list into:
- **content sources** - produce manifests (`path != "" || chart != "" || ref == ""`).
- **ref-only sources** - exist solely to provide `$ref` value files (`ref != ""`).
  (A source with BOTH `ref` and `path` counts as content AND ref - GH#401.)

## The dimensions of the input space

A single Application is the product of these axes. Not every combination is
valid; invalid ones are noted.

1. **Cardinality** - `source` (single) vs `sources` (multi). `ApplicationSet`
   reads from `spec.template.spec` but is otherwise identical.
2. **Primary source type** - what the content source is:
   - `path:` local **directory** of plain manifests / jsonnet (`directory:`)
   - `path:` local **Helm chart** (dir has `Chart.yaml`)
   - `path:` local **Kustomize** (dir has `kustomization.yaml`)
   - `path:` + **plugin** (CMP)
   - `chart:` **remote Helm chart** (from a Helm/OCI registry; no local files)
3. **Repo locality of the primary** - does `repoURL` match `--repo`/`--repo-regex`?
   - **local** (matches -> files are in the branch checkout)
   - **cross-repo** (does not match -> files are NOT local)
4. **Ref sources present?** - none / one-or-more `$ref` sources.
5. **Ref locality** - each ref's `repoURL`: local (same repo) vs external.
6. **valueFiles shape** (Helm only) - where the values live:
   - none
   - in-chart relative (`values.yaml`, `overrides/x.yaml`)
   - out-of-chart relative (`../shared/x.yaml`)
   - out-of-chart absolute (`/env/x.yaml`, resolved from repo root)
   - `$ref/...` (points into a ref source)
   - remote URL (`https://.../values.yaml`)
7. **Filesystem quirks in the streamed tree** - none / in-bounds symlink /
   out-of-bounds symlink / **directory** symlink.
8. **Revision redirection** - top-level app vs app-of-apps **child**, with /
   without `--redirect-target-revisions` (#446).

The following axes were **added after the repo-server audit** (they were missing
from the original list and are the source of the F-H tables):

9. **Explicit type vs on-disk type** - a source may set an explicit `helm:` /
   `kustomize:` / `directory:` / `plugin:` block. `ExplicitType()`
   (`types.go:3799`) **overrides** on-disk marker-file detection, and **two**
   explicit blocks is a hard error ("multiple application sources defined"). Our
   `isLocalHelmChart` / `isKustomizeSource` only look at on-disk files, so they
   can disagree with the repo server.
10. **`.argocd-source.yaml` override files** - `.argocd-source.yaml` and
    `.argocd-source-<appName>.yaml` are read **from the app/chart dir** and
    merge-patched onto the source **before** type detection
    (`mergeSourceParameters`, `repository.go:1872`). They can inject
    `helm.valueFiles` (incl. out-of-chart!), `helm.parameters`,
    `directory.recurse`, or even a whole tool block. Our tool does not read them
    at all.
11. **`helm.fileParameters`** - `fileParameters[].path` is a **first-class ref
    candidate** exactly like `valueFiles` (`repository.go:571-575`) and can be
    `$ref/...`, out-of-chart, or remote. Our `rewriteRefValueFiles` only rewrites
    `ValueFiles`.
12. **value-file resolution variants** (Helm) beyond axis 6: env-var
    placeholders (`env.Envsubst`), glob patterns (lexical order = merge order),
    disallowed URL scheme (hard error), `ignoreMissingValueFiles` true/false
    (zero-glob -> gRPC NotFound). Allowed schemes come from
    `HelmOptions.ValuesFileSchemes` (argocd-cm); nil = **no** remote allowed.
13. **Kustomize / Directory external-file pulls** - Kustomize `components:` may
    reference dirs **outside** the app dir and are **not** repoRoot-bounded
    (unlike value files); `--enable-helm` build option pulls remote charts;
    jsonnet `libs:` are **repoRoot-relative** and can import cross-app.

Axes 6-8 only bite once a case is already on a **Stream** strategy, which is
why they produced separate bugs. Axes 9-13 can bite on **any** strategy and are
largely unmodeled today.

## Legend

- Expected strategy: **R** / **S** / **S+refs** (see table above).
- Testability:
  - **U** = unit-testable today against `buildManifestRequestForSource`
    (assert chosen strategy + request fields + which files end up in the
    tarball). No repo server needed.
  - **I** = needs a real repo server (the failure happens *inside* it) ->
    integration test (branch-N mechanism).
- Status: ✅ covered by a matrix test (passing) · ⏭️ integration-only, present
  as a skipped placeholder (real coverage belongs in branch-N) · 🟥 known-broken
  (open issue) · ⬜ no test yet (gap) · ⚠️ partial.

---

## A. Single source (`spec.source`)

| # | Primary type | Repo | valueFiles | Expected | Test | Status | Notes / test |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A1 | local dir (manifests/jsonnet) | local | - | S (branch root, Path set) | U | ✅ | `TableA` row A1. Plain non-chart dir falls through to branch root. |
| A2 | local Helm chart | local | none | S (chart dir, Path cleared) | U | ✅ | `TableA` row A2 (also `..._SingleSource_LocalChart`). |
| A3 | local Helm chart | local | in-chart | S (chart dir) | U | ✅ | `TableA` row A3 (also `..._LocalChart_InChartValueFile_StreamsChartDir`). |
| A4 | local Helm chart | local | out-of-chart relative `../` | S (branch root) | U | ✅ | `TableA` row A4 (also `..._LocalChart_OutOfChartRelativeValueFile`). |
| A5 | local Helm chart | local | out-of-chart absolute `/` | S (branch root) | U | ✅ | `TableA` row A5 (the v0.2.10 fix, #444). |
| A6 | local Helm chart | local | remote URL | S (chart dir; URL ignored) | U | ✅ | `TableA` row A6 - asserts URL valueFiles do not force the branch root. |
| A7 | local Kustomize | local | - | S (branch root, Path set) | U | ✅ | `TableA` row A7 (also `..._SingleSource_Kustomize_StreamsBranchRoot`). |
| A8 | local Kustomize w/ `helmCharts` | local | - | S (branch root) | U | ✅ | `TableA` row A8 (also `..._SingleSource_KustomizeWithHelm_StreamsBranchRoot`). |
| A9 | local chart, in-bounds symlink in tree | local | any | S, render OK | U+I | ✅/⏭️ | `TableA9_InBoundsSymlink_RoutesAndExcludesUnrelated` asserts the tarball excludes unrelated symlinks (U). The repo server's symlink gate (#438) is the I half - integration. |
| A10 | local chart, **out-of-bounds** file symlink | local | any | render fails today; **#438** | I | ⏭️ | `TableA10_OutOfBoundsSymlink_Integration` (skipped placeholder). Repo-server rejects "out-of-bounds symlinks"; only observable in a real render. |
| A11 | local chart, **directory** symlink inside chart | local | any | render OK (**#448** fixed by #449) | U+I | ✅ | `TableA11_DirectorySymlinkInChart_RoutesSuccessfully` - routing no longer errors (EISDIR fixed). `copyDir` dir-symlink following also covered by `TestCopyDir_FollowsDirectorySymlink`. |
| A12 | local plugin (CMP) `path:` | local | - | S (branch root, Path set) | U | ✅ | `TableA` row A12. Plugin sources have no Chart.yaml/kustomization -> branch root. (branch-16 covers CMP end-to-end.) |
| A13 | remote Helm `chart:` | external | none | R (`streamDir==""`) | U | ✅ | `TableA` row A13 (also `..._SingleSource_ExternalChart_NoRefs`). |
| A14 | remote Helm `chart:` (OCI) | external | none | R | U | ✅ | `TableA` row A14 - `oci://` registry chart, same routing as A13. |
| A15 | local `path:` source | **cross-repo** | - | R (`streamDir==""`) | U | ✅ | `TableA` row A15. Path is **kept** (not cleared) - the repo server resolves it inside the fetched foreign repo. |

## B. Multi-source (`spec.sources`)

| # | Primary type | Primary repo | Ref(s) | valueFiles | Expected | Test | Status | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| B1 | local Helm chart | local | one local `$ref` | `$ref/...` | S+refs (rewrite paths) | U | ✅ | `TableB` subtest B1 (also `..._MultiSource_LocalChart_WithRef_RewritesValueFiles`). |
| B2 | local Helm chart | local | local `$ref` w/ both ref+path | `$ref/...` | S+refs | U | ✅ | `TableB` subtest B2. The ref+path source, routed as content, still streams `.refs` (it sees itself in the ref list). |
| B3 | local Helm chart | local | one **external** `$ref` | `$ref/...` | R + RefSources | U | ✅ | `TableB` subtest B3 (also `..._SameRepoPrimaryWithExternalRef_UsesRemoteRPC`, #426). |
| B4 | local `path:` (non-chart) | local | local `$ref` | n/a | S+refs | U | ✅ | `TableB` subtest B4 - plain manifests primary + local ref still synthesizes a `.refs` tree. |
| B5 | remote Helm `chart:` | external | one local `$ref` | `$ref/...` | S+refs via puller (or R+RefSources without one) | U+I | ✅ | **#441 fixed by #442.** `TableB` subtests B5 (with `fakeChartPuller` -> pulls chart, streams `.refs`, clears Chart) and B5b (no puller -> R + RefSources). Real registry pull is the I half. |
| B6 | remote Helm `chart:` | external | one external `$ref` | `$ref/...` | R + RefSources | U | ✅ | `TableB` subtest B6 (also `..._ExternalChart_WithRef_UsesRemoteRPC`). External ref forces R even with a puller present. |
| B7 | remote Helm `chart:` | external | local `$ref` w/ both ref+path | `$ref/...` | R + RefSources | U | ✅ | `TableB` subtest B7 (also `..._ApplicationSet_ExternalChart_WithRef`). |
| B8 | local Helm chart | local | local `$ref`, value file is out-of-chart abs `/` | mixed | S+refs (must also include out-of-chart file) | U | ✅ | `TableB` subtest B8 - the A5 (out-of-chart abs) + refs interaction; `$ref` rewritten, out-of-chart abs file reachable. |
| B9 | two local content sources (no ref) | local | - | per-source | S each (one request per content source) | U | ✅ | `TableB` subtest B9 (also `..._MultipleContentSources_BuildsOneRequestEach`). |
| B10 | local primary + external primary (mixed content) | mixed | - | - | per-source: S for local, R for external | U | ✅ | `TableB` subtest B10 - each content source routed independently. |
| B11 | ref-only source with **no path** (points at repo root) | local | local `$ref` (path="") | `$ref/x` | S+refs (ref dir = branch root) | U | ✅ | `TableB` subtest B11 - direct assertion that `$ref` with no path resolves against the repo root. |

## C. Cardinality / kind variants

| # | Case | Expected | Test | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| C1 | `ApplicationSet` (sources under `spec.template.spec`) | same routing as the equivalent Application | U | ✅ | `TableC1_ApplicationSet_RoutesLikeApplication` - builds the same logical app as both kinds and asserts identical strategy + RefSources keys. |
| C2 | app with neither `source` nor `sources` | error | U | ✅ | `TableC_DegenerateTopologies_Error` subtest C2 (also `..._NoSource_ReturnsError`). |
| C3 | all sources are ref-only (no content) | error | U | ✅ | `TableC_DegenerateTopologies_Error` subtests C3 (all ref-only) and C3b (empty `sources: []`) - direct assertions. |

## D. App-of-apps / traversal (orthogonal to the source matrix)

| # | Case | Expected | Test | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| D1 | child app source = local | child rendered like a normal app | U | ✅ | `TableD1_ChildApplication_RoutesLikeSeed` - a discovered child routes through `buildManifestRequestForSource` like a seed (local chart -> S chart dir, remote chart -> R). |
| D2 | child app, revision **not** in `--redirect-target-revisions` | child left on its pinned revision | U | ✅ | **#446 fixed by #447.** `TableD2_ChildHonorsRedirectTargetRevisions` (in-list -> redirected, out-of-list -> untouched, empty -> redirect all). Also `appofapps_test.go` `BuildChildApplication_*`. |
| D3 | child app source pinned to a ref that only exists remotely | render from remote, not redirected | I | ⏭️ | `TableD3_ChildRemoteOnlyRef_Integration` (skipped placeholder). Consequence of D2; the remote-only fetch is only observable in a real render. |

## E. Cross-cutting request-content correctness (not routing, but repo-server-api specific)

| # | Case | Expected | Test | Status | Notes |
| --- | --- | --- | --- | --- | --- |
| E1 | `ManifestRequest.ApiVersions` / `KubeVersion` populated | both set from the cluster | U | ✅ | **#432 (fixed v0.2.9).** `TableE_RequestInvariants_AllStrategies` asserts both on all three strategies (R, S chart-dir, S+refs); every A/B row asserts them too. |
| E2 | `ProjectName=default`, `ProjectSourceRepos=["*"]` on every request | permissive, so helm-build errors aren't masked as permission errors | U | ✅ | **#416.** `TableE_RequestInvariants_AllStrategies` asserts `assertDefaultProjectFields` on R / S / S+refs; every A/B row asserts it too. |
| E3 | transient helm-dep build error surfaced (not swallowed) | original error preserved | I | ⏭️ | `TableE3_HelmDepBuildErrorSurfaced_Integration` (skipped placeholder). **#416** - the surfaced message is only observable against a real repo server (E2 is the unit-level precondition). |

---

## Repo-server audit findings (NEW - not yet implemented)

The tables above were derived from *our* model of the repo server. Reading the
actual repo-server source (`argo-cd/reposerver/repository/repository.go`)
surfaced input dimensions the original axes missed. Tables **F-H** below
enumerate them. **Two rows (F4, H1) are confirmed latent bugs in our tool**, not
just coverage gaps.

> ⚠️ None of the F-H rows have tests yet. They are the real output of "does the
> matrix cover all cases?" - the answer was **no**.

### F. Source-type detection / overrides

| # | Case | Expected | Test | Status | Notes (repo-server ref) |
| --- | --- | --- | --- | --- | --- |
| F1 | dir has `Chart.yaml` but manifest sets `directory:` explicitly | render as **Directory**, not Helm | U | ⬜ | `ExplicitType()` wins over on-disk (`types.go:3799`). Our `isLocalHelmChart` ignores the explicit block -> may narrow to chart dir wrongly. |
| F2 | dir has `kustomization.yaml` but manifest sets `helm:` explicitly | render as **Helm** | U | ⬜ | Same precedence. Our `isKustomizeSource` would keep branch root; repo server renders Helm. |
| F3 | manifest sets **both** `helm:` and `kustomize:` | **hard error** "multiple application sources defined" | U/I | ⬜ | `ExplicitType()` errors (`types.go:3816`). Our tool streams something instead of failing. Negative case. |
| F4 | **`.argocd-source.yaml` in chart dir adds out-of-chart `helm.valueFiles`** | must stream **branch root** (file is outside chart) | U+I | 🟥 | **CONFIRMED BUG.** `mergeSourceParameters` (`repository.go:1872`) injects valueFiles our `hasOutOfChartValueFile` never sees (it reads only the manifest). We narrow to chart dir -> repo server render fails (missing file). Same class as #444. |
| F5 | `.argocd-source-<appName>.yaml` injects a `helm:`/`kustomize:` block | type changes accordingly | U+I | ⬜ | Override applied **before** type detection. Our tool ignores override files entirely. |
| F6 | dir has **both** `Chart.yaml` and `kustomization.yaml` | repo-server discovery picks **Kustomize** | U | ⬜ | `discovery.go:65` overwrites Helm with Kustomize. Our `buildStreamDirForLocalSource` checks Kustomize first too (branch root) - verify parity. |
| F7 | source type disabled via `EnabledSourceTypes` | falls back to **Directory** | I | ⬜ | `IsManifestGenerationEnabled` (`repository.go:1938`). We never set EnabledSourceTypes; document behavior. |

### G. Helm value-file resolution variants (extends axis 6)

| # | Case | Expected | Test | Status | Notes (repo-server ref) |
| --- | --- | --- | --- | --- | --- |
| G1 | valueFile with env placeholder `$ENV/x.yaml` (via `helm.valuesObject`/env) | resolved after `env.Envsubst` | U+I | ⬜ | `repository.go:1502`. Our `hasOutOfChartValueFile` does not envsubst before classifying -> could misjudge chart-dir vs branch-root. |
| G2 | valueFile **glob** `env/*.yaml` | expanded lexically; order = merge precedence | U+I | ⬜ | `isGlobPath` + `doublestar` (`repository.go:1516`). A glob can match out-of-chart files -> our narrowing logic ignores globs. |
| G3 | valueFile remote URL with **disallowed** scheme | **hard error** | I | ⬜ | `isURLSchemeAllowed` (`resolved.go:53`). Schemes from `HelmOptions.ValuesFileSchemes`; nil = none allowed. |
| G4 | missing valueFile, `ignoreMissingValueFiles: true` vs `false` | skip vs fail | U+I | ⬜ | `repository.go:1542`. Affects whether a missing out-of-chart file even matters. |
| G5 | valueFile leading-slash `/env/x.yaml` (repo-root relative) | resolved from repo root | U | ⚠️ | Covered by A5/B8 for the *routing* outcome, but not the resolution semantics directly. |

### H. Helm `fileParameters` (parallel to valueFiles - largely unmodeled)

| # | Case | Expected | Test | Status | Notes (repo-server ref) |
| --- | --- | --- | --- | --- | --- |
| H1 | multi-source, **`helm.fileParameters[].path: $ref/foo`** | path rewritten like a valueFile; ref staged | U+I | 🟥 | **CONFIRMED BUG.** Repo server treats fileParameters as ref candidates (`repository.go:571-575`); our `rewriteRefValueFiles` only rewrites `ValueFiles`, never `FileParameters` -> ref staged but path unrewritten -> render fails. Same class as #441/#426. |
| H2 | `fileParameters[].path` out-of-chart relative `../x` | branch root must be streamed | U+I | ⬜ | fileParameters resolve like valueFiles (`repository.go:1343`). Our `hasOutOfChartValueFile` ignores fileParameters -> may narrow wrongly. |
| H3 | `fileParameters[].path` remote URL | fetched by repo server | I | ⬜ | Same scheme rules as valueFiles. |

### I. Kustomize / Directory external pulls (extends axis 13)

| # | Case | Expected | Test | Status | Notes (repo-server ref) |
| --- | --- | --- | --- | --- | --- |
| I1 | Kustomize `components:` referencing `../other-dir` (outside app) | branch root must be streamed | U+I | ⬜ | components are **not** repoRoot-bounded (`kustomize.go:337`). We always stream branch root for Kustomize, so this likely works - but unverified. |
| I2 | Kustomize `--enable-helm` build option + `helmCharts:` | pulls remote chart during build | I | ⬜ | `isHelmEnabled` (`kustomize.go:427`). Needs build options + network. |
| I3 | Directory jsonnet `libs:` referencing another repo dir | imports resolved repoRoot-relative | U+I | ⬜ | `repository.go:2263`. Cross-app import; branch root must be streamed. |

### Negative / invalid combinations (document, don't "fix")

| # | Case | Expected | Notes |
| --- | --- | --- | --- |
| N1 | a `$ref` source that itself sets `chart:` | repo server **rejects** | "Helm charts are not yet not supported for 'ref' sources" (`repository.go:594`). |
| N2 | raw streamed single-source with `$ref` value file, no `.refs` staging | fails "failed to find repo" | Streaming path does not populate `gitRepoPaths` (`repository.go:1565`). Validates *why* our `.refs` staging + rewrite mechanism exists - and why it must cover fileParameters too (H1). |

---

## What we found (honest summary)

Tables A-E are **complete and 1:1 with their rows**: 35 rows, each with a named
test. 9 passing test functions cover the unit rows; 3 skipped placeholders mark
the integration-only rows (A10, D3, E3).

**Bugs found by the unit matrix alone (A-E): 0.** Every unit row passed against
`main`, or was an already-tracked issue.

**Bugs found by the repo-server audit (F-H): 2 confirmed, plus several
unmodeled dimensions.** This is the real payoff and it only appeared once we
stopped trusting our own model and read the repo-server source:

- **F4** - `.argocd-source.yaml` can add out-of-chart `helm.valueFiles` that our
  `hasOutOfChartValueFile` never sees (it inspects only the manifest), so we
  narrow the stream to the chart dir and the render fails on the missing file.
  Same class as #444.
- **H1** - `helm.fileParameters[].path: $ref/...` is a first-class ref candidate
  in the repo server, but our `rewriteRefValueFiles` only rewrites `ValueFiles`,
  so the ref is staged into `.refs/` while the fileParameter path is left
  unrewritten and unresolvable. Same class as #441/#426.

Both are **latent** (no open issue yet) - exactly the "broken-but-valid case we
discover one GitHub issue at a time" this exercise was meant to pre-empt.

What the A-E exercise bought us regardless:

1. **Regression net for recently-merged fixes.** Several fixes had no dedicated
   regression test before; the matrix now pins them so they cannot silently
   regress: #444 (A5 out-of-chart abs valueFiles), #442 (B5 remote chart +
   same-repo ref), #449 (A11 directory symlink), #447 (D2/D3 children honor
   `--redirect-target-revisions`).
2. **Closed real coverage gaps** the draft flagged as ⬜/⚠️ (untested-but-valid,
   "where the next bug hides"): A1, A6, A12, A14, B4, B8, B10, B11, C1, and the
   E2 invariant across **all three** strategies (the #416 root cause was that it
   was only asserted on the streamed local-chart path).
3. **Flushed out four wrong assumptions** (caught as failing tests, then fixed):
   - A15: cross-repo source keeps `Path` (the repo server resolves it in the
     fetched foreign repo); it is **not** cleared.
   - B2: a `ref`+`path` source, routed as content, still streams `.refs` (it
     sees itself in the ref list) rather than degrading to a chart-dir stream.
   - B5/B8: a `$ref` source with no `path` points at the **repo root**, so
     `$cfg/values.yaml` resolves to `<root>/values.yaml`.
   - A11: on the single-source path the chart streams via the tarball compressor,
     which preserves a directory symlink **as a symlink entry**; the
     `copyDir`-follows-dir-symlink behavior (#449) is on the refs/slow path.

## Where the next real bug most likely hides (integration)

Unit tests structurally cannot see failures that happen **inside** the repo
server. The skipped placeholders are precisely those cases, and they are the
highest-value targets for proactive bug-hunting via branch-N integration:

- **A10 (#438)** - out-of-bounds file symlink rejected by the repo server's
  symlink safety gate. Needs an out-of-bounds symlink fixture.
- **B5 (#441/#442)** - the *real* registry pull + same-repo ref render (the unit
  test stubs the puller).
- **D3 (#446)** - a child pinned to a remote-only ref rendered by a real server.
- **E3 (#416)** - a transient helm-dependency build error surfaced with its
  original message rather than masked.

## Test file layout

- `source_matrix_a_test.go` - Table A (single source) + the A9/A10/A11 symlink
  rows. Hosts the shared helpers (`streamStrategy`, `classifyStrategy`,
  `writeBranchFiles`, `replaceRepo`).
- `source_matrix_b_test.go` - Table B (multi-source), one big table-driven test
  with per-row subtests B1-B11 (+ B5b).
- `source_matrix_cde_test.go` - Tables C, D, E.
