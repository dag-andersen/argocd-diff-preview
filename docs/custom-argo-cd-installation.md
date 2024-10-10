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
    - name: Set ArgoCD Custom Values
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

# Argo CD Config Management Plugins (CMP)

You can install any [Argo CD Config Management Plugin](https://argo-cd.readthedocs.io/en/stable/operator-manual/config-management-plugins/) that is supported through the [Argo CD Helm Chart](https://artifacthub.io/packages/helm/argo/argo-cd). However, there is no guarantee that the plugin will work with the tool, as this depends on the plugin and its specific implementation

!!! important "Questions, issues, or suggestions"
    If you experience issues or have any questions, please open an issue in the repository! 🚀