# Working with the Cluster Generator

When using an `ApplicationSet` with the [Cluster Generator](https://argo-cd.readthedocs.io/en/stable/operator-manual/applicationset/Generators-Cluster/), Argo CD dynamically generates `Application` resources based on the clusters registered in Argo CD. 

`argocd-diff-preview` does not need access to your remote clusters; however, it needs the secrets representing them. To enable the tool to render Applications for your target clusters, you must provide dummy "Cluster Secrets". This allows the `ApplicationSet` controller to loop over these secrets and generate the Applications.

The tool will apply these secrets to the local cluster before the rendering process starts.

You do **not** need to provide valid connection credentials (like bearer tokens or TLS certs) for the tool to work, because `argocd-diff-preview` only *renders* the manifests locally; it never actually connects to the target clusters to deploy anything. Dummy server URLs and names are sufficient.

```yaml title="secrets/my-clusters.yaml" hl_lines="7 21"
apiVersion: v1
kind: Secret
metadata:
  name: cluster-staging
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: cluster # This label is required for Argo CD to recognize this secret as a cluster secret
    environment: staging # Can be used by the generator (e.g. {{.metadata.labels.environment}})
type: Opaque
stringData:
  name: staging-cluster
  server: https://10.0.0.1
  config: dummy-string
---
apiVersion: v1
kind: Secret
metadata:
  name: cluster-production
  namespace: argocd
  labels:
    argocd.argoproj.io/secret-type: cluster # This label is required for Argo CD to recognize this secret as a cluster secret
    environment: production
type: Opaque
stringData:
  name: production-cluster
  server: https://10.0.0.2
  config: dummy-string
```

This means that if we have an `ApplicationSet` that uses the Cluster Generator, Argo CD will correctly generate the Applications based on the dummy secrets:


```yaml title="appset.yaml"
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: guestbook
spec:
  goTemplate: true
  generators:
  - clusters: {} # Automatically targets all clusters registered with Argo CD
  template:
    metadata:
      name: '{{.name}}-guestbook'
    spec:
      project: default
      source:
        repoURL: https://github.com/argoproj/argo-cd.git
        targetRevision: HEAD
        path: guestbook/{{.metadata.labels.environment}}
      destination:
        server: '{{.server}}' # argocd-diff-preview will automatically redirect this to the local cluster: https://kubernetes.default.svc
        namespace: guestbook
```

---

## Mounting the Secrets in GitHub Actions

You need to mount the folder containing these secrets into the Docker container, similar to how repository credentials are provided.

When the tool starts, it will run `kubectl apply -f /secrets` to apply the cluster secrets to the `argocd` namespace. The `ApplicationSet` controller will then discover them and successfully generate the applications.

```yaml title=".github/workflows/generate-diff.yml" hl_lines="32"
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - name: Generate Diff
        run: |
          # 1. Create the secrets directory
          mkdir -p secrets
          
          # 2. Write the cluster secrets into the directory
          cat > secrets/my-clusters.yaml << "EOF"
          apiVersion: v1
          kind: Secret
          metadata:
            name: cluster-staging
            namespace: argocd
            labels:
              argocd.argoproj.io/secret-type: cluster
              environment: staging
          stringData:
            name: staging-cluster
            server: https://10.0.0.1
          EOF
          
          # 3. Mount the secrets directory when running the tool
          docker run \
            --network=host \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v $(pwd)/main:/base-branch \
            -v $(pwd)/pull-request:/target-branch \
            -v $(pwd)/output:/output \
            -v $(pwd)/secrets:/secrets \
            -e TARGET_BRANCH=refs/pull/${{ github.event.number }}/merge \
            -e REPO=${{ github.repository }} \
            dagandersen/argocd-diff-preview:v0.1.25
```
