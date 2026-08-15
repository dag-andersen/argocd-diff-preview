# Pre-installing CRDs

Some applications need certain Custom Resource Definitions (CRDs) to be installed before a proper render can happen. The ephemeral cluster created by `argocd-diff-preview` only contains the Kubernetes APIs provided by the cluster itself and the resources installed with Argo CD. It does not automatically contain the CRDs from your destination cluster.

Use the preinstall folder to apply CRDs and other cluster prerequisites before Argo CD is installed and Applications are rendered.

## Prepare the preinstall folder

Create a directory and place the required CRD manifests in it:

```bash
mkdir -p preinstall
cp external-secrets-crd.yaml preinstall/
```

Every file directly inside the directory is applied in filename order. Files may contain multiple YAML documents. Subdirectories are not traversed.

If resources depend on one another, use filename prefixes to control their order:

```text
preinstall/
├── 00-external-secrets-crd.yaml
├── 10-cert-manager-crds.yaml
└── 20-other-prerequisites.yaml
```

## Docker

Mount the directory at `/preinstall`:

```bash
docker run \
  --network=host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v $(pwd)/main:/base-branch \
  -v $(pwd)/pull-request:/target-branch \
  -v $(pwd)/preinstall:/preinstall \
  -v $(pwd)/output:/output \
  -e TARGET_BRANCH=refs/pull/123/merge \
  -e REPO=example/repository \
  dagandersen/argocd-diff-preview:latest
```

The default preinstall path inside the container is `/preinstall`, so no additional option is required.

## Standalone binary

The standalone binary reads manifests from `./preinstall` by default:

```bash
argocd-diff-preview \
  --repo example/repository \
  --base-branch main \
  --target-branch feature/my-change
```

Use `--preinstall-folder` to select a different directory:

```bash
argocd-diff-preview \
  --repo example/repository \
  --base-branch main \
  --target-branch feature/my-change \
  --preinstall-folder ./cluster-prerequisites
```

The equivalent environment variable is `PREINSTALL_FOLDER`.

## Why install CRDs?

Installing the CRDs makes their APIs available to rendering and Kubernetes discovery in the ephemeral cluster. This is useful when:

- A Helm chart checks `.Capabilities.APIVersions` before rendering custom resources.
- Custom resources use the same name in different namespaces.
- The tool needs to determine whether a custom resource is namespaced or cluster-scoped.
- Other resources must exist before Argo CD or an Application can be rendered correctly.

Without an installed CRD, an unknown custom resource is treated as namespaced. This safe default prevents namespaces from being removed and same-named resources from being silently combined. Pre-installing the CRD still provides the most accurate result because the tool can discover its actual scope.

!!! note
    Preinstall manifests are applied only to the ephemeral preview cluster. They are not synced as part of an Argo CD Application and do not modify the destination cluster.

Keep repository credentials and other Argo CD secrets in the separate `/secrets` folder. The preinstall folder is intended for CRDs and other cluster prerequisites.
