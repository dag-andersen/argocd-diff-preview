# Custom Argo CD Installation

Argo CD is installed using a [Helm Chart](https://artifacthub.io/packages/helm/argo/argo-cd). You can specify the Chart version with the `--argocd-chart-version` option. It defaults to the latest version.

You can modify the Argo CD Helm Chart installation by providing the tool with a `values.yaml` file in the Argo CD config directory. Check out all the available values in the [Argo CD Helm Chart](https://artifacthub.io/packages/helm/argo/argo-cd).

When running through Docker, mount the file into `/argocd-config/values.yaml` inside the container. When running the standalone binary, place the file in `./argocd-config/values.yaml` or point `--argocd-config-dir` to another directory.

You can see the default configuration in the repository under [`argocd-config`](https://github.com/dag-andersen/argocd-diff-preview/tree/main/argocd-config).

*Example:*

Here we set `configs.cm."kustomize.buildOptions"` in the Chart.

=== "Docker"

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

=== "Binary"

    The binary reads Argo CD Helm values from `./argocd-config` by default. Create that directory before running the tool:

    ```yaml title=".github/workflows/generate-diff.yml" linenums="1"
    jobs:
      build:
        ...
        steps:
          ...
        - name: Set Argo CD Custom Values
          run: |
            mkdir -p argocd-config

            cat > argocd-config/values.yaml << "EOF"
            # set whatever helm values you want
            configs:
              cm:
                kustomize.buildOptions: --load-restrictor LoadRestrictionsNone --enable-helm
            EOF

        - name: Generate Diff
          run: |
            argocd-diff-preview \
              --repo <owner>/<repo> \
              --base-branch <base-branch> \
              --target-branch <target-branch>
    ```

    If you want to keep the Argo CD values file somewhere else, pass the directory with `--argocd-config-dir`:

    ```yaml title=".github/workflows/generate-diff.yml" linenums="1"
    jobs:
      build:
        ...
        steps:
          ...
        - name: Set Argo CD Custom Values
          run: |
            mkdir -p custom-argocd-config

            cat > custom-argocd-config/values.yaml << "EOF"
            # set whatever helm values you want
            configs:
              cm:
                kustomize.buildOptions: --load-restrictor LoadRestrictionsNone --enable-helm
            EOF

        - name: Generate Diff
          run: |
            argocd-diff-preview \
              --repo <owner>/<repo> \
              --base-branch <base-branch> \
              --target-branch <target-branch> \
              --argocd-config-dir ./custom-argocd-config
    ```

    The directory can contain `values.yaml` and `values-override.yaml`. If both files exist, the tool passes both files to Helm in that order, so `values-override.yaml` can override values from `values.yaml`.

---

# Argo CD Config Management Plugins (CMP)

You can install any [Argo CD Config Management Plugin](https://argo-cd.readthedocs.io/en/stable/operator-manual/config-management-plugins/) that is supported through the [Argo CD Helm Chart](https://artifacthub.io/packages/helm/argo/argo-cd).

*Example:*

This example installs the [ArgoCD Lovely plugin](https://github.com/crumbhole/argocd-lovely-plugin) using the `values.yaml` file.

=== "Docker"

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

=== "Binary"

    Use the same Helm values, but write them to the config directory that the binary can read:

    ```yaml title=".github/workflows/generate-diff.yml" linenums="1"
    jobs:
      build:
        ...
        steps:
          ...
        - name: Set Argo CD Custom Values
          run: |
            mkdir -p argocd-config

            cat > argocd-config/values.yaml << "EOF"
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
            argocd-diff-preview \
              --repo <owner>/<repo> \
              --base-branch <base-branch> \
              --target-branch <target-branch>
    ```

    If the config directory is not named `argocd-config` or is not in the current working directory, add `--argocd-config-dir <path>` to the command.

---

## CI fallback for KSOPS (stub plugin)

If you use [KSOPS](https://github.com/viaduct-ai/kustomize-sops) as a kustomize exec plugin to generate Secrets from SOPS-encrypted files, you may run into two issues when running argocd-diff-preview in CI:

1. **Kustomize build failures** - SOPS decryption keys aren't available in CI, so KSOPS generators fail.
2. **Application CRD schema validation failures** - SOPS-encrypted Application manifests contain a top-level `.sops` metadata block that isn't part of the ArgoCD Application CRD schema.

### Stub KSOPS plugin

Install a fake `ksops` binary that returns an empty `ResourceList`. Kustomize treats the generator as successful, and all other resources (Deployments, Services, ConfigMaps, etc.) render correctly. Since both branches get the same stub, secrets are consistently absent from the diff - only non-secret changes show up.

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

This workaround is only needed when the ArgoCD `Application` manifest itself is SOPS-encrypted (e.g. `my-app.sops.yaml` containing `kind: Application`). In that case, the top-level `.sops` metadata block added by SOPS isn't part of the Application CRD schema, so ArgoCD rejects it during validation.

If your SOPS-encrypted files are only used as data sources for KSOPS generators (Secrets, ConfigMaps, etc.) rather than Application manifests, you don't need this step - the stub plugin above is sufficient.

To fix this, strip the `.sops` block from encrypted Application files before running argocd-diff-preview:

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

---

## OCI Helm charts

The default chart repository is the classic HTTP Helm repository at `https://argoproj.github.io/argo-helm`.

You can also install Argo CD from an OCI Helm chart by setting `--argocd-chart-url` to an `oci://` reference, for example `oci://ghcr.io/argoproj/argo-helm/argo-cd`.

```bash
argocd-diff-preview \
  --argocd-chart-url=oci://ghcr.io/argoproj/argo-helm/argo-cd \
  --argocd-chart-version=8.6.1 \
  --repo=<owner>/<repo> \
  --target-branch=<branch>
```

!!! important "Questions, issues, or suggestions"
    If you experience issues or have any questions, please open an issue in the repository! 🚀
