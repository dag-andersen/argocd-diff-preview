# Application Selection

Rendering manifests for all applications in your repository on every pull request can be time-consuming, especially in large monorepos. By default, `argocd-diff-preview` renders all applications it finds, but you can significantly speed up the process by limiting which applications are rendered.

This page describes **4 strategies** for controlling which applications are rendered:

1. **Rendering only changed applications** - Automatically detect and render only applications affected by file changes
2. **Ignoring individual applications** - Explicitly exclude specific applications from rendering
3. **Label selectors** - Filter applications based on Kubernetes labels
4. **File path regex** - Target applications based on their file path location

---

## 1. Rendering Only Changed Applications

The most efficient way to optimize rendering is to process only applications that are actually affected by changes in your pull request. This approach uses annotations to define which files each application depends on, then automatically renders only the relevant applications.

### Option A: Watch Pattern Annotation

The `argocd-diff-preview/watch-pattern` annotation allows you to specify which file paths should trigger a render for each application. When files matching the pattern change in a pull request, the application will be rendered.

**How it works:**

- Add the `argocd-diff-preview/watch-pattern` annotation to your Application or ApplicationSet manifests
- The annotation accepts a comma-separated list of file paths or regex patterns
- Applications are automatically rendered when their watch-patterns match changed files
- Applications are always rendered if their own manifest file changes (no need to include it in the pattern)

!!! important "Note"
    The `argocd-diff-preview/watch-pattern` annotation must exist in the base branch for filtering to work. This ensures the tool knows which files to watch before comparing branches.

**Example: Application with Watch Pattern**

In this example, the `my-app` application will be rendered if:
- Changes are made to files in `examples/helm/charts/myApp/` (matching the regex pattern `examples/helm/charts/myApp/.*`)
- Changes are made to the `examples/helm/values/filtered.yaml` file
- Changes are made to the Application manifest itself (automatic behavior)

```yaml title="Application" hl_lines="7-9"
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
  annotations:
    argocd-diff-preview/watch-pattern: |
      examples/helm/charts/myApp/.*,
      examples/helm/values/filtered.yaml
spec:
  sources:
    - repoURL: https://github.com/dag-andersen/argocd-diff-preview
      ref: local-files
    - path: examples/helm/charts/myApp
      repoURL: https://github.com/dag-andersen/argocd-diff-preview
      helm:
        valueFiles:
          - $local-files/examples/helm/values/filtered.yaml
  # ...
```

**Example: ApplicationSet with Watch Pattern**

For ApplicationSets, add the annotation in two places:
- On the ApplicationSet itself (`metadata.annotations`)
- On the template that generates applications (`spec.template.metadata.annotations`)

The template annotation can use generator variables like `{{ .path.basename }}` to create application-specific watch-patterns.

```yaml title="ApplicationSet" hl_lines="7-9 21-23"
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: my-appset
  namespace: argocd
  annotations:
    argocd-diff-preview/watch-pattern: |
      examples/helm/charts/.*, 
      examples/helm/values/filtered.yaml
spec:
  generators:
    - git:
        repoURL: https://github.com/dag-andersen/argocd-diff-preview
        revision: HEAD
        directories:
          - path: examples/helm/charts/*
  template:
    metadata:
      name: '{{ .path.basename }}'
      annotations:
        argocd-diff-preview/watch-pattern: |
          examples/helm/charts/{{ .path.basename }}/.*, 
          examples/helm/values/filtered.yaml
    spec:
      project: {{ .path.basename }}
      source:
        repoURL: https://github.com/dag-andersen/argocd-diff-preview
        targetRevision: HEAD
        path: '{{ .path.path }}'
        helm:
          valueFiles:
            - ../../values/filtered.yaml
            - values.yaml
  # ...
```

### Option B: Argo CD Manifest Paths Annotation (BETA)

As an alternative to the `watch-pattern` annotation, you can use Argo CD's native `argocd.argoproj.io/manifest-generate-paths` annotation. This annotation serves a similar purpose: defining which file paths should trigger a render for the application.

**Example:**

```yaml title="Application" hl_lines="7"
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
  annotations:
    argocd.argoproj.io/manifest-generate-paths: .
spec:
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
  # ...
```

