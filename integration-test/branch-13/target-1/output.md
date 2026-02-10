## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Added (1):
+ ignore-annotation-example (+79)
```

<details>
<summary>ignore-annotation-example (examples/ignore-annotation/app.yaml)</summary>
<br>

#### Deployment/super-app-name (default)
```diff
+apiVersion: apps/v1
+kind: Deployment
+metadata:
+  labels:
+    app.kubernetes.io/instance: ignore-annotation-example
+    app.kubernetes.io/managed-by: Helm
+    app.kubernetes.io/name: myApp
+    app.kubernetes.io/version: 1.16.0
+    helm.sh/chart: myApp-0.1.0
+  name: super-app-name
+  namespace: default
+spec:
+  replicas: 1
+  selector:
+    matchLabels:
+      app.kubernetes.io/instance: ignore-annotation-example
+      app.kubernetes.io/name: myApp
+  template:
+    metadata:
+      labels:
+        app.kubernetes.io/instance: ignore-annotation-example
+        app.kubernetes.io/managed-by: Helm
+        app.kubernetes.io/name: myApp
+        app.kubernetes.io/version: 1.16.0
+        helm.sh/chart: myApp-0.1.0
+    spec:
+      containers:
+      - image: nginx:1.16.0
+        imagePullPolicy: IfNotPresent
+        livenessProbe:
+          httpGet:
+            path: /
+            port: http
+        name: myApp
+        ports:
+        - containerPort: 80
+          name: http
+          protocol: TCP
+        readinessProbe:
+          httpGet:
+            path: /
+            port: http
+        resources: {}
+        securityContext: {}
+      securityContext: {}
+      serviceAccountName: super-app-name
```
#### Service/super-app-name (default)
```diff
+apiVersion: v1
+kind: Service
+metadata:
+  labels:
+    app.kubernetes.io/instance: ignore-annotation-example
+    app.kubernetes.io/managed-by: Helm
+    app.kubernetes.io/name: myApp
+    app.kubernetes.io/version: 1.16.0
+    helm.sh/chart: myApp-0.1.0
+  name: super-app-name
+  namespace: default
+spec:
+  ports:
+  - name: http
+    port: 80
+    protocol: TCP
+    targetPort: http
+  selector:
+    app.kubernetes.io/instance: ignore-annotation-example
+    app.kubernetes.io/name: myApp
+  type: ClusterIP
```
#### ServiceAccount/super-app-name (default)
```diff
+apiVersion: v1
+automountServiceAccountToken: true
+kind: ServiceAccount
+metadata:
+  labels:
+    app.kubernetes.io/instance: ignore-annotation-example
+    app.kubernetes.io/managed-by: Helm
+    app.kubernetes.io/name: myApp
+    app.kubernetes.io/version: 1.16.0
+    helm.sh/chart: myApp-0.1.0
+  name: super-app-name
+  namespace: default
```
</details>

_Skipped resources_: 
- Applications: `2` (base) -> `3` (target)
- ApplicationSets: `1` (base) -> `0` (target)

_Stats_:
[Applications: 41], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
