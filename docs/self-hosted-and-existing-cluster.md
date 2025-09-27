# 🚧 BETA 🚧: Self-Hosted Runner with Existing Cluster

> This feature combines the speed benefits of an existing cluster with enhanced security by running the pipeline from inside your cluster. This approach eliminates the need to share cluster credentials with your CI/CD pipeline.

Instead of providing cluster credentials to GitHub Actions, you run the pipeline from inside the same cluster as your dedicated Argo CD instance using self-hosted GitHub Actions runners.

This approach offers the best of both worlds: fast execution (no cluster creation overhead) and enhanced security (no credential sharing).

## How It Works

1. **Install Action Runner Controller (ARC)** in your cluster alongside the dedicated Argo CD instance
2. **The self-hosted runner accesses Argo CD secrets directly** using `kubectl get secrets -n argocd`
3. **These secrets are automatically passed** to `argocd-diff-preview` for authentication with Git repositories and Helm registries
4. **The tool runs exactly as before**, but without any credential management complexity

## Requirements

- A Kubernetes cluster with Argo CD installed (dedicated instance, not production)
- Action Runner Controller (ARC) installed in the cluster
- Proper RBAC configuration for the runner service account
- GitHub repository configured to use the self-hosted runner

## Example Demo

### _Step 1_: Create cluster (skip if you already have a cluster with Argo CD installed)

```bash
kind create cluster --name existing-cluster
helm repo add argo https://argoproj.github.io/argo-helm
helm install argo-cd argo/argo-cd --version 8.0.3
```

### _Step 2_: Install Action Runner Controller (ARC)

Install the ARC controller and runner set:

```bash
helm install arc arc-systems/gha-runner-scale-set-controller \
  --version 0.12.1 \
  --namespace arc-systems \
  --create-namespace

helm install arc-runner-set arc-runners/gha-runner-scale-set \
  --version 0.12.1 \
  --namespace arc-runners \
  --create-namespace \
  -f arc-runner-set.yaml
```

Create the `arc-runner-set.yaml` configuration:

```yaml
# arc-runner-set.yaml
githubConfigUrl: "https://github.com/<org>/<repo>"
githubConfigSecret: arc-runner-auth

controllerServiceAccount:
  name: arc-gha-rs-controller
  namespace: arc-systems

runnerScaleSetName: arc-runner-test

template:
  spec:
    serviceAccountName: arc-runner
    automountServiceAccountToken: true
```

Create the authentication secret `arc-runner-auth`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: arc-runner-auth
  namespace: arc-runners
type: Opaque
stringData:
  github_token: "your-github-token-here"
```

### _Step 3_: Configure RBAC

Create the necessary service account and RBAC permissions:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: arc-runner
  namespace: arc-runners
---
kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: arc-runner-diff-preview
  namespace: argocd  # or your Argo CD namespace
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["*"]
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: arc-runner-diff-preview
  namespace: argocd  # or your Argo CD namespace
subjects:
  - kind: ServiceAccount
    name: arc-runner
    namespace: arc-runners
roleRef:
  kind: Role
  name: arc-runner-diff-preview
  apiGroup: rbac.authorization.k8s.io
```

