---
name: Bug report
about: Create a report to help us improve
title: 'Bug | some title '
labels: ''
assignees: dag-andersen

---

### Describe the bug
...

### Expected behavior
...

### Standard output (with `--debug` flag)
```
...
🤖 Getting resources from branch (branch: some/branch)
❌ Failed to get manifests for application: xxx, error: ...
❌ Failed to process application: XXXX
❌ Failed to extract resources
💥 Deleting cluster...
💥 Cluster deleted successfully
❌ failed to get resources: failed to process applications
```

### Application causing problems (if applicable)
```yaml
apiVersion: argoproj.io/v1alpha1
kind: ApplicationSet
metadata:
  name: git-generator-example-appset
spec:
  goTemplate: true
  goTemplateOptions: ["missingkey=error"]
  generators:
    - git:
        repoURL: https://github.com/dag-andersen/argocd-diff-preview.git
        revision: HEAD
        directories:
          - path: examples/git-generator/resources/**
        values:
          name: "{{ index .path.segments 3 }}"
  template:
    ...
```


### Parameters
```
✨ Running with:
✨ - local-cluster-tool: kind
✨ - cluster-name: argocd-diff-preview
✨ - kind-options: --config ./kind-config/options.yaml
✨ - base-branch: integration-test/branch-3/base
✨ - target-branch: integration-test/branch-3/target
✨ - secrets-folder: ./secrets
✨ - output-folder: ./output
✨ - argocd-namespace: argocd
✨ - repo: dag-andersen/argocd-diff-preview
✨ - timeout: 90 seconds
  ...
```

### Your pipeline (if applicable)
```yaml
jobs:
  render-diff:
    ...
    steps:
      - uses: actions/checkout@v4
        with:
          path: pull-request

      - uses: actions/checkout@v4
        with:
          ref: main
          path: main

      - name: Generate Diff
        run: |
          docker run \
            --network=host \
            -v /var/run/docker.sock:/var/run/docker.sock \
            -v $(pwd)/main:/base-branch \
            -v $(pwd)/pull-request:/target-branch \
            -v $(pwd)/output:/output \
            -e TARGET_BRANCH=${{ github.head_ref }} \
            -e REPO=${{ github.repository }} \
            dagandersen/argocd-diff-preview:vX.X.X

      - name: Post diff as comment
        run: |
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md --edit-last || \
          gh pr comment ${{ github.event.number }} --repo ${{ github.repository }} --body-file output/diff.md
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

```
