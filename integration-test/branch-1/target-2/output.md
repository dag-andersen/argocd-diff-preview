## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Deleted (1):
- folder2 (-19)
```

<details>
<summary>folder2 (examples/git-generator/app/app-set.yaml)</summary>
<br>

#### Deployment/deploy-from-folder-two
```diff
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  name: deploy-from-folder-two
-spec:
-  replicas: 2
-  selector:
-    matchLabels:
-      app: myapp
-  template:
-    metadata:
-      labels:
-        app: myapp
-    spec:
-      containers:
-      - image: dag-andersen/myapp:latest
-        name: myapp
-        ports:
-        - containerPort: 80
```
</details>

_Stats_:
[Applications: 25], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
