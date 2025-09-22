# Options

This document describes all the available options for ArgoCD Diff Preview. Options can be provided via command-line flags or environment variables.

## Usage

```
argocd-diff-preview [FLAGS] [OPTIONS] --repo <repo> --target-branch <target-branch>
```

## Required Options

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--repo <repo>` | `REPO` | Git Repository in format OWNER/REPO (e.g., `dag-andersen/argocd-diff-preview`) |
| `--target-branch <target-branch>`, `-t` | `TARGET_BRANCH` | Target branch name (the branch you want to compare with the base branch) |

## Flags

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--debug`, `-d` | `DEBUG` | `false` | Activate debug mode |
| `--dry-run` | `DRY_RUN` | `false` | Perform a trial run listing the applications that would be processed |
| `--ignore-invalid-watch-pattern` | `IGNORE_INVALID_WATCH_PATTERN` | `false` | Ignore invalid watch pattern Regex on Applications |
| `--keep-cluster-alive` | `KEEP_CLUSTER_ALIVE` | `false` | Keep cluster alive after the tool finishes |
| `--help`, `-h` | - | - | Prints help information |
| `--version`, `-V` | - | - | Prints version information |

## Options

| Option | Environment Variable | Default | Description |
|--------|---------------------|---------|-------------|
| `--argocd-chart-version <version>` | `ARGOCD_CHART_VERSION` | `latest` | Argo CD Helm Chart version |
| `--argocd-chart-name <name>` | `ARGOCD_CHART_NAME` | `argo` | Argo CD Helm Chart name |
| `--argocd-chart-url <url>` | `ARGOCD_CHART_URL` | `https://argoproj.github.io/argo-helm` | Argo CD Helm Chart URL |
| `--argocd-namespace <namespace>` | `ARGOCD_NAMESPACE` | `argocd` | Namespace to use for Argo CD |
| `--base-branch <branch>`, `-b` | `BASE_BRANCH` | `main` | Base branch name |
| `--cluster <tool>` | `CLUSTER` | `auto` | Local cluster tool. Options: kind, minikube, auto |
| `--cluster-name <name>` | `CLUSTER_NAME` | `argocd-diff-preview` | Cluster name (only for kind) |
| `--kind-options <options>` | `KIND_OPTIONS` | - | Additional options for kind cluster creation |
| `--kind-internal` | `KIND_INTERNAL` | `false` | Use the kind cluster's internal address in the kubeconfig. Allows connecting to it when running the CLI in a container. |
| `--diff-ignore <pattern>`, `-i` | `DIFF_IGNORE` | - | Ignore lines in diff. Example: `v[1,9]+.[1,9]+.[1,9]+` for ignoring version changes |
| `--file-regex <regex>`, `-r` | `FILE_REGEX` | - | Regex to filter files. Example: `/apps_.*\.yaml` |
| `--files-changed <files>` | `FILES_CHANGED` | - | List of files changed between branches (comma, space or newline separated) |
| `--line-count <count>`, `-c` | `LINE_COUNT` | `7` | Generate diffs with \<n\> lines of context |
| `--log-format <format>` | `LOG_FORMAT` | `human` | Log format. Options: human, json |
| `--max-diff-length <length>` | `MAX_DIFF_LENGTH` | `65536` | Max diff message character count. It only limits the generated Markdown file |
| `--output-folder <folder>`, `-o` | `OUTPUT_FOLDER` | `./output` | Output folder where the diff will be saved |
| `--redirect-target-revisions <revs>` | `REDIRECT_TARGET_REVISIONS` | - | List of target revisions to redirect |
| `--secrets-folder <folder>`, `-s` | `SECRETS_FOLDER` | `./secrets` | Secrets folder where the secrets are read from |
| `--selector <selector>`, `-l` | `SELECTOR` | - | Label selector to filter on (e.g., `key1=value1,key2=value2`) |
| `--timeout <seconds>` | `TIMEOUT` | `180` | Set timeout in seconds |
| `--title <title>` | `TITLE` | `Argo CD Diff Preview` | Custom title for the markdown output |
