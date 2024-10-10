# Helm/Kustomize generated ArgoCD applications

`argocd-diff-preview` will only look for YAML files in the repository with `kind: Application` or `kind: ApplicationSet`. If your applications are generated from a Helm chart or Kustomize template, you will have to add a step in the pipeline that renders the chart/template.

*Helm and Kustomize examples:*

```yaml title=".github/workflows/generate-diff.yml" linenums="1"
jobs:
  build:
    ...
    steps:
      ...
    - uses: actions/checkout@v4
      with:
        path: pull-request

    - name: Generate with helm chart
      run: helm template pull-request/some/path/my-chart > pull-request/rendered-apps.yaml

    - name: Generate with kustomize
      run: kustomize build pull-request/some/path/my-kustomize > pull-request/rendered-apps.yaml
      
    - name: Generate Diff
      run: |
        docker run \
          --network=host \
          -v /var/run/docker.sock:/var/run/docker.sock \
          -v $(pwd)/main:/base-branch \
          ...
```
This will place the rendered manifests inside the `pull-request` folder, and the tool will pick them up.
