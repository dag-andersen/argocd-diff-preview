# Options

This document describes all the available options for `argocd-diff-preview`. Options can be provided via command-line flags or environment variables.

## Usage

```bash
argocd-diff-preview [FLAGS] [OPTIONS] --repo <repo> --target-branch <target-branch>
```

## Required Options

| Flag                                    | Environment Variable | Description                                                                      |
| --------------------------------------- | -------------------- | -------------------------------------------------------------------------------- |
| `--repo <repo>`                         | `REPO`               | Git Repository in format `OWNER/REPO` (e.g., `dag-andersen/argocd-diff-preview`) |
| `--target-branch <target-branch>`, `-t` | `TARGET_BRANCH`      | Target branch name (the branch you want to compare with the base branch)         |

## Flags

| Flag                                | Environment Variable              | Default | Description                                                                                                                      |
| ----------------------------------- | --------------------------------- | ------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `--create-cluster`                  | `CREATE_CLUSTER`                  | `true`  | Create a new cluster if it doesn't exist                                                                                         |
| `--disable-client-throttling`       | `DISABLE_CLIENT_THROTTLING`       | `false` | Disable client-side throttling (rely on API Priority and Fairness instead)                                                       |
| `--auto-detect-files-changed`       | `AUTO_DETECT_FILES_CHANGED`       | `false` | Auto detect files changed between branches                                                                                       |
| `--watch-if-no-watch-pattern-found` | `WATCH_IF_NO_WATCH_PATTERN_FOUND` | `false` | Render applications without watch-pattern annotation                                                                             |
| `--debug`, `-d`                     | `DEBUG`                           | `false` | Activate debug mode                                                                                                              |
| `--dry-run`                         | `DRY_RUN`                         | `false` | Show which applications would be processed without creating a cluster or generating a diff                                       |
| `--hide-deleted-app-diff`           | `HIDE_DELETED_APP_DIFF`           | `false` | Hide diff content for deleted applications (only show deletion header)                                                           |
| `--ignore-invalid-watch-pattern`    | `IGNORE_INVALID_WATCH_PATTERN`    | `false` | Ignore invalid watch-pattern Regex on Applications                                                                               |
| `--keep-cluster-alive`              | `KEEP_CLUSTER_ALIVE`              | `false` | Keep cluster alive after the tool finishes                                                                                       |
| `--kind-internal`                   | `KIND_INTERNAL`                   | `false` | Use the kind cluster's internal address in the kubeconfig (allows connecting to the cluster when running the CLI in a container) |
| `--version`, `-v`                   | -                                 | -       | Prints version information                                                                                                       |

## Options

| Option                                    | Environment Variable         | Default                                | Description                                                                         |
| ----------------------------------------- | ---------------------------- | -------------------------------------- | ----------------------------------------------------------------------------------- |
| `--argocd-chart-version <version>`        | `ARGOCD_CHART_VERSION`       | `latest`                               | Argo CD Helm Chart version                                                          |
| `--argocd-chart-name <name>`              | `ARGOCD_CHART_NAME`          | `argo`                                 | Argo CD Helm Chart name                                                             |
| `--argocd-chart-url <url>`                | `ARGOCD_CHART_URL`           | `https://argoproj.github.io/argo-helm` | Argo CD Helm Chart URL                                                              |
| `--argocd-chart-repo-username <username>` | `ARGOCD_CHART_REPO_USERNAME` | -                                      | Argo CD Helm Chart Private repository username                                      |
| `--argocd-chart-repo-password <password>` | `ARGOCD_CHART_REPO_PASSWORD` | -                                      | Argo CD Helm Chart Private repository password                                      |
| `--argocd-login-options <options>`        | `ARGOCD_LOGIN_OPTIONS`       | -                                      | Additional options to pass to `argocd login` command                                |
| `--argocd-namespace <namespace>`          | `ARGOCD_NAMESPACE`           | `argocd`                               | Namespace to use for Argo CD                                                        |
| `--base-branch <branch>`, `-b`            | `BASE_BRANCH`                | `main`                                 | Base branch name                                                                    |
| `--cluster <tool>`                        | `CLUSTER`                    | `auto`                                 | Local cluster tool. Options: `kind`, `minikube`, `k3d`, `auto`                      |
| `--cluster-name <name>`                   | `CLUSTER_NAME`               | `argocd-diff-preview`                  | Cluster name (only for kind & k3d)                                                  |
| `--diff-ignore <pattern>`, `-i`           | `DIFF_IGNORE`                | -                                      | Ignore lines in diff. Example: `v[1,9]+.[1,9]+.[1,9]+` for ignoring version changes |
| `--file-regex <regex>`, `-r`              | `FILE_REGEX`                 | -                                      | Regex to filter files. Example: `/apps_.*\.yaml`                                    |
| `--files-changed <files>`                 | `FILES_CHANGED`              | -                                      | List of files changed between branches (comma, space or newline separated)          |
| `--ignore-resources <rules>`              | `IGNORE_RESOURCES`           | -                                      | Ignore resources in diff. Format: `group:kind:name` (comma-separated, `*` wildcard) |
| `--k3d-options <options>`                 | `K3D_OPTIONS`                | -                                      | k3d options (only for k3d)                                                          |
| `--kind-options <options>`                | `KIND_OPTIONS`               | -                                      | kind options (only for kind)                                                        |
| `--line-count <count>`, `-c`              | `LINE_COUNT`                 | `7`                                    | Generate diffs with \<n\> lines of context                                          |
| `--log-format <format>`                   | `LOG_FORMAT`                 | `human`                                | Log format. Options: `human`, `json`                                                |
| `--max-diff-length <length>`              | `MAX_DIFF_LENGTH`            | `65536`                                | Max diff message character count (only limits the generated Markdown file)          |
| `--output-folder <folder>`, `-o`          | `OUTPUT_FOLDER`              | `./output`                             | Output folder where the diff will be saved                                          |
| `--redirect-target-revisions <revs>`      | `REDIRECT_TARGET_REVISIONS`  | -                                      | List of target revisions to redirect                                                |
| `--secrets-folder <folder>`, `-s`         | `SECRETS_FOLDER`             | `./secrets`                            | Secrets folder where the secrets are read from                                      |
| `--selector <selector>`, `-l`             | `SELECTOR`                   | -                                      | Label selector to filter on (e.g., `key1=value1,key2=value2`)                       |
| `--timeout <seconds>`                     | `TIMEOUT`                    | `180`                                  | Set timeout in seconds                                                              |
| `--title <title>`                         | `TITLE`                      | `Argo CD Diff Preview`                 | Custom title for the markdown output                                                |
