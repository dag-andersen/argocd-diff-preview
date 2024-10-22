# Options
```
USAGE:
    argocd-diff-preview [FLAGS] [OPTIONS] --repo <repo> --target-branch <target-branch>

FLAGS:
    -d, --debug      Activate debug mode
    -h, --help       Prints help information
    -V, --version    Prints version information

OPTIONS:
        --argocd-chart-version <version>
                Argo CD Helm Chart version 
                [env: ARGOCD_CHART_VERSION=]

    -b, --base-branch <base-branch>
                Base branch name
                [env: BASE_BRANCH=]  [default: main]

        --base-branch-folder <folder>
                Base branch folder 
                [env: BASE_BRANCH_FOLDER=]  [default: base-branch]

    -i, --diff-ignore <diff-ignore>
                Ignore lines in diff. Example: use 'v[1,9]+.[1,9]+.[1,9]+' 
                for ignoring changes caused by version changes following semver 
                [env: DIFF_IGNORE=]

    -r, --file-regex <file-regex>
                Regex to filter files. Example: "/apps_.*\.yaml" 
                [env: FILE_REGEX=]

        --files-changed <files-changed>
                List of files changed between the two branches.
                Input must be a comma or space separated list of strings.
                When provided, only Applications watching these files will be rendered
                [env: FILES_CHANGED=]

    -c, --line-count <line-count>
                Generate diffs with <n> lines above and below the highlighted 
                changes in the diff. 
                [env: LINE_COUNT=]  [Default: 10]

        --local-cluster-tool <tool>
                Local cluster tool. Options: kind, minikube
                [env: LOCAL_CLUSTER_TOOL=] [default: auto]

        --max-diff-length <length>
                Max diff message character count.
                [env: MAX_DIFF_LENGTH=]  [Default: 65536] (GitHub comment limit)

    -o, --output-folder <output-folder>
                Output folder where the diff will be saved 
                [env: OUTPUT_FOLDER=]  [default: ./output]

        --repo <repo>
                Git Repository. Format: OWNER/REPO 
                [env: REPO=]

    -s, --secrets-folder <secrets-folder>
                Secrets folder where the secrets are read from 
                [env: SECRETS_FOLDER=]  [default: ./secrets]

    -l, --selector <selector>
                Label selector to filter on. 
                Supports '=', '==', and '!='. (e.g. -l key1=value1,key2=value2) 
                [env: SELECTOR=]

    -t, --target-branch <target-branch>
                Target branch name 
                [env: TARGET_BRANCH=]

        --target-branch-folder <folder>
                Target branch folder 
                [env: TARGET_BRANCH_FOLDER=]  [default: target-branch]

        --timeout <timeout>
                Set timeout for waiting for Applications to become 'OutOfSync' 
                [env: TIMEOUT=]  [default: 180]
```