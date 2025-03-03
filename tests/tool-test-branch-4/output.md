## Argo CD Diff Preview

Summary:
```bash
 /dev/null => target/folder3 | 23 +++++++++++++++++++++++
 1 file changed, 23 insertions(+)
```

<details>
<summary>Diff:</summary>
<br>

```diff
diff --git target/folder3 target/folder3
new file mode 100644
index 0000000..98a842f
--- /dev/null
+++ target/folder3
@@ -0,0 +1,23 @@
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
