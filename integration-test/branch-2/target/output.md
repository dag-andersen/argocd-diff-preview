## Argo CD Diff Preview

Summary:
```yaml
Modified (1):
± internal-chart-example (+5|-5)
```

<details>
<summary>internal-chart-example (examples/internal-chart/app.yaml)</summary>
<br>

#### Deployment: super-app-name → new-app-name (default)
```diff
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   labels:
     app.kubernetes.io/instance: internal-chart-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: new-app-name
   namespace: default
 spec:
-  replicas: 1
+  replicas: 5
   selector:
     matchLabels:
       app.kubernetes.io/instance: internal-chart-example
       app.kubernetes.io/name: myApp
   template:
     metadata:
       labels:
         app.kubernetes.io/instance: internal-chart-example
         app.kubernetes.io/managed-by: Helm
         app.kubernetes.io/name: myApp
@@ skipped 12 lines (25 -> 36) @@
         - containerPort: 80
           name: http
           protocol: TCP
         readinessProbe:
           httpGet:
             path: /
             port: http
         resources: {}
         securityContext: {}
       securityContext: {}
-      serviceAccountName: super-app-name
+      serviceAccountName: new-app-name
```
#### Service: super-app-name → new-app-name (default)
```diff
 apiVersion: v1
 kind: Service
 metadata:
   labels:
     app.kubernetes.io/instance: internal-chart-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: new-app-name
   namespace: default
 spec:
   ports:
   - name: http
     port: 80
     protocol: TCP
     targetPort: http
   selector:
     app.kubernetes.io/instance: internal-chart-example
     app.kubernetes.io/name: myApp
```
#### ServiceAccount: super-app-name → new-app-name (default)
```diff
 apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   labels:
     app.kubernetes.io/instance: internal-chart-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: new-app-name
   namespace: default
```
</details>

_Stats_:
[Applications: 36], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
