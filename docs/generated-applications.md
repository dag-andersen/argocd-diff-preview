# Helm/Kustomize Generated Argo CD Applications

`argocd-diff-preview` discovers applications by scanning your repository for YAML files with `kind: Application` or `kind: ApplicationSet`. If your Application manifests are not committed directly to the repository but are instead *generated* by a Helm chart or Kustomize template, you need to render them first and place the output somewhere in the branch folder before running the tool.

---

## Example scenario

Imagine a repository with this structure:

```
argocd-gitops-repo/
├── app-templates/
│   ├── helm-chart/
│   │   ├── Chart.yaml
│   │   ├── values.yaml
│   │   └── templates/
│   │       ├── app-frontend.yaml   ← generates kind: Application
│   │       └── app-backend.yaml    ← generates kind: Application
│   └── kustomize/
│       ├── base/
│       │   ├── kustomization.yaml
│       │   └── app.yaml            ← generates kind: Application
│       └── overlays/
│           └── production/
│               └── kustomization.yaml
...
```

The `app-templates/helm-chart/` Helm chart and the `app-templates/kustomize/` Kustomize overlay both generate Argo CD Application manifests when rendered. These manifests are never committed to the repository as files; they only exist as rendered output.

Without a pre-render step, `argocd-diff-preview` would find no applications at all, because there are no raw `kind: Application` YAML files anywhere in the repo.

---

## Solution: pre-render in CI

Add steps in your pipeline that render the chart and/or Kustomize overlay and write the output into the branch folder. The tool will then pick up those files exactly as if they were committed to the repository.

```yaml title=".github/workflows/generate-diff.yml" linenums="1"
name: Argo CD Diff Preview

on:
  pull_request:
    branches:
      - main

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write

    steps:
      - uses: actions/checkout@v4
        with:
          path: pull-request

      - uses: actions/checkout@v4
        with:
          ref: main
          path: main

      - name: Render Helm chart for main
        run: |
          helm template app-templates main/app-templates/helm-chart \
            > main/rendered-helm-applications.yaml

      - name: Render Helm chart for pull-request
        run: |
          helm template app-templates pull-request/app-templates/helm-chart \
            > pull-request/rendered-helm-applications.yaml

      - name: Render Kustomize overlay for main
        run: |
          kustomize build main/app-templates/kustomize/overlays/production \
            > main/rendered-kustomize-applications.yaml

      - name: Render Kustomize overlay for pull-request
        run: |
          kustomize build pull-request/app-templates/kustomize/overlays/production \
            > pull-request/rendered-kustomize-applications.yaml

      - name: Generate Diff
        run: |
          docker run \
            --network=host \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v $(pwd)/main:/base-branch \
            -v $(pwd)/pull-request:/target-branch \
            -v $(pwd)/output:/output \
            -e TARGET_BRANCH=refs/pull/${{ github.event.number }}/merge \
            -e REPO=${{ github.repository }} \
            dagandersen/argocd-diff-preview:v0.2.0

      - name: Post diff as comment
        run: |
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md --edit-last || \
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

!!! tip "Output file name and location"
    The rendered output files can be named anything and placed anywhere inside the branch folder. The tool recursively scans the entire folder for `kind: Application` and `kind: ApplicationSet` resources. You can render as many generators as you like into separate files - they will all be picked up.

---

## Watch patterns and file change detection

Pre-rendering breaks automatic file change detection. Watch patterns work by matching changed files against each application's `watch-pattern` annotation. But with pre-rendering, the Application manifests all come from a generated file (e.g. `rendered-applications.yaml`) that is never committed to the repository - so it will never appear in the list of changed files. As a result, no application will ever be triggered by watch-pattern matching, and all applications will be skipped if `--watch-if-no-watch-pattern-found=false`.

**The recommended workaround** is to detect changed files yourself in CI and pass them explicitly via `--files-changed` (or the `FILES_CHANGED` environment variable), combined with `watch-pattern` annotations on your Application templates that point at the actual source directories in your repository. This is described in detail under [Approach 2: Manual File Detection](application-selection.md#approach-2-manual-file-detection) in the Application Selection page.
