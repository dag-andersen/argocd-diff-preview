# Self-hosted Github Actions Runners on kubernetes

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
            dagandersen/argocd-diff-preview:v0.1.12 --kind-options '--config /base-branch/hack/kind.yaml'
```
