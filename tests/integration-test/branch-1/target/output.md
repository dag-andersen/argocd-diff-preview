## Argo CD Diff Preview

Summary:
```yaml
Total: 2 files changed

Deleted (1):
- folder2

Modified (1):
± nginx-ingress
```

<details>
<summary>folder2 (examples/git-generator/app/app-set.yaml)</summary>
<br>

```diff
@@ Application deleted: folder2 (examples/git-generator/app/app-set.yaml) @@
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  annotations: {}
-  name: deploy-from-folder-two
-spec:
-  replicas: 2
-  selector:
-    matchLabels:
-      app: myapp
-  template:
-    metadata:
-      labels:
-        app: myapp
-    spec:
-      containers:
-      - image: dag-andersen/myapp:latest
-        name: myapp
-        ports:
-        - containerPort: 80
```

</details>

<details>
<summary>nginx-ingress (examples/helm/applications/nginx.yaml)</summary>
<br>

```diff
@@ Application modified: nginx-ingress (examples/helm/applications/nginx.yaml) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx-controller
   namespace: default
 
@@ skipped 9 lines (20 -> 28) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx-controller
   namespace: default
 spec:
@@ skipped 29 lines (39 -> 67) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx-controller-admission
   namespace: default
 spec:
@@ skipped 21 lines (78 -> 98) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
   namespace: default
 
@@ skipped 9 lines (109 -> 117) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx-admission
 webhooks:
 - admissionReviewVersions:
@@ skipped 30 lines (128 -> 157) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx-controller
   namespace: default
 spec:
@@ skipped 13 lines (168 -> 180) @@
         app.kubernetes.io/managed-by: Helm
         app.kubernetes.io/name: ingress-nginx
         app.kubernetes.io/part-of: ingress-nginx
-        app.kubernetes.io/version: 1.10.0
-        helm.sh/chart: ingress-nginx-4.10.0
+        app.kubernetes.io/version: 1.11.1
+        helm.sh/chart: ingress-nginx-4.11.1
     spec:
       containers:
       - args:
@@ skipped 99 lines (191 -> 289) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx
 spec:
   controller: k8s.io/ingress-nginx
@@ skipped 9 lines (300 -> 308) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
 rules:
 - apiGroups:
@@ skipped 78 lines (319 -> 396) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
 roleRef:
   apiGroup: rbac.authorization.k8s.io
@@ skipped 16 lines (407 -> 422) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
   namespace: default
 rules:
@@ skipped 87 lines (433 -> 519) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
   namespace: default
 roleRef:
```

</details>

_Stats_:
[], [], [], [], []
