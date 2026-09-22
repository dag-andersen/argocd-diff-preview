# Repo-server-api source-combination matrix (test coverage spec)

> Status: **All rows implemented; known-bug repros skipped by default.** Every
> row in this matrix now has a named test. Most pass; five (TYP5, REACH5,
> REACH6, REACH11, REF13) are 🟥 SKIP - real tests asserting repo-server-correct
> behavior our tool does not yet produce. They are skipped in normal test runs
> so CI stays green, and can be enabled with `RUN_KNOWN_BUG_TESTS=1`.
> Integration-only rows are skipped placeholders.
>
> This matrix enumerates the inputs to the one function under test -
> `buildManifestRequestForSource` in `pkg/reposerverextract/extract.go` - and the
> repo-server behaviour each must satisfy. It is organised by **what the code
> decides**, not by the YAML shape of the Application.
>
> Rows whose status is ✅ have a named passing test; 🟥 SKIP rows are confirmed
> latent bugs with opt-in failing repros; ⏭️ rows are integration-only skipped
> placeholders; ⬜ rows are gaps with no test yet.
>
> Goal: stop discovering broken-but-valid cases one GitHub issue at a time by
> making the whole input space explicit and backing each cell with a test.

## Why this exists

7 of the last 10 issues were repo-server-api bugs (#416, #426, #432, #438,
#441, #446, #448). They are all the same shape: `buildManifestRequestForSource`
makes a **routing decision** across several independent dimensions, and we only
ever verified the exact points users happened to hit.

The original version of this doc was organised by Application *shape*
(single-source `A`, multi-source `B`, ...). That grouping conflated three
different axes and scattered the same logical decision (e.g. "must we stream the
branch root because a value file escapes the chart dir?") across three letters.
After auditing the real repo-server source, the doc was **re-organised around the
decision the code makes**. See "Mapping from the old A-H scheme" at the bottom.

## How the code routes (the thing under test)

Entry point: `renderApp` -> `splitSources` -> `buildManifestRequestForSource`.

`splitSources` first splits the source list into:
- **content sources** - produce manifests (`path != "" || chart != "" || ref == ""`).
- **ref-only sources** - exist solely to provide `$ref` value files (`ref != ""`).
  (A source with BOTH `ref` and `path` counts as content AND ref - GH#401.)

For each content source, the function chooses one of four observable outcomes,
exposed via the returned `streamDir` and asserted by `classifyStrategy` /
`streamStrategy` in `strategy_test.go`:

| Strategy | `streamDir` | How we call the repo server | When |
| --- | --- | --- | --- |
| **remote** | `""` | `GenerateManifest` (unary). Repo server git-fetches everything itself. `RefSources` populated if there are refs. | Files not available locally (remote chart, cross-repo source, external ref). |
| **stream(chart-dir)** | chart dir | `GenerateManifestWithFiles` (tarball), narrowed to the chart dir; `Path` cleared. | Same-repo local Helm chart, all files inside the chart dir. |
| **stream(branch-root)** | branch folder | `GenerateManifestWithFiles`, whole branch root; `Path` kept. | Same-repo local source whose files (or value files) live outside a single chart dir. |
| **stream(.refs)** | temp dir w/ `.refs/` | `GenerateManifestWithFiles` of a synthesized tree: content dir + `.refs/<ref>/`, `$ref/...` paths rewritten to relative. | Same-repo local `path:`/chart source whose same-repo `$ref` sources are also local. |

## The seven decision categories

Each category below corresponds to one part of the decision the code makes (or
must make). The tables that follow are grouped by category.

1. **STR - Strategy selection.** The core routing outcome (remote / chart-dir /
   branch-root / .refs), driven by: `chart:` vs `path:`, repo locality
   (`repoURL` vs `--repo`), and whether refs are present + their locality.
   Cardinality (`source` vs `sources`, Application vs ApplicationSet) collapses
   into "are there ref sources?" and is verified here as a parity check.
2. **TYP - Source-type resolution.** How the repo server decides
   Helm/Kustomize/Directory/Plugin: on-disk marker files, an **explicit**
   `helm:`/`kustomize:`/`directory:`/`plugin:` block (which *overrides*
   detection), the `.argocd-source.yaml` override file, and the multi-block
   error.
3. **REACH - File reachability.** The single question "must we stream the branch
   root because some required file lives outside the chart dir?" - across
   *every* way a file can escape: relative `../`, absolute `/`, `$ref`, glob,
   env-subst, remote URL, **valueFiles AND fileParameters AND
   override-injected**, plus symlinks.
4. **REF - Ref handling.** Staging `.refs/`, rewriting `$ref` paths, local vs
   external refs, the remote-chart-with-local-refs puller.
5. **REQ - Request-content invariants.** Fields that must be set on *every*
   request regardless of strategy: `KubeVersion`/`ApiVersions` (#432),
   `ProjectName`/`ProjectSourceRepos` (#416).
6. **TRV - Traversal.** App-of-apps children: routing parity with seed apps and
   `--redirect-target-revisions` handling (#446).
7. **NEG - Negatives / unsupported.** Degenerate topologies (no content source)
   and combinations the repo server explicitly rejects.

## Legend

- Testability: **U** = unit-testable against `buildManifestRequestForSource`
  (assert strategy + request fields + tarball contents); **I** = needs a real
  repo server (failure happens *inside* it) -> integration (branch-N).
- Status: ✅ covered by a named, **passing** test · 🟥 SKIP known-bug repro,
  skipped by default but executable with `RUN_KNOWN_BUG_TESTS=1` · ⏭️
  integration-only skipped placeholder · ⬜ gap, no test yet · ⚠️ partial.

> **Note on the 🟥 SKIP rows.** Every matrix row has a named test. The five 🟥
> SKIP rows (TYP5, REACH5, REACH6, REACH11, REF13) are real tests asserting the
> repo-server-correct behavior; they would fail today because our tool does not
> yet produce it. Normal `go test ./pkg/reposerverextract/...` stays green. Run
> `RUN_KNOWN_BUG_TESTS=1 go test ./pkg/reposerverextract/...` to execute the
> failing repros. They live in `gaps_test.go`.

---

## STR - Strategy selection

| # | Source | Repo | Refs | Expected strategy | Test | U/I | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| STR1 | local Helm chart | local | none | stream(chart-dir), `Path` cleared | `Strategy` row STR1 | U | ✅ |
| STR2 | remote `chart:` | external | none | remote (`streamDir==""`) | `Strategy` row STR2 | U | ✅ |
| STR3 | remote `chart:` (OCI) | external | none | remote | `Strategy` row STR3 | U | ✅ |
| STR4 | local `path:` | **cross-repo** | none | remote; `Path` **kept** (resolved in foreign repo) | `Strategy` row STR4 | U | ✅ |
| STR5 | ApplicationSet (sources under `spec.template.spec`) | - | - | identical routing to the equivalent Application | `Strategy_AppSetRoutesLikeApp` | U | ✅ |

## TYP - Source-type resolution

| # | Case | Expected | Test | U/I | Status | Notes (repo-server ref) |
| --- | --- | --- | --- | --- | --- | --- |
| TYP1 | local dir of plain manifests / jsonnet | stream(branch-root), `Path` set | `Strategy` row TYP1 | U | ✅ | Non-chart/non-kustomize dir falls through to branch root. |
| TYP2 | local Kustomize | stream(branch-root) | `Strategy` row TYP2 | U | ✅ | `kustomization.yaml` detected. |
| TYP3 | local Kustomize w/ `helmCharts` (Chart.yaml present) | stream(branch-root) | `Strategy` row TYP3 | U | ✅ | Kustomize wins over Helm when both present. |
| TYP4 | local plugin (CMP) `path:` | stream(branch-root), `Path` set | `Strategy` row TYP4 | U | ✅ | No Chart.yaml/kustomization -> branch root. branch-16 covers CMP end-to-end. |
| TYP5 | dir has `Chart.yaml` but manifest sets `directory:` explicitly | render as **Directory** | `TYP5_ExplicitDirectoryOverChartYaml` | U | 🟥 SKIP | `ExplicitType()` wins over on-disk (`types.go:3799`). Our `isLocalHelmChart` ignores the explicit block -> narrows as Helm. **Skipped by default; fails with `RUN_KNOWN_BUG_TESTS=1`.** |
| TYP6 | dir has `kustomization.yaml` but manifest sets `helm:` explicitly | stream(branch-root) | `TYP6_ExplicitHelmOverKustomization` | U | ✅ | Passes - we stream branch root for non-chart dirs anyway. |
| TYP7 | manifest sets **both** `helm:` and `kustomize:` | **hard error** "multiple application sources defined" | `TYP7_MultipleExplicitTypes_Integration` | I | ⏭️ | `ExplicitType()` errors inside the repo server (`types.go:3816`). |
| TYP8 | `.argocd-source-<appName>.yaml` injects a `helm:`/`kustomize:` block | type changes accordingly | `TYP8_ArgocdSourceInjectsType_Integration` | I | ⏭️ | Override applied **before** type detection inside the repo server (`repository.go:1872`). |
| TYP9 | dir has **both** `Chart.yaml` and `kustomization.yaml` | discovery picks **Kustomize** | `TYP9_BothChartAndKustomization_PicksKustomize` | U | ✅ | Passes - our `isKustomizeSource` check first matches the repo server (`discovery.go:65`). |
| TYP10 | source type disabled via `EnabledSourceTypes` | falls back to **Directory** | `TYP10_DisabledSourceTypeFallsBackToDirectory_Integration` | I | ⏭️ | `IsManifestGenerationEnabled` (`repository.go:1938`). We never set EnabledSourceTypes. |

## REACH - File reachability (chart-dir vs branch-root)

The fast path narrows the stream to the chart dir only when every required file
is inside it (`hasOutOfChartValueFile` -> false). Each row is a way a file may
or may not escape.

| # | Case | Expected | Test | U/I | Status | Notes (repo-server ref) |
| --- | --- | --- | --- | --- | --- | --- |
| REACH1 | Helm chart, in-chart relative values (`values.yaml`) | stream(chart-dir) | `Strategy` row REACH1 | U | ✅ | |
| REACH2 | Helm chart, out-of-chart relative values (`../shared/x`) | stream(branch-root) | `Strategy` row REACH2 | U | ✅ | |
| REACH3 | Helm chart, out-of-chart absolute values (`/env/x`) | stream(branch-root) | `Strategy` row REACH3 | U | ✅ | Leading-slash = repo-root-relative. The v0.2.10 fix (#444). |
| REACH4 | Helm chart, remote-URL values | stream(chart-dir); URL ignored for narrowing | `Strategy` row REACH4 | U | ✅ | URL schemes skipped by `hasOutOfChartValueFile`. |
| REACH5 | **`.argocd-source.yaml` adds out-of-chart `helm.valueFiles`** | must stream(branch-root) | `REACH5_ArgocdSourceOutOfChartValues_BUG` | U+I | 🟥 SKIP | **CONFIRMED BUG.** `mergeSourceParameters` injects valueFiles our `hasOutOfChartValueFile` never sees (it reads only the manifest). We narrow -> render fails. Same class as #444. **Skipped by default; fails with `RUN_KNOWN_BUG_TESTS=1`.** |
| REACH6 | valueFile with env placeholder `$ENV/x.yaml` | resolved after `env.Envsubst`, then classified | `REACH6_EnvPlaceholderValueFile` | U+I | 🟥 SKIP | `repository.go:1502`. We treat `$ARGOCD_ENV_*` as a `$ref` and skip it when classifying -> narrow. **Skipped by default; fails with `RUN_KNOWN_BUG_TESTS=1`.** |
| REACH7 | valueFile **glob** `env/*.yaml` matching out-of-chart files | stream(branch-root) | `REACH7_GlobOutOfChartValueFile` | U+I | ✅ | Passes - the `../` prefix is caught by the existing relative-escape check. |
| REACH8 | valueFile remote URL with **disallowed** scheme | **hard error** | `REACH8_DisallowedUrlScheme_Integration` | I | ⏭️ | `isURLSchemeAllowed` (`resolved.go:53`). Schemes from `HelmOptions.ValuesFileSchemes`; nil = none. |
| REACH9 | missing valueFile, `ignoreMissingValueFiles` true vs false | skip vs fail | `REACH9_IgnoreMissingValueFiles` | U+I | ✅ | Passes - an out-of-chart value file forces branch root regardless of the flag. |
| REACH10 | leading-slash valueFile resolution semantics | resolved from repo root | (in REACH3/REF8) | U | ⚠️ | Routing covered; resolution semantics not asserted directly. |
| REACH11 | `helm.fileParameters[].path` out-of-chart relative `../x` | must stream(branch-root) | `REACH11_FileParameterOutOfChart_BUG` | U+I | 🟥 SKIP | fileParameters resolve like valueFiles (`repository.go:1343`); `hasOutOfChartValueFile` ignores them. **Skipped by default; fails with `RUN_KNOWN_BUG_TESTS=1`** (same blind spot as REF13). |
| REACH12 | `helm.fileParameters[].path` remote URL | fetched by repo server | `REACH12_FileParameterRemoteUrl_Integration` | I | ⏭️ | Same scheme rules as valueFiles. |
| REACH13 | local chart, unrelated **in-bounds** symlink elsewhere in repo | routes; tarball excludes unrelated files | `Strategy_InBoundsSymlink_*` | U+I | ✅/⏭️ | Tarball exclusion is U; the repo server symlink gate (#438) is the I half. |
| REACH14 | local chart, **out-of-bounds** file symlink | render rejected by repo server | `Strategy_OutOfBoundsSymlink_Integration` | I | ⏭️ | #438 - only observable in a real render. |
| REACH15 | local chart, **directory** symlink inside chart | routes without error | `Strategy_DirectorySymlinkInChart_*` | U+I | ✅ | #448 fixed by #449. `copyDir` following also covered by `TestCopyDir_FollowsDirectorySymlink`. |

## REF - Ref handling

| # | Primary | Primary repo | Ref(s) | Expected | Test | U/I | Status |
| --- | --- | --- | --- | --- | --- | --- | --- |
| REF1 | local Helm chart | local | one local `$ref` | stream(.refs), values rewritten | `Refs` row REF1 | U | ✅ |
| REF2 | local Helm chart | local | local `$ref` w/ both ref+path (GH401) | stream(.refs) | `Refs` row REF2 | U | ✅ |
| REF3 | local Helm chart | local | one **external** `$ref` | remote + RefSources | `Refs` row REF3 | U | ✅ |
| REF4 | local plain dir (non-chart) | local | local `$ref` | stream(.refs) | `Refs` row REF4 | U | ✅ |
| REF5 | remote `chart:` | external | one local `$ref` (with puller) | stream(.refs) via puller; Chart cleared | `Refs` row REF5 | U+I | ✅ |
| REF6 | remote `chart:` | external | one local `$ref` (no puller) | remote + RefSources | `Refs` row REF6 | U | ✅ |
| REF7 | remote `chart:` | external | one external `$ref` | remote + RefSources | `Refs` row REF7 | U | ✅ |
| REF8 | remote `chart:` | external | local `$ref` w/ both ref+path | remote + RefSources | `Refs` row REF8 | U | ✅ |
| REF9 | local Helm chart | local | local `$ref` + out-of-chart abs value | stream(.refs), out-of-chart file reachable | `Refs` row REF9 | U | ✅ |
| REF10 | two local content sources | local | none | stream each independently | `Refs` row REF10 | U | ✅ |
| REF11 | local + external content (mixed) | mixed | none | stream local, remote external | `Refs` row REF11 | U | ✅ |
| REF12 | ref-only source with **no path** (repo root) | local | local `$ref` (path="") | stream(.refs), ref dir = branch root | `Refs` row REF12 | U | ✅ |
| REF13 | local chart | local | **`helm.fileParameters[].path: $ref/foo`** | path rewritten like a valueFile; ref staged | `REF13_FileParameterRef_BUG` | U+I | 🟥 SKIP | **CONFIRMED BUG.** Repo server treats fileParameters as ref candidates (`repository.go:571-575`); our `rewriteRefValueFiles` only rewrites `ValueFiles`, never `FileParameters` -> ref staged but path unrewritten -> render fails. Same class as #441/#426. **Skipped by default; fails with `RUN_KNOWN_BUG_TESTS=1`.** |

## REQ - Request-content invariants (every strategy)

| # | Case | Expected | Test | U/I | Status |
| --- | --- | --- | --- | --- | --- |
| REQ1 | `KubeVersion` / `ApiVersions` populated on R / S / S+refs | both set from the cluster | `RequestInvariants_AllStrategies` | U | ✅ (#432) |
| REQ2 | `ProjectName=default`, `ProjectSourceRepos=["*"]` on R / S / S+refs | permissive, so helm-build errors aren't masked | `RequestInvariants_AllStrategies` | U | ✅ (#416) |
| REQ3 | transient helm-dep build error surfaced (not swallowed) | original error preserved | `HelmDepBuildErrorSurfaced_Integration` | I | ⏭️ (#416) |

## TRV - Traversal (app-of-apps)

| # | Case | Expected | Test | U/I | Status |
| --- | --- | --- | --- | --- | --- |
| TRV1 | child app source = local / remote | child routes through `buildManifestRequestForSource` like a seed | `Traversal_ChildRoutesLikeSeed` | U | ✅ |
| TRV2 | child honors `--redirect-target-revisions` (in/out/empty) | redirect / leave / redirect-all | `Traversal_ChildHonorsRedirectTargetRevisions` | U | ✅ (#446 fixed by #447) |
| TRV3 | child pinned to a ref that only exists remotely | render from remote, not redirected | `Traversal_ChildRemoteOnlyRef_Integration` | I | ⏭️ |

## NEG - Negatives / unsupported

| # | Case | Expected | Test | U/I | Status | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| NEG1 | app with neither `source` nor `sources` | error | `Negatives_DegenerateTopologies` NEG1 | U | ✅ | |
| NEG2 | all sources are ref-only (no content) | error | `Negatives_DegenerateTopologies` NEG2 | U | ✅ | |
| NEG3 | empty `sources: []` | error | `Negatives_DegenerateTopologies` NEG3 | U | ✅ | |
| NEG4 | a `$ref` source that itself sets `chart:` | repo server **rejects** | `NEG4_RefSourceWithChart_Integration` | I | ⏭️ | "Helm charts are not yet not supported for 'ref' sources" (`repository.go:594`). |
| NEG5 | raw streamed single-source with `$ref` (no `.refs` staging) | fails "failed to find repo" | `NEG5_RawStreamedRefUnsupported_Integration` | I | ⏭️ | Streaming path does not populate `gitRepoPaths` (`repository.go:1565`). Validates *why* our `.refs` staging + rewrite exists - and why it must cover fileParameters (REF13). |

---

## What we found (honest summary)

**Bugs found by the unit matrix alone (the ✅ rows): 0.** Every implemented unit
row passed against `main`, or was an already-tracked issue.

**Bugs found by auditing the repo-server source: 2 confirmed, plus several
unmodeled dimensions.** This is the real payoff and it only appeared once we
stopped trusting our own model and read the repo-server. All now have failing
tests in `gaps_test.go`:

- **REACH5** (🟥 SKIP) - `.argocd-source.yaml` can add out-of-chart
  `helm.valueFiles` that `hasOutOfChartValueFile` never sees (it inspects only
  the manifest), so we narrow to the chart dir and the render fails. Same class
  as #444.
- **REF13** (🟥 SKIP) - `helm.fileParameters[].path: $ref/...` is a first-class
  ref candidate in the repo server, but `rewriteRefValueFiles` only rewrites
  `ValueFiles`, so the ref is staged while the fileParameter path is left
  unrewritten and unresolvable. Same class as #441/#426.
- **REACH11** (🟥 SKIP) - the broader form of the same blind spot: *any*
  out-of-chart `fileParameters` path (not just `$ref`) is ignored when deciding
  chart-dir vs branch-root, because `hasOutOfChartValueFile` only inspects
  `ValueFiles`.
- **TYP5** (🟥 SKIP) - an explicit `directory:` block on a dir that has a
  `Chart.yaml` is rendered as Directory by the repo server, but we narrow it as
  a Helm chart (our type detection ignores explicit blocks).
- **REACH6** (🟥 SKIP) - an env-placeholder value file (`$ARGOCD_ENV_*`) is
  treated as a `$ref` and skipped when classifying, so we may narrow when the
  resolved path actually escapes the chart dir.

All five are **latent** (no open issue yet) - exactly the "broken-but-valid case
we discover one GitHub issue at a time" this exercise was meant to pre-empt.

The implemented rows still bought us a regression net (pinning #444, #442, #449,
#447, which had no dedicated test) and closed real coverage gaps, and flushed out
four wrong assumptions in our own model:
- STR4: cross-repo source keeps `Path` (the repo server resolves it in the
  fetched foreign repo); it is **not** cleared.
- REF2: a `ref`+`path` source, routed as content, still streams `.refs` (it sees
  itself in the ref list) rather than degrading to a chart-dir stream.
- REF5/REF9: a `$ref` source with no `path` points at the **repo root**, so
  `$cfg/values.yaml` resolves to `<root>/values.yaml`.
- REACH15: on the single-source path the chart streams via the tarball
  compressor, which preserves a directory symlink **as a symlink entry**; the
  `copyDir`-follows-dir-symlink behaviour (#449) is on the refs/slow path.

## Where the next real bug most likely hides (integration)

Unit tests cannot see failures that happen **inside** the repo server. The
highest-value targets for proactive bug-hunting via branch-N integration:

- **REACH14 (#438)** - out-of-bounds file symlink rejected by the symlink gate.
- **REF5 (#441/#442)** - the *real* registry pull + same-repo ref render.
- **TRV3 (#446)** - a child pinned to a remote-only ref rendered by a real server.
- **REQ3 (#416)** - a transient helm-dependency build error surfaced unmasked.
- Plus the new ⬜ TYP / REACH / REF rows from the repo-server audit, several of
  which (TYP5-TYP10, REACH6-REACH12) need a real render to confirm end-to-end.

## Test file layout

- `strategy_test.go` - STR + TYP (on-disk) + REACH rows (the routing table and
  the symlink tests). Hosts the shared helpers (`streamStrategy`,
  `classifyStrategy`, `writeBranchFiles`, `replaceRepo`).
- `refs_test.go` - REF rows (multi-source / ref handling), one table-driven test
  with per-row subtests.
- `crosscutting_test.go` - STR5 (appset parity), REQ, TRV, NEG.
- `gaps_test.go` - the repo-server-audit rows that were not in the original
  tables: the 5 🟥 SKIP rows (skipped by default) and the integration-only ⏭️
  placeholders. Hosts the `routeSingle` helper.

## Mapping from the old A-H scheme

For anyone cross-referencing earlier commits or issues:

| Old | New |
| --- | --- |
| A1 | TYP1 |
| A2 | STR1 |
| A3 / A4 / A5 / A6 | REACH1 / REACH2 / REACH3 / REACH4 |
| A7 / A8 | TYP2 / TYP3 |
| A9 / A10 / A11 | REACH13 / REACH14 / REACH15 |
| A12 | TYP4 |
| A13 / A14 / A15 | STR2 / STR3 / STR4 |
| B1..B11 (+B5b) | REF1..REF12 |
| C1 | STR5 |
| C2 / C3 / C3b | NEG1 / NEG2 / NEG3 |
| D1 / D2 / D3 | TRV1 / TRV2 / TRV3 |
| E1 / E2 / E3 | REQ1 / REQ2 / REQ3 |
| F1 / F2 / F3 / F4 / F5 / F6 / F7 | TYP5 / TYP6 / TYP7 / REACH5 / TYP8 / TYP9 / TYP10 |
| G1..G5 | REACH6..REACH10 |
| H1 / H2 / H3 | REF13 / REACH11 / REACH12 |
| N1 / N2 | NEG4 / NEG5 |
