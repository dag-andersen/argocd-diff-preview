## Argo CD Diff Preview

Summary:
```yaml
Total: 2 files changed

Modified (2):
± valid-manifest-generate-paths-example
± watch-pattern-valid-regex-example
```

<details>
<summary>valid-manifest-generate-paths-example (examples/manifest-generate-paths/valid-annotation.yaml)</summary>
<br>

```diff
@@ Application modified: valid-manifest-generate-paths-example (examples/manifest-generate-paths/valid-annotation.yaml) @@
 ---
 apiVersion: v1
 kind: Service
 metadata:
   annotations:
-    argocd.argoproj.io/tracking-id: valid-manifest-generate-paths-example:/Service:default/super-app-name
+    argocd.argoproj.io/tracking-id: valid-manifest-generate-paths-example:/Service:default/super-duper-app-name
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
 
 ---
 apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   annotations:
-    argocd.argoproj.io/tracking-id: valid-manifest-generate-paths-example:/ServiceAccount:default/super-app-name
+    argocd.argoproj.io/tracking-id: valid-manifest-generate-paths-example:/ServiceAccount:default/super-duper-app-name
   labels:
     app.kubernetes.io/instance: valid-manifest-generate-paths-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: super-duper-app-name
   namespace: default
 
 ---
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   annotations:
-    argocd.argoproj.io/tracking-id: valid-manifest-generate-paths-example:apps/Deployment:default/super-app-name
+    argocd.argoproj.io/tracking-id: valid-manifest-generate-paths-example:apps/Deployment:default/super-duper-app-name
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
@@ skipped 15 lines (70 -> 84) @@
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

</details>

<details>
<summary>watch-pattern-valid-regex-example (examples/watch-pattern/valid-regex.yaml)</summary>
<br>

```diff
@@ Application modified: watch-pattern-valid-regex-example (examples/watch-pattern/valid-regex.yaml) @@
 ---
 apiVersion: v1
 kind: Service
 metadata:
   annotations:
-    argocd.argoproj.io/tracking-id: watch-pattern-valid-regex-example:/Service:default/super-app-name
+    argocd.argoproj.io/tracking-id: watch-pattern-valid-regex-example:/Service:default/super-duper-app-name
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
 
 ---
 apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   annotations:
-    argocd.argoproj.io/tracking-id: watch-pattern-valid-regex-example:/ServiceAccount:default/super-app-name
+    argocd.argoproj.io/tracking-id: watch-pattern-valid-regex-example:/ServiceAccount:default/super-duper-app-name
   labels:
     app.kubernetes.io/instance: watch-pattern-valid-regex-example
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: super-duper-app-name
   namespace: default
 
 ---
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   annotations:
-    argocd.argoproj.io/tracking-id: watch-pattern-valid-regex-example:apps/Deployment:default/super-app-name
+    argocd.argoproj.io/tracking-id: watch-pattern-valid-regex-example:apps/Deployment:default/super-duper-app-name
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
@@ skipped 15 lines (70 -> 84) @@
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

</details>

Rendered x Applications in x
