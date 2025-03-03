## Argo CD Diff Preview

Summary:
```bash
 {base => target}/nginx-ingress | 50 +++++++++++++++++++++---------------------
 1 file changed, 25 insertions(+), 25 deletions(-)
```

<details>
<summary>Diff:</summary>
<br>

```diff
diff --git base/nginx-ingress target/nginx-ingress
index c0f21e4..3b9b73c 100644
--- base/nginx-ingress
+++ target/nginx-ingress
@@ -3,40 +3,40 @@ apiVersion: v1
 data:
   allow-snippet-annotations: "false"
 kind: ConfigMap
 metadata:
   labels:
     app.kubernetes.io/component: controller
     app.kubernetes.io/instance: nginx-ingress
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
 
 ---
 apiVersion: v1
 kind: Service
 metadata:
   annotations: null
   labels:
     app.kubernetes.io/component: controller
     app.kubernetes.io/instance: nginx-ingress
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
   ipFamilies:
   - IPv4
   ipFamilyPolicy: SingleStack
   ports:
   - appProtocol: http
     name: http
     port: 80
@@ -56,23 +56,23 @@ spec:
 ---
 apiVersion: v1
 kind: Service
 metadata:
   labels:
     app.kubernetes.io/component: controller
     app.kubernetes.io/instance: nginx-ingress
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
   ports:
   - appProtocol: https
     name: https-webhook
     port: 443
     targetPort: webhook
   selector:
     app.kubernetes.io/component: controller
@@ -84,40 +84,40 @@ spec:
 apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   labels:
     app.kubernetes.io/component: controller
     app.kubernetes.io/instance: nginx-ingress
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
 
 ---
 apiVersion: admissionregistration.k8s.io/v1
 kind: ValidatingWebhookConfiguration
 metadata:
   annotations: null
   labels:
     app.kubernetes.io/component: admission-webhook
     app.kubernetes.io/instance: nginx-ingress
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
   - v1
   clientConfig:
     service:
       name: nginx-ingress-ingress-nginx-controller-admission
       namespace: default
       path: /networking/v1/ingresses
   failurePolicy: Fail
@@ -138,44 +138,44 @@ webhooks:
 ---
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   labels:
     app.kubernetes.io/component: controller
     app.kubernetes.io/instance: nginx-ingress
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
   minReadySeconds: 0
   replicas: 1
   revisionHistoryLimit: 10
   selector:
     matchLabels:
       app.kubernetes.io/component: controller
       app.kubernetes.io/instance: nginx-ingress
       app.kubernetes.io/name: ingress-nginx
   template:
     metadata:
       labels:
         app.kubernetes.io/component: controller
         app.kubernetes.io/instance: nginx-ingress
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
         - /nginx-ingress-controller
         - --publish-service=$(POD_NAMESPACE)/nginx-ingress-ingress-nginx-controller
         - --election-id=nginx-ingress-ingress-nginx-leader
         - --controller-class=k8s.io/ingress-nginx
         - --ingress-class=test
         - --configmap=$(POD_NAMESPACE)/nginx-ingress-ingress-nginx-controller
         - --validating-webhook=:8443
@@ -186,21 +186,21 @@ spec:
         - name: POD_NAME
           valueFrom:
             fieldRef:
               fieldPath: metadata.name
         - name: POD_NAMESPACE
           valueFrom:
             fieldRef:
               fieldPath: metadata.namespace
         - name: LD_PRELOAD
           value: /usr/local/lib/libmimalloc.so
-        image: registry.k8s.io/ingress-nginx/controller:v1.10.0@sha256:42b3f0e5d0846876b1791cd3afeb5f1cbbe4259d6f35651dcc1b5c980925379c
+        image: registry.k8s.io/ingress-nginx/controller:v1.11.1@sha256:e6439a12b52076965928e83b7b56aae6731231677b01e81818bce7fa5c60161a
         imagePullPolicy: IfNotPresent
         lifecycle:
           preStop:
             exec:
               command:
               - /wait-shutdown
         livenessProbe:
           failureThreshold: 5
           httpGet:
             path: /healthz
@@ -264,39 +264,39 @@ spec:
 ---
 apiVersion: networking.k8s.io/v1
 kind: IngressClass
 metadata:
   labels:
     app.kubernetes.io/component: controller
     app.kubernetes.io/instance: nginx-ingress
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
 
 ---
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRole
 metadata:
   labels:
     app.kubernetes.io/instance: nginx-ingress
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
   - ""
   resources:
   - configmaps
   - endpoints
   - nodes
   - pods
   - secrets
@@ -365,46 +365,46 @@ rules:
 
 ---
 apiVersion: rbac.authorization.k8s.io/v1
 kind: ClusterRoleBinding
 metadata:
   labels:
     app.kubernetes.io/instance: nginx-ingress
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
   kind: ClusterRole
   name: nginx-ingress-ingress-nginx
 subjects:
 - kind: ServiceAccount
   name: nginx-ingress-ingress-nginx
   namespace: default
 
 ---
 apiVersion: rbac.authorization.k8s.io/v1
 kind: Role
 metadata:
   labels:
     app.kubernetes.io/component: controller
     app.kubernetes.io/instance: nginx-ingress
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
 - apiGroups:
   - ""
   resources:
   - namespaces
   verbs:
   - get
 - apiGroups:
@@ -482,23 +482,23 @@ rules:
 ---
 apiVersion: rbac.authorization.k8s.io/v1
 kind: RoleBinding
 metadata:
   labels:
     app.kubernetes.io/component: controller
     app.kubernetes.io/instance: nginx-ingress
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
   apiGroup: rbac.authorization.k8s.io
   kind: Role
   name: nginx-ingress-ingress-nginx
 subjects:
 - kind: ServiceAccount
   name: nginx-ingress-ingress-nginx
   namespace: default
```

</details>
