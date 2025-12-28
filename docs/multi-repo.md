# Multi-repo support

This page explains how to use `argocd-diff-preview` when your Argo CD Applications and their managed Kubernetes resources live in separate repositories.

## Terminology

Let's define the two repositories we're working with:

| Repository | Contains |
|------------|----------|
| **Application Repository** | Argo CD `Application` and `ApplicationSet` manifests |
| **Resource Repository** | Kubernetes resources (Helm charts, Kustomize overlays, plain YAML) referenced by the applications |

If you want diffs triggered by changes in **either** repository, you'll need a pipeline in both.


## Two key rules

### Rule 1: `--repo` must match the repository with the PR

The `--repo` flag tells `argocd-diff-preview` which repository has the change. The tool uses this to redirect the correct `targetRevision` to the PR branch.

| Flag | Purpose |
|------|---------|
| `--repo` | Must match the repository where the PR was opened |
| `--target-branch` | The PR branch that applications should be redirected to |

```bash
docker run \
  --network=host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/main:/base-branch \
  -v $(pwd)/pull-request:/target-branch \
  -v $(pwd)/output:/output \
  -e REPO=<org>/<repo-with-pr> \       # ⬅️ Must match the repo with the PR
  -e TARGET_BRANCH=<pr-branch> \       # ⬅️ The branch to redirect applications to
  dagandersen/argocd-diff-preview:v0.1.20
```

!!! note "Why does this matter?"
    The tool only redirects applications whose `spec.source.repoURL` matches the `--repo` flag. If you specify the wrong repo, the applications won't point to your PR branch.

---

### Rule 2: Always clone the Application Repository

The `/base-branch` and `/target-branch` folders must contain the **Application Repository** — this is where `argocd-diff-preview` looks for `Application` and `ApplicationSet` manifests.

This means even when running a pipeline in the Resource Repository, you still need to clone the Application Repository into these folders.

---

## Example pipelines

### Pipeline in the Application Repository

This is the standard setup — nothing unusual here:

- Clone the Application Repository (this repo)
- Provide secrets for both repositories
- Set `--repo` to the Application Repository

```yaml title=".github/workflows/diff.yml (Application Repository)" hl_lines="15 19 66-68"
name: Argo CD Diff Preview

on:
  pull_request:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write

    steps:
      - uses: actions/checkout@v4       ⬅️ We pull the Application Repository
        with:
          path: pull-request

      - uses: actions/checkout@v4       ⬅️ We pull the Application Repository
        with:
          ref: main
          path: main

      - name: Prepare secrets
        run: |
          mkdir secrets

          # Secret for Application Repository
          cat > secrets/app-repo-creds.yaml << "EOF"
          apiVersion: v1
          kind: Secret
          metadata:
            name: app-repo-creds
            namespace: argocd
            labels:
              argocd.argoproj.io/secret-type: repo-creds
          stringData:
            url: https://github.com/<org>/<application-repository>
            password: ${{ secrets.GITHUB_TOKEN }}
            username: not-used
          EOF

          # Secret for Resource Repository
          cat > secrets/resource-repo-creds.yaml << "EOF"
          apiVersion: v1
          kind: Secret
          metadata:
            name: resource-repo-creds
            namespace: argocd
            labels:
              argocd.argoproj.io/secret-type: repo-creds
          stringData:
            url: https://github.com/<org>/<resource-repository>
            password: ${{ secrets.RESOURCE_REPO_TOKEN }}
            username: not-used
          EOF

      - name: Generate Diff
        run: |
          docker run \
            --network=host \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v $(pwd)/main:/base-branch \
            -v $(pwd)/pull-request:/target-branch \
            -v $(pwd)/output:/output \
            -v $(pwd)/secrets:/secrets \                                         ⬅️ Mount the secrets folder
            -e TARGET_BRANCH=refs/pull/${{ github.event.number }}/merge \        ⬅️ The PR branch on the Application Repository
            -e REPO=${{ github.repository }} \                                   ⬅️ Application Repository
            dagandersen/argocd-diff-preview:v0.1.20

      - name: Post diff as comment
        run: |
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md --edit-last || \
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

#### Local equivalent

```bash hl_lines="1 2 15"
git clone https://github.com/<org>/<application-repository> base-branch --depth 1 -q -b main
git clone https://github.com/<org>/<application-repository> target-branch --depth 1 -q -b pr-branch