For more details on this annotation, see the [Argo CD documentation](https://argo-cd.readthedocs.io/en/stable/operator-manual/high_availability/#manifest-paths-annotation).

### Implementing Changed File Detection in CI/CD

Once you've added watch-pattern annotations to your applications, configure your CI/CD pipeline to detect changed files and use them for filtering. Here are two approaches:

#### Approach 1: Automatic Detection (Recommended)

The simplest approach is to use the `--auto-detect-files-changed` flag. The tool will automatically determine which files changed in the pull request and match them against the watch-patterns.

**Configuration options:**

- `--auto-detect-files-changed=true` - Enables automatic file change detection
- `--watch-if-no-watch-pattern-found=true` - Renders applications that don't have a watch-pattern annotation 
- `--watch-if-no-watch-pattern-found=false` - Skips applications that don't have a watch-pattern annotation (default behavior)

**How it works:**
- Applications with a `watch-pattern` annotation are rendered only if their patterns match changed files
- Applications without a `watch-pattern` annotation follow the `--watch-if-no-watch-pattern-found` setting
- Applications are always rendered if their own manifest file changes

```yaml title=".github/workflows/generate-diff.yml" linenums="1" hl_lines="36-37"
name: Generate Diff

on:
  pull_request:
    branches:
      - main

jobs:
  generate-diff:
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

      - name: Generate Diff
        run: |
          docker run \
            --network=host \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v $(pwd)/main:/base-branch \
            -v $(pwd)/pull-request:/target-branch \
            -v $(pwd)/output:/output \
            dagandersen/argocd-diff-preview:v0.1.17 \
            --target-branch=refs/pull/${{ github.event.number }}/merge \
            --repo=${{ github.repository }} \
            --auto-detect-files-changed=true \
            --watch-if-no-watch-pattern-found=true
```

#### Approach 2: Manual File Detection

For more control over file detection, you can manually detect changed files and pass them to the tool using the `--files-changed` option or `FILES_CHANGED` environment variable. This approach is useful if you have custom logic for determining which files matter.

**How it works:**
- Use a GitHub Action like [`tj-actions/changed-files`](https://github.com/tj-actions/changed-files) to detect changed files
- Pass the file list to `argocd-diff-preview`
- Only applications with matching `watch-pattern` annotations will be rendered
- Applications without a `watch-pattern` annotation follow the `--watch-if-no-watch-pattern-found` setting

```yaml title=".github/workflows/generate-diff.yml" linenums="1" hl_lines="16-18 39"
name: Generate Diff

on:
  pull_request:
    branches:
      - main

jobs:
  generate-diff:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write

    steps:
      - name: Get changed files
        id: changed-files
        uses: tj-actions/changed-files@v45

      - uses: actions/checkout@v4
        with:
          path: pull-request

      - uses: actions/checkout@v4
        with:
          ref: main
          path: main

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
            -e FILES_CHANGED="${{ steps.changed-files.outputs.all_changed_files }}" \
            dagandersen/argocd-diff-preview:v0.1.19
```

---

## 2. Ignoring Individual Applications

Sometimes you need to permanently exclude specific applications from diff previews. This is useful for applications that:
- Are rarely updated and don't need review
- Generate very large diffs that aren't useful
- Have sensitive information you don't want in pull request comments

Add the `argocd-diff-preview/ignore: "true"` annotation to any Application or ApplicationSet manifest to skip it during rendering.

**Example: Ignoring an Application**

```yaml title="Application" hl_lines="7"
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
  annotations:
    argocd-diff-preview/ignore: "true"
spec:
  # ...
```

**Example: Ignoring an ApplicationSet**

```yaml title="ApplicationSet" hl_lines="7"
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: my-appset
  namespace: argocd
  annotations:
    argocd-diff-preview/ignore: "true"
spec:
  # ...
```

!!! note "Combining with other filters"
    The ignore annotation takes precedence over other selection methods. An ignored application will never be rendered, even if it matches label selectors or file patterns.

---

## 3. Label Selectors

Label selectors provide a flexible way to filter applications using Kubernetes labels. This is particularly useful when you organize your applications by team, environment, or criticality.

Use the `--selector` flag with label matching expressions. The tool supports standard Kubernetes label selector operators:
- `=` or `==` for equality
- `!=` for inequality

**Example: Filter by team**

```bash
argocd-diff-preview --selector "team=platform"
```

This command renders only applications with the label `team: platform`. All applications without this label are skipped.

**Example: Application with labels**

```yaml title="Application" hl_lines="6-8"
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
  labels:
    team: platform
    environment: production
spec:
  # ...
```

**Common use cases:**
- Filter by team: `--selector "team=platform"`
- Filter by environment: `--selector "environment=production"`
- Exclude specific values: `--selector "criticality!=low"`

!!! tip "Multiple selectors"
    You can combine multiple label selectors to create more specific filters based on your application organization structure.

---

## 4. Application File Path Regex

File path regex filtering is useful for targeting applications based on their location in your repository structure. This is especially helpful in monorepos where different teams or projects maintain applications in separate directories.

Use the `--file-regex` flag to specify a regular expression pattern. Only Application and ApplicationSet manifests whose file paths match the pattern will be rendered.

**Example: Filter by team directory**

If *Team A* maintains their applications in a `team-a/` directory:

```bash
argocd-diff-preview --file-regex="team-a/"
```

This renders only applications whose file paths contain `team-a/`, such as:
- `apps/team-a/frontend.yaml`
- `team-a/services/backend.yaml`
- `infrastructure/team-a/database.yaml`

**Example: Filter by multiple directories**

```bash
argocd-diff-preview --file-regex="(team-a|team-b)/"
```

This renders applications from either `team-a/` or `team-b/` directories.

**Common patterns:**
- Single team: `--file-regex="team-a/"`
- Multiple teams: `--file-regex="(team-a|team-b)/"`
- Environment-specific: `--file-regex="production/"`
- Exclude directories: `--file-regex="^(?!.*/deprecated/).*$"`
