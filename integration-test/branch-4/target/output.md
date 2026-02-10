## integration-test/branch-4

Summary:
```yaml
Total: 1 files changed

Added (1):
+ folder3 (+19)
```

<details>
<summary>folder3 (examples/git-generator/app/app-set.yaml)</summary>
<br>

```diff
@@ Application added: folder3 (examples/git-generator/app/app-set.yaml) @@
@@ Resource: Deployment/deploy-from-folder-three @@
+apiVersion: apps/v1
+kind: Deployment
+metadata:
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
```

</details>

_Stats_:
[Applications: 25], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
