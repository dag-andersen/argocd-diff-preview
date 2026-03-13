# App-of-Apps Example

This example demonstrates the **app-of-apps pattern** with a 3-level chain of 5 Argo CD Applications.

## Structure

```
root-app  (registered in Argo CD)
│   source: apps/root/
│   renders → 2 child Application YAMLs
│
├── level-1a-app
│   │   source: apps/level-1a/
│   │   renders → 2 child Application YAMLs
│   │
│   ├── level-2a-app  (leaf)
│   │       source: apps/level-2a/
│   │       renders → ConfigMap: level-2a-config
│   │
│   └── level-2b-app  (leaf)
│           source: apps/level-2b/
│           renders → ConfigMap: level-2b-config
│
└── level-1b-app  (leaf)
        source: apps/level-1b/
        renders → ConfigMap: level-1b-config
```

**Total: 5 applications across 3 levels.**

Only `root-app.yaml` needs to be registered with Argo CD.  
The child Application resources are discovered automatically by `argocd-diff-preview` when using `--render-method=repo-server-api`.

## Usage

```bash
argocd-diff-preview \
  --render-method=repo-server-api \
  --repo https://github.com/dag-andersen/argocd-diff-preview \
  --file-regex "examples/app-of-apps/root-app.yaml"
```
