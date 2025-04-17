## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Added (1):
+ folder3
```

<details>
<summary>folder3 (examples/git-generator/app/app-set.yaml)</summary>
<br>

```diff
@@ Application added: folder3 @@
+---
+apiVersion: apps/v1
+kind: Deployment
+metadata:
+  labels:
+    argocd.argoproj.io/instance: folder3
+  name: deploy-from-folder-three
+spec:
+  replicas: 2
+  selector:
+    matchLabels:
+      app: myapp
+  template:
+    metadata:
+      labels:
+        app: myapp
+    spec:
+      containers:
+      - image: dag-andersen/myapp:latest
+        name: myapp
+        ports:
+        - containerPort: 80
+
```

</details>
