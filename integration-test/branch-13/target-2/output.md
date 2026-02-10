## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Added (1):
+ label-selectors-example (+79)
```

<details>
<summary>label-selectors-example (examples/label-selectors/app.yaml)</summary>
<br>

@@ Application added: label-selectors-example (examples/label-selectors/app.yaml) @@
#### Deployment/super-app-name (some-namespace)
```diff
+apiVersion: apps/v1
+kind: Deployment
+metadata:
+  labels:
+    app.kubernetes.io/instance: label-selectors-example
+    app.kubernetes.io/managed-by: Helm
+    app.kubernetes.io/name: myApp
+    app.kubernetes.io/version: 1.16.0
+    helm.sh/chart: myApp-0.1.0
+  name: super-app-name
+  namespace: some-namespace
+spec:
+  replicas: 5
+  selector:
+    matchLabels:
+      app.kubernetes.io/instance: label-selectors-example
+      app.kubernetes.io/name: myApp
+  template:
+    metadata:
+      labels:
+        app.kubernetes.io/instance: label-selectors-example
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
#### Service/super-app-name (some-namespace)
```diff
+apiVersion: v1
+kind: Service
+metadata:
+  labels:
+    app.kubernetes.io/instance: label-selectors-example
+    app.kubernetes.io/managed-by: Helm
+    app.kubernetes.io/name: myApp
+    app.kubernetes.io/version: 1.16.0
+    helm.sh/chart: myApp-0.1.0
+  name: super-app-name
+  namespace: some-namespace
+spec:
+  ports:
+  - name: http
+    port: 80
+    protocol: TCP
+    targetPort: http
+  selector:
+    app.kubernetes.io/instance: label-selectors-example
+    app.kubernetes.io/name: myApp
+  type: ClusterIP
```
#### ServiceAccount/super-app-name (some-namespace)
```diff
+apiVersion: v1
+automountServiceAccountToken: true
+kind: ServiceAccount
+metadata:
+  labels:
+    app.kubernetes.io/instance: label-selectors-example
+    app.kubernetes.io/managed-by: Helm
+    app.kubernetes.io/name: myApp
+    app.kubernetes.io/version: 1.16.0
+    helm.sh/chart: myApp-0.1.0
+  name: super-app-name
+  namespace: some-namespace
```
</details>

_Skipped resources_: 
- Applications: `12` (base) -> `11` (target)
- ApplicationSets: `7` (base) -> `7` (target)

_Stats_:
[Applications: 1], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
