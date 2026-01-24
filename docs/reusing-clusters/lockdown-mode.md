# Lockdown Mode (Namespace-Scoped Argo CD)

By default, Argo CD is installed with cluster-wide permissions, meaning it can read and manage resources across all namespaces. However, some organizations require a more restricted setup where Argo CD only has permissions within a single namespace. This is often referred to as "namespace-scoped" or "lockdown mode".

This page explains how to use `argocd-diff-preview` with a namespace-scoped Argo CD installation.

## Why Lockdown Mode?

When using a pre-installed Argo CD instance for diff previews, you may want to restrict its permissions for security reasons:

- **Prevent secret access**: A cluster-scoped Argo CD can read secrets from any namespace. With lockdown mode, Argo CD can only access resources in its own namespace.
- **Isolation**: The diff preview Argo CD instance is completely isolated from your production workloads.

## How It Works

In lockdown mode, `argocd-diff-preview` uses the Argo CD API directly to retrieve manifests, rather than relying on the application controller's sync status. This allows the tool to work even when Argo CD doesn't have permission to check the destination namespaces.

## Requirements

- Argo CD installed with `createClusterRoles: false` (namespace-scoped)
- The `--use-argocd-api="true"` flag enabled when running `argocd-diff-preview`

## Installing Namespace-Scoped Argo CD

Here's an example Helm values file for installing Argo CD with namespace-scoped permissions:

```yaml title="values.yaml" linenums="1" hl_lines="3"
nameOverride: argocd-diff-preview
namespaceOverride: "argocd-diff-preview"
createClusterRoles: false # The important part!
crds:
  install: false # Only install CRDs if you don't have them already installed
notifications:
  enabled: false
dex:
  enabled: false
applicationSet:
  replicas: 0
controller:
  roleRules:
    - apiGroups:
        - "*"
      resources:
        - "*"
      verbs:
        - get
        - list
        - watch
    - apiGroups:
        - argoproj.io
      resources:
        - applications
        - applications/status
      verbs:
        - get
        - list
        - watch
        - update
        - patch
```

Install with:

```bash
helm repo add argo https://argoproj.github.io/argo-helm
helm install argo-cd argo/argo-cd \
  --create-namespace \
  --namespace argocd-diff-preview \
  -f values.yaml
```

## Running `argocd-diff-preview` in Lockdown Mode

To use lockdown mode, add the `--use-argocd-api="true"` flag:

```bash title=".github/workflows/generate-diff.yml" linenums="1" hl_lines="13"
docker run \
  --network host \
  -v ~/.kube:/root/.kube \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/output:/output \
  -v $(pwd)/base-branch:/base-branch \
  -v $(pwd)/target-branch:/target-branch \
  -e TARGET_BRANCH=my-feature-branch \
  -e REPO=my-org/my-repo \
  dagandersen/argocd-diff-preview:v0.1.22 \
  --argocd-namespace=argocd-diff-preview \
  --create-cluster=false \
  --use-argocd-api="true"
```

## Limitations

- Applications will show as "Unknown" status in the Argo CD UI since the application controller cannot access the destination namespaces
- Some ApplicationSet generators that require cluster-wide access may not work

## Related Issues

- [#250 - Limit ArgoCD to a single namespace](https://github.com/dag-andersen/argocd-diff-preview/issues/250)
