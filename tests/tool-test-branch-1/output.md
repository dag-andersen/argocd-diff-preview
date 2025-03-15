## Argo CD Diff Preview

Summary:
```bash
 {base => target}/nginx-ingress | 48 +++++++++++++++++++++---------------------
 1 file changed, 24 insertions(+), 24 deletions(-)
```

<details>
<summary>Diff:</summary>
<br>

```diff
diff --git base/nginx-ingress target/nginx-ingress
index c0f21e4..3b9b73c 100644
--- base/nginx-ingress
+++ target/nginx-ingress
@@ -10,9 +10,9 @@ metadata:
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
 
@@ -27,9 +27,9 @@ metadata:
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
@@ -63,9 +63,9 @@ metadata:
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
@@ -91,9 +91,9 @@ metadata:
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
 
@@ -108,9 +108,9 @@ metadata:
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
@@ -145,9 +145,9 @@ metadata:
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
@@ -167,8 +167,8 @@ spec:
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
@@ -271,9 +271,9 @@ metadata:
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
@@ -287,9 +287,9 @@ metadata:
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
@@ -372,9 +372,9 @@ metadata:
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
@@ -395,9 +395,9 @@ metadata:
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
@@ -489,9 +489,9 @@ metadata:
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
