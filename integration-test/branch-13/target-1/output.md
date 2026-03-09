## Argo CD Diff Preview

Summary:
```yaml
Added (1):
+ ignore-annotation-example (+79)
```

<details>
<summary>ignore-annotation-example (examples/ignore-annotation/app.yaml)</summary>
<br>

#### Deployment: default/super-app-name
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
#### Service: default/super-app-name
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
#### ServiceAccount: default/super-app-name
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
- Applications: `5` (base) -> `6` (target)
- ApplicationSets: `2` (base) -> `1` (target)

_Stats_:
[Applications: 31], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
