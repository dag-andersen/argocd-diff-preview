# Self-hosted Github Actions Runners on kubernetes

## Running the arc-runners inside the same cluster as Argo CD

One of the benefits of using self-hosted runners (compared to GitHub-hosted runners) is that you can re-use Argo CD’s access credentials from the host cluster.

In other words, if your arc-runner pod runs inside the same cluster as Argo CD, you can run `kubectl get secrets -n argocd` from inside your pipeline and access the secrets from the host cluster, and then use them form `argocd-diff-preview`.

*Example:*

This example is meant for inspiration. You can structure it in many different ways.

```yaml title=".github/workflows/generate-diff.yml" linenums="1" hl_lines="11 30-68 77"
name: Diff Preview

on:
  pull_request:
    branches:
    - "main"

jobs:
  diff-preview-prod:
    name: Diff Preview
    runs-on: your-arc-runner
    permissions:
      contents: read
      pull-requests: write

    steps:
      - uses: actions/checkout@v4
        with:
          path: pull-request
          fetch-depth: 0

      - uses: actions/checkout@v4
        with:
          ref: main
          path: main

      - uses: azure/setup-kubectl@v4
        id: install

      # Get secret from the host cluster and apply it to the ephemeral local cluster (for the diff preview) ⬇️⬇️⬇️⬇️⬇️⬇️⬇️⬇️⬇️
      - name: Get secrets
        run: |
          kubectl config view || true
          kubectl get secrets -n argocd || true

          mkdir -p secrets

          # Get the secrets from the host cluster
          kubectl get secrets -n argocd -o json -l argocd.argoproj.io/secret-type > argocd-secrets.json

          # Clean up the secrets
          jq '{
            apiVersion: "v1",
            kind: "List",
            items: [
              .items[] 
              | del(
                  .metadata.annotations,
                  .metadata.creationTimestamp,
                  .metadata.ownerReferences,
                  .metadata.resourceVersion,
                  .metadata.selfLink,
                  .metadata.uid,
                  .metadata.managedFields
                ) 
              | .metadata.namespace = "argocd"
            ]
          }' argocd-secrets.json > processed-secrets.json

          # Split into individual files
          counter=1
          jq -c '.items[]' processed-secrets.json | while IFS= read -r line; do
            if [ -n "$line" ]; then
              echo "$line" | jq '.' > "secrets/manifest-$(printf "%03d" $counter).json"
              counter=$((counter + 1))
            fi
          done

          # Clean up temporary files
          rm -f argocd-secrets.json processed-secrets.json

      - name: Generate Diff
        run: |
          docker run \
            --network=host \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v $(pwd)/main:/base-branch \
            -v $(pwd)/pull-request:/target-branch \
            -v $(pwd)/secrets:/secrets \
            -v $(pwd)/output:/output \
            -e TARGET_BRANCH=refs/pull/${{ github.event.number }}/merge \
            -e REPO=${{ github.repository }} \
            dagandersen/argocd-diff-preview:v0.1.15

      - name: Post diff as comment
        run: |
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md --edit-last || \
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## CIDR collision

When using Action Runner Controller (ARC) to run self-host your GitHub Actions Runners. You need to ensure that the Service and Pod CIDRs of the kind cluster created by `argocd-diff-preview` don't overlap with your host cluster's CIDRs.

The default CIDRs are:

| Service | CIDR          |
| ------- | ------------- |
| Service | 10.96.0.0/16  |
| Pod     | 10.244.0.0/16 |

To configure kind:

1. Create a file in your repo, for instance `hack/kind.yaml`, with the following content:
```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  serviceSubnet: "10.80.0.0/16"
  podSubnet: "10.128.0.0/16"
```
2. Add the flag `--kind-options '--config /base-branch/hack/kind.yaml'` to `argocd-diff-preview`.
```yaml
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
            dagandersen/argocd-diff-preview:v0.1.15 \
            --kind-options '--config /base-branch/hack/kind.yaml'
```
