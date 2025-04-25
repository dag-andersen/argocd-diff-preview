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

### Application causing Problems
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