mkdir secrets
# store some secrets in the ./secrets folder

docker run \
    --network host \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v $(pwd)/output:/output \
    -v $(pwd)/base-branch:/base-branch \
    -v $(pwd)/target-branch:/target-branch \
    -v $(pwd)/secrets:/secrets \
    -e TARGET_BRANCH=pr-branch \
    -e REPO=<org>/<application-repository> \
    dagandersen/argocd-diff-preview:v0.1.20
```

### Pipeline in the Resource Repository

This is where it gets interesting:

- Clone the **Application Repository** (not the current repo!)
- Provide secrets for both repositories  
- Set `--repo` to the **Resource Repository** (where the PR is)

```yaml title=".github/workflows/diff.yml (Resource Repository)" hl_lines="19-21 26-28 73-75"
name: Argo CD Diff Preview

on:
  pull_request:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      pull-requests: write

    steps:
      # ⚠️ Clone the APPLICATION Repository, not this Resource Repository!
      # Both clone `main` because there's no PR on the Application Repository
      - uses: actions/checkout@v4
        with:
          repository: <org>/<application-repository>
          token: ${{ secrets.APP_REPO_TOKEN }}
          ref: main
          path: pull-request

      - uses: actions/checkout@v4
        with:
          repository: <org>/<application-repository>
          token: ${{ secrets.APP_REPO_TOKEN }}
          ref: main
          path: main

      - name: Prepare secrets
        run: |
          mkdir secrets

          # Secret for Application Repository
          cat > secrets/app-repo-creds.yaml << "EOF"
          apiVersion: v1
          kind: Secret
          metadata:
            name: app-repo-creds
            namespace: argocd
            labels:
              argocd.argoproj.io/secret-type: repo-creds
          stringData:
            url: https://github.com/<org>/<application-repository>
            password: ${{ secrets.APP_REPO_TOKEN }}
            username: not-used
          EOF

          # Secret for Resource Repository
          cat > secrets/resource-repo-creds.yaml << "EOF"
          apiVersion: v1
          kind: Secret
          metadata:
            name: resource-repo-creds
            namespace: argocd
            labels:
              argocd.argoproj.io/secret-type: repo-creds
          stringData:
            url: https://github.com/<org>/<resource-repository>
            password: ${{ secrets.GITHUB_TOKEN }}
            username: not-used
          EOF

      - name: Generate Diff
        run: |
          docker run \
            --network=host \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v $(pwd)/main:/base-branch \
            -v $(pwd)/pull-request:/target-branch \
            -v $(pwd)/output:/output \
            -v $(pwd)/secrets:/secrets \                                         ⬅️ Mount the secrets folder
            -e TARGET_BRANCH=refs/pull/${{ github.event.number }}/merge \        ⬅️ The PR branch on the Resource Repository
            -e REPO=<org>/<resource-repository> \                                ⬅️ Resource Repository (where the PR is!)
            dagandersen/argocd-diff-preview:v0.1.20

      - name: Post diff as comment
        run: |
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md --edit-last || \
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

!!! warning "Cross-repository access"
    The Resource Repository pipeline needs a Personal Access Token (PAT) or GitHub App token with access to the Application Repository. The default `GITHUB_TOKEN` only has access to the current repository.

#### Local equivalent

```bash hl_lines="1 2 15"
git clone https://github.com/<org>/<application-repository> base-branch --depth 1 -q -b main
git clone https://github.com/<org>/<application-repository> target-branch --depth 1 -q -b main

mkdir secrets
# store some secrets in the ./secrets folder

docker run \
    --network host \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v $(pwd)/output:/output \
    -v $(pwd)/base-branch:/base-branch \
    -v $(pwd)/target-branch:/target-branch \
    -v $(pwd)/secrets:/secrets \
    -e TARGET_BRANCH=pr-branch \
    -e REPO=<org>/<resource-repository> \
    dagandersen/argocd-diff-preview:v0.1.20
```

---

## Summary

| Pipeline location | Clone which repo? | `--repo` flag |
|-------------------|-------------------|---------------|
| Application Repository | Application Repository | Application Repository |
| Resource Repository | Application Repository | Resource Repository |

With pipelines in both repositories, you'll get diffs for changes to either your Applications or the resources they manage.

---

!!! tip "Questions?"
    If you have questions or suggestions, please [open an issue](https://github.com/dag-andersen/argocd-diff-preview/issues)!
