## Argo CD Diff Preview

Summary:
```yaml
Total: 2 files changed

Modified (2):
± valid-manifest-generate-paths-example (+4|-4)
± watch-pattern-valid-regex-example (+4|-4)
```

<details>
<summary>valid-manifest-generate-paths-example (examples/manifest-generate-paths/valid-annotation.yaml)</summary>
<br>

@@ Application modified: valid-manifest-generate-paths-example (examples/manifest-generate-paths/valid-annotation.yaml) @@
#### Deployment/super-duper-app-name (default)
```diff
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   labels:
     app.kubernetes.io/instance: valid-manifest-generate-paths-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: super-duper-app-name
   namespace: default
 spec:
   replicas: 5
   selector:
     matchLabels:
       app.kubernetes.io/instance: valid-manifest-generate-paths-example
       app.kubernetes.io/name: myApp
   template:
     metadata:
       labels:
@@ skipped 15 lines (21 -> 35) @@
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
+      serviceAccountName: super-duper-app-name
```
#### Service/super-duper-app-name (default)
```diff
 apiVersion: v1
 kind: Service
 metadata:
   labels:
     app.kubernetes.io/instance: valid-manifest-generate-paths-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: super-duper-app-name
   namespace: default
 spec:
   ports:
   - name: http
     port: 80
     protocol: TCP
     targetPort: http
   selector:
     app.kubernetes.io/instance: valid-manifest-generate-paths-example
     app.kubernetes.io/name: myApp
   type: ClusterIP
```
#### ServiceAccount/super-duper-app-name (default)
```diff
 apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   labels:
     app.kubernetes.io/instance: valid-manifest-generate-paths-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: super-duper-app-name
   namespace: default
```
</details>

<details>
<summary>watch-pattern-valid-regex-example (examples/watch-pattern/valid-regex.yaml)</summary>
<br>

@@ Application modified: watch-pattern-valid-regex-example (examples/watch-pattern/valid-regex.yaml) @@
#### Deployment/super-duper-app-name (default)
```diff
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   labels:
     app.kubernetes.io/instance: watch-pattern-valid-regex-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: super-duper-app-name
   namespace: default
 spec:
   replicas: 5
   selector:
     matchLabels:
       app.kubernetes.io/instance: watch-pattern-valid-regex-example
       app.kubernetes.io/name: myApp
   template:
     metadata:
       labels:
@@ skipped 15 lines (21 -> 35) @@
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
+      serviceAccountName: super-duper-app-name
```
#### Service/super-duper-app-name (default)
```diff
 apiVersion: v1
 kind: Service
 metadata:
   labels:
     app.kubernetes.io/instance: watch-pattern-valid-regex-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: super-duper-app-name
   namespace: default
 spec:
   ports:
   - name: http
     port: 80
     protocol: TCP
     targetPort: http
   selector:
     app.kubernetes.io/instance: watch-pattern-valid-regex-example
     app.kubernetes.io/name: myApp
   type: ClusterIP
```
#### ServiceAccount/super-duper-app-name (default)
```diff
 apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   labels:
     app.kubernetes.io/instance: watch-pattern-valid-regex-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: super-duper-app-name
   namespace: default
```
</details>

_Stats_:
[Applications: 4], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
