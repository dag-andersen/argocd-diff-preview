## Argo CD Diff Preview

Summary:
```diff
Total: 1 files changed

Modified (1):
± nginx-ingress
```

<details>
<summary>Diff:</summary>
<br>

```diff
@@ Application modified: nginx-ingress @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx-controller
   namespace: default
 
@@ skipped 8 lines (20 -> 27) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx-controller
   namespace: default
 spec:
@@ skipped 27 lines (39 -> 65) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx-controller-admission
   namespace: default
 spec:
@@ skipped 19 lines (77 -> 95) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
   namespace: default
 
@@ skipped 8 lines (107 -> 114) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx-admission
 webhooks:
 - admissionReviewVersions:
@@ skipped 28 lines (126 -> 153) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx-controller
   namespace: default
 spec:
@@ skipped 13 lines (165 -> 177) @@
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
@@ skipped 97 lines (188 -> 284) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx
 spec:
   controller: k8s.io/ingress-nginx
@@ skipped 7 lines (296 -> 302) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
 rules:
 - apiGroups:
@@ skipped 76 lines (314 -> 389) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
 roleRef:
   apiGroup: rbac.authorization.k8s.io
@@ skipped 14 lines (401 -> 414) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
   namespace: default
 rules:
@@ skipped 85 lines (426 -> 510) @@
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: ingress-nginx
     app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
+    app.kubernetes.io/version: 1.11.1
     argocd.argoproj.io/instance: nginx-ingress
-    helm.sh/chart: ingress-nginx-4.10.0
+    helm.sh/chart: ingress-nginx-4.11.1
   name: nginx-ingress-ingress-nginx
   namespace: default
 roleRef:
```

</details>
