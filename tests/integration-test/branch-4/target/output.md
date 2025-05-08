## integration-test/branch-4

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
@@ Application added: folder3 (examples/git-generator/app/app-set.yaml) @@
+---
+apiVersion: apps/v1
+kind: Deployment
+metadata:
+  annotations:
+    argocd.argoproj.io/tracking-id: folder3:apps/Deployment:/deploy-from-folder-three
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

Rendered x Applications in x
