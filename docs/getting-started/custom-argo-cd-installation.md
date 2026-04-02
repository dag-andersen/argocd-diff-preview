# Custom Argo CD Installation

Argo CD is installed using a [Helm Chart](https://artifacthub.io/packages/helm/argo/argo-cd). You can specify the Chart version with the `--argocd-chart-version` option. It defaults to the latest version.

You can modify the Argo CD Helm Chart installation by providing the tool with a `values.yaml` file and mounting it in the `argocd-config` folder within the container. Check out all the available values in the [Argo CD Helm Chart](https://artifacthub.io/packages/helm/argo/argo-cd).

*Example:*

Here we set `configs.cm."kustomize.buildOptions"` in the Chart.

```yaml title=".github/workflows/generate-diff.yml" linenums="1"
jobs:
  build:
    ...
    steps:
      ...
    - name: Set Argo CD Custom Values
      run: |
        cat > values.yaml << "EOF"
        # set whatever helm values you want
        configs:
          cm:
            kustomize.buildOptions: --load-restrictor LoadRestrictionsNone --enable-helm
        EOF

    - name: Generate Diff
      run: |
        docker run \
          --network=host \
          -v /var/run/docker.sock:/var/run/docker.sock \
          -v $(pwd)/main:/base-branch \
          -v $(pwd)/pull-request:/target-branch \
          -v $(pwd)/values.yaml:/argocd-config/values.yaml \   ⬅️ Mount values.yaml
          ...
```

---

# Argo CD Config Management Plugins (CMP)

You can install any [Argo CD Config Management Plugin](https://argo-cd.readthedocs.io/en/stable/operator-manual/config-management-plugins/) that is supported through the [Argo CD Helm Chart](https://artifacthub.io/packages/helm/argo/argo-cd).

*Example:*

This example installs the [ArgoCD Lovely plugin](https://github.com/crumbhole/argocd-lovely-plugin) using the `values.yaml` file.

```yaml title=".github/workflows/generate-diff.yml" linenums="1"
jobs:
  build:
    ...
    steps:
      ...
    - name: Set Argo CD Custom Values
      run: |
        cat > values.yaml << "EOF"
        repoServer:
          extraContainers:
          # ArgoCD Lovely plugin - https://github.com/crumbhole/argocd-lovely-plugin
            - name: lovely-plugin
              image: ghcr.io/crumbhole/lovely:1.1.1
              securityContext:  
                runAsNonRoot: true
                runAsUser: 999
              volumeMounts:
                  # Import the repo-server's plugin binary
                - mountPath: /var/run/argocd
                  name: var-files
                - mountPath: /home/argocd/cmp-server/plugins
                  name: plugins
                - mountPath: /tmp
                  name: lovely-tmp
          volumes:
            - emptyDir: {}
              name: lovely-tmp
        EOF

    - name: Generate Diff
      run: |
        docker run \
          --network=host \
          -v /var/run/docker.sock:/var/run/docker.sock \
          -v $(pwd)/main:/base-branch \
          -v $(pwd)/pull-request:/target-branch \
          -v $(pwd)/values.yaml:/argocd-config/values.yaml \   ⬅️ Mount values.yaml
          ...
```

---

## CI fallback for KSOPS (stub plugin)

If you use [KSOPS](https://github.com/viaduct-ai/kustomize-sops) as a kustomize exec plugin to generate Secrets from SOPS-encrypted files, you may run into two issues when running argocd-diff-preview in CI:

1. **Kustomize build failures** — SOPS decryption keys aren't available in CI, so KSOPS generators fail.
2. **Application CRD schema validation failures** — SOPS-encrypted Application manifests contain a top-level `.sops` metadata block that isn't part of the ArgoCD Application CRD schema.

### Stub KSOPS plugin

Install a fake `ksops` binary that returns an empty `ResourceList`. Kustomize treats the generator as successful, and all other resources (Deployments, Services, ConfigMaps, etc.) render correctly. Since both branches get the same stub, secrets are consistently absent from the diff — only non-secret changes show up.

Add this to the `values.yaml` mounted at `--argocd-config-dir`:

```yaml title="values.yaml"
configs:
  cm:
    kustomize.buildOptions: "--enable-alpha-plugins --enable-exec"

repoServer:
  volumes:
    - name: custom-tools
      emptyDir: {}
  volumeMounts:
    - mountPath: /usr/local/bin/ksops
      name: custom-tools
      subPath: ksops
  initContainers:
    - name: install-fake-ksops
      image: alpine:3.23
      command: ["/bin/sh", "-c"]
      args:
        - |
          cat > /custom-tools/ksops << 'SCRIPT'
          #!/bin/sh
          # Stub KSOPS: returns an empty resource list so kustomize builds
          # succeed without real SOPS decryption keys.
          echo '{"apiVersion":"config.kubernetes.io/v1","kind":"ResourceList","items":[]}'
          SCRIPT
          chmod +x /custom-tools/ksops
      volumeMounts:
        - mountPath: /custom-tools
          name: custom-tools
  env:
    - name: XDG_CONFIG_HOME
      value: /.config
```

### Stripping `.sops` metadata from encrypted manifests

If your repository contains SOPS-encrypted Application manifests (e.g. `*.sops.yaml` files), the top-level `.sops` metadata block will cause ArgoCD Application CRD schema validation to fail. To fix this, strip the `.sops` block from encrypted files before running argocd-diff-preview:

```bash
# Strip .sops metadata from encrypted manifests so the top-level field
# doesn't break Application CRD schema validation in argocd-diff-preview.
# The actual file content stays intact for kustomize/KSOPS generators.
find "$base_dir" "$target_dir" -name '*.sops.yaml' | while IFS= read -r f; do
  sed '/^sops:$/,$d' "$f" > "${f}.tmp" && mv "${f}.tmp" "$f"
done
```

Run this after extracting the branch archives and before invoking argocd-diff-preview.

!!! tip
    This approach can be adapted for other kustomize exec plugins that depend on external secrets or credentials not available in CI.

!!! important "Questions, issues, or suggestions"
    If you experience issues or have any questions, please open an issue in the repository! 🚀