# App of Apps

The [App of Apps pattern](https://argo-cd.readthedocs.io/en/stable/operator-manual/cluster-bootstrapping/) is a common Argo CD pattern where a parent Application renders child Application manifests. The parent application points to a directory of Application YAML files, and Argo CD creates those child applications automatically.

`argocd-diff-preview` already works with the App of Apps pattern **out of the box** - as long as the child Applications (or ApplicationSets) exist as plain YAML manifests in your repository. The tool scans your repository files, finds every `kind: Application` and `kind: ApplicationSet`, and renders them all.

However, if your child Applications are *generated* at render time by a parent - for example, a root Application that points to a Helm chart which templates child Application manifests - they never exist as files in the repo. In that case you need either pre-rendering or the `--traverse-app-of-apps` flag.

---

## When you don't need `--traverse-app-of-apps`

If your child Applications are committed as plain YAML files in the repository, the tool picks them up automatically. No extra flags needed.

---

## When you do need it

If child Applications are generated dynamically (e.g. by a Helm chart or Kustomize overlay inside a parent Application), they are invisible to the file scanner. You have two options:

### Option 1: Pre-render your Application manifests

If your applications are generated from Helm or Kustomize, you can pre-render them in your CI pipeline and place the output in the branch folder. `argocd-diff-preview` will then pick them up as regular files. See [Helm/Kustomize generated Argo CD applications](./generated-applications.md) for details and examples.

### Option 2: Use `--traverse-app-of-apps`

!!! warning "🧪 Experimental"
    The `--traverse-app-of-apps` flag is an experimental feature. Its behaviour may change in future releases without a deprecation notice.

!!! tip "Prefer pre-rendering when possible"
    The `--traverse-app-of-apps` flag is **slower** and **more limited** than the standard rendering flow. Consider [pre-rendering](./generated-applications.md) first.

Only use `--traverse-app-of-apps` when the child Applications are *not* committed as plain manifests to the repository AND can *not* be pre-rendered easily.

---

## How it works

When `--traverse-app-of-apps` is enabled, the tool performs a breadth-first expansion:

1. **Render a parent application** - exactly as it normally would.
2. **Scan the rendered manifests** for any resources of `kind: Application`.
3. **Enqueue child applications** - each discovered child is added to the render queue as if it were a top-level application.
4. **Repeat** - until no new child applications are found or the maximum depth is reached.

---

## Requirements

- **Render method:** `--traverse-app-of-apps` requires `--render-method=repo-server-api`. The flag will cause an error if used with any other render method.

---

## Usage

```bash
argocd-diff-preview \
  --render-method=repo-server-api \
  --traverse-app-of-apps
```

Or via environment variables:

```bash
RENDER_METHOD=repo-server-api \
TRAVERSE_APP_OF_APPS=true \
argocd-diff-preview
```

---

## Application selection

Child applications discovered through the App of Apps expansion are subject to the same [application selection](application-selection.md) filters as top-level applications:

| Filter | Applied to child apps? |
|---|---|
| Watch-pattern annotations (`--files-changed`) | ✅ Yes - the child app's own annotations are evaluated |
| Label selectors (`--selector`) | ✅ Yes |
| `--watch-if-no-watch-pattern-found` | ✅ Yes |
| File path regex (`--file-regex`) | ❌ No - child apps have no physical file path |

!!! warning "Filters apply at every level of the tree"
    A child application is only discovered if its **parent is rendered**. If a parent application is excluded by a selector, watch-pattern, or any other filter, the tool never renders it - and therefore never sees its children. This means changes further down the tree can go undetected.

    For example, if you use `--selector "team=frontend"` and your root app does not have the label `team: frontend`, none of its children will be processed - even if a child app *does* carry that label.

    When using application selection filters together with `--traverse-app-of-apps`, make sure your **root and intermediate applications pass the filters**, not just the leaf applications you care about.

!!! tip "Watch patterns on child apps"
    You can add `argocd-diff-preview/watch-pattern` or `argocd.argoproj.io/manifest-generate-paths` annotations directly to your child Application manifests. These annotations are evaluated against the PR's changed files, just like they are for top-level applications.

### Recommended: use `--file-regex` to select only root applications

If you follow the App of Apps pattern, a practical approach is to use `--file-regex` to select only the root application files and let the tree traversal take care of the rest. This way the root apps are always rendered, and all children are discovered automatically.

For example, if your root application is defined in `apps/root.yaml`:

```bash
argocd-diff-preview \
  --render-method=repo-server-api \
  --traverse-app-of-apps \
  --file-regex="^apps/root\.yaml$"
```

This avoids the problem described above where filters accidentally exclude a parent and silently hide changes in its children.

---

## Cycle and diamond protection

The expansion engine tracks every `(app-id, branch)` pair it has already rendered. This means:

- **Cycles** (A → B → A) are detected and broken automatically.
- **Diamond dependencies** (A → C and B → C) cause C to be rendered only once.

---

## Depth limit

The expansion stops after a maximum depth of **10 levels** to guard against runaway trees. If your App of Apps hierarchy is deeper than 10 levels, applications beyond that depth will not be rendered and a warning will be logged.

---

## Output

Diff output for child applications looks identical to that of top-level applications. The application name in the diff header includes a breadcrumb showing which parent generated it, making it easy to trace the app-of-apps tree.

For example, a diff generated with a two-level app-of-apps hierarchy might look like this:

```
<details>
<summary>child-app-1 (parent: my-root-app)</summary>
<br>

#### ConfigMap: default/some-config
...
```
