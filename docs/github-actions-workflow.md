# GitHub Actions Workflow

## Public repositories

If your repository is public and only uses public Helm charts, you can use the following GitHub Actions workflow to generate a diff between the main branch and the pull request branch. The diff will then be posted as a comment on the pull request.

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

      - name: Generate Diff
        run: |
          docker run \
            --network=host \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v $(pwd)/main:/base-branch \
            -v $(pwd)/pull-request:/target-branch \
            -v $(pwd)/output:/output \
            -e TARGET_BRANCH=${{ github.head_ref }} \
            -e REPO=${{ github.repository }} \
            dagandersen/argocd-diff-preview:v0.0.22

      - name: Post diff as comment
        run: |
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md --edit-last || \
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## Private repositories and Helm Charts

In the simple code examples above, we do not provide the cluster with any credentials, which only works if the image/Helm Chart registry and the Git repository are public. Since your repository might not be public you need to provide the tool with the necessary read-access credentials for the repository. This can be done by placing the Argo CD repo secrets in folder mounted at `/secrets`. When the tool starts, it will simply run `kubectl apply -f /secrets` to apply every resource to the cluster, before starting the rendering process.

```yaml title=".github/workflows/generate-diff.yml" linenums="19" hl_lines="7-22 32"
...
    - uses: actions/checkout@v4
      with:
        ref: main
        path: main

    - name: Prepare secrets
      run: |
        mkdir secrets
        cat > secrets/secret.yaml << "EOF"
        apiVersion: v1
        kind: Secret
        metadata:
          name: private-repo
          namespace: argocd
          labels:
            argocd.argoproj.io/secret-type: repo-creds
        stringData:
          url: https://github.com/${{ github.repository }}
          password: ${{ secrets.GITHUB_TOKEN }}  ⬅️ Short-lived GitHub Token
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
          -v $(pwd)/secrets:/secrets \           ⬅️ Mount the secrets folder
          -e TARGET_BRANCH=${{ github.head_ref }} \
          -e REPO=${{ github.repository }} \
          dagandersen/argocd-diff-preview:v0.0.22
```

For more info, see the [Argo CD docs](https://argo-cd.readthedocs.io/en/stable/operator-manual/argocd-repo-creds-yaml/)