> **Important!** This documentation might be outdated. Please refer to the newest way to install ARC [here](https://docs.github.com/en/actions/tutorials/use-actions-runner-controller/quickstart).

### _Step 4_: Create GitHub Actions Workflow

Create a workflow that uses your self-hosted runner:

```yaml
name: Diff Preview

on:
  pull_request:
    branches:
    - "main"

jobs:
  diff-preview:
    name: Diff Preview
    runs-on: arc-runner-test  # replace with your runner name
    permissions:
      contents: read
      pull-requests: write

    steps:
      - uses: actions/checkout@v4
        with:
          path: target-branch
          fetch-depth: 0

      - uses: actions/checkout@v4
        with:
          ref: main
          path: base-branch

      - name: Setup kubectl
        uses: azure/setup-kubectl@v4
        id: setup-kubectl

      - name: Install Argo CD CLI
        run: |
          curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
          sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
          rm argocd-linux-amd64
          argocd --help

      - name: Install argocd-diff-preview
        run: |
          curl -LJO https://github.com/dag-andersen/argocd-diff-preview/releases/download/v0.1.17/argocd-diff-preview-Linux-x86_64.tar.gz
          tar -xvf argocd-diff-preview-Linux-x86_64.tar.gz
          sudo mv argocd-diff-preview /usr/local/bin
          argocd-diff-preview --version

      - name: Generate Diff
        run: |
          argocd-diff-preview \
            --repo ${{ github.repository }} \
            --base-branch main \
            --target-branch refs/pull/${{ github.event.number }}/merge \
            --argocd-namespace=default \
            --create-cluster=false

      - name: Print start of diff
        run: |
          head -n 200 output/diff.md

      - name: Comment preview
        run: |
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md --edit-last || \
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Alternative: Using Docker (if preferred)

If you prefer to use the Docker image instead of the binary:

```yaml
      - name: Generate Diff
        run: |
          docker run \
            --network host \
            -v ~/.kube:/root/.kube \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v $(pwd)/output:/output \
            -v $(pwd)/base-branch:/base-branch \
            -v $(pwd)/target-branch:/target-branch \
            -e TARGET_BRANCH=refs/pull/${{ github.event.number }}/merge \
            -e REPO=${{ github.repository }} \
            dagandersen/argocd-diff-preview:v0.1.17 \
            --argocd-namespace=default \
            --create-cluster=false
```

## Expected Output

When running successfully, you'll see output similar to:

```
✨ Running with:
✨ - reusing existing cluster
✨ - base-branch: main
✨ - target-branch: refs/pull/123/merge
✨ - output-folder: ./output
✨ - argocd-namespace: default
✨ - repo: your-org/your-repo
✨ - timeout: 180 seconds
🔑 Unique ID for this run: 60993
🤖 Fetching all files for branch (branch: main)
🤖 Found 52 files in dir base-branch (branch: main)
...
🦑 Logging in to Argo CD through CLI...
🦑 Logged in to Argo CD successfully
🤖 Converting ApplicationSets to Applications in both branches
...
🤖 Patching 19 Applications (branch: main)
🤖 Patching 19 Applications (branch: refs/pull/123/merge)
🤖 Rendered 11 out of 38 applications (timeout in 175 seconds)
🧼 Waiting for all application deletions to complete...
🧼 All application deletions completed
🤖 Got all resources from 19 applications from base-branch and got 19 from target-branch in 7s
🔮 Generating diff between main and refs/pull/123/merge
🙏 Please check the ./output/diff.md file for differences
✨ Total execution time: 10s
```

## CIDR Collision

When using Action Runner Controller (ARC), you may need to ensure that the Service and Pod CIDRs of the ephemeral kind cluster created by `argocd-diff-preview` don't overlap with your host cluster's CIDRs.

The default CIDRs are:

| Service | CIDR          |
| ------- | ------------- |
| Service | 10.96.0.0/16  |
| Pod     | 10.244.0.0/16 |

To configure kind with different CIDRs:

1. Create a file in your repo, for instance `hack/kind.yaml`:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  serviceSubnet: "10.80.0.0/16"
  podSubnet: "10.128.0.0/16"
```

2. Add the flag `--kind-options '--config /base-branch/hack/kind.yaml'` to `argocd-diff-preview`:

```bash
argocd-diff-preview \
  --repo ${{ github.repository }} \
  --base-branch main \
  --target-branch refs/pull/${{ github.event.number }}/merge \
  --argocd-namespace=default \
  --create-cluster=false \
  --kind-options '--config /base-branch/hack/kind.yaml'
```

## Troubleshooting

### Runner Not Starting
- Verify the `githubConfigSecret` exists and contains a valid GitHub token
- Check that the runner scale set name matches your workflow configuration
- Ensure proper RBAC permissions are configured

### Permission Errors
- Verify the service account has proper permissions in the Argo CD namespace
- Check that `automountServiceAccountToken: true` is set in the runner template

### Network Issues
- If you encounter CIDR conflicts, configure custom CIDRs as shown above
- Ensure the runner can access Docker socket if using the Docker approach

For more detailed troubleshooting, refer to the [ARC documentation](https://github.com/actions/actions-runner-controller).
