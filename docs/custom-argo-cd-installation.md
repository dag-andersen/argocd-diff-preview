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

!!! important "Questions, issues, or suggestions"
    If you experience issues or have any questions, please open an issue in the repository! 🚀