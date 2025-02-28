## Argo CD Diff Preview

Summary:
```bash
 {base => target}/my-app | 10 +++++-----
 1 file changed, 5 insertions(+), 5 deletions(-)
```

<details>
<summary>Diff:</summary>
<br>

```diff
diff --git base/my-app target/my-app
index eb9e290..7cf6187 100644
--- base/my-app
+++ target/my-app
@@ -2,21 +2,21 @@
 apiVersion: v1
 kind: Service
 metadata:
   labels:
     app.kubernetes.io/instance: my-app
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     argocd.argoproj.io/instance: my-app
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
     app.kubernetes.io/instance: my-app
     app.kubernetes.io/name: myApp
@@ -27,38 +27,38 @@ apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   labels:
     app.kubernetes.io/instance: my-app
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     argocd.argoproj.io/instance: my-app
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: new-app-name
   namespace: default
 
 ---
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   labels:
     app.kubernetes.io/instance: my-app
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     argocd.argoproj.io/instance: my-app
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: new-app-name
   namespace: default
 spec:
-  replicas: 1
+  replicas: 5
   selector:
     matchLabels:
       app.kubernetes.io/instance: my-app
       app.kubernetes.io/name: myApp
   template:
     metadata:
       labels:
         app.kubernetes.io/instance: my-app
         app.kubernetes.io/managed-by: Helm
         app.kubernetes.io/name: myApp
@@ -77,12 +77,12 @@ spec:
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

</details>
