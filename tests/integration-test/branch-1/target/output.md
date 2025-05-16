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
 
@@ skipped 8 lines (19 -> 26) @@
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
@@ skipped 28 lines (37 -> 64) @@
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
@@ skipped 20 lines (75 -> 94) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
+    app.kubernetes.io/version: 1.11.1
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
   namespace: default
 
@@ skipped 8 lines (105 -> 112) @@
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
@@ skipped 29 lines (123 -> 151) @@
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
@@ skipped 13 lines (162 -> 174) @@
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
@@ skipped 98 lines (185 -> 282) @@
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
@@ skipped 8 lines (293 -> 300) @@
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
@@ skipped 77 lines (311 -> 387) @@
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
@@ skipped 15 lines (398 -> 412) @@
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
@@ skipped 86 lines (423 -> 508) @@
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

Rendered x Applications in x
