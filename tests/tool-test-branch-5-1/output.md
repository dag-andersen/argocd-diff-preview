## Argo CD Diff Preview

Summary:
```bash
 {base => target}/my-app-watch-pattern-valid-regex | 8 ++++----
 1 file changed, 4 insertions(+), 4 deletions(-)
```

<details>
<summary>Diff:</summary>
<br>

```diff
diff --git base/my-app-watch-pattern-valid-regex target/my-app-watch-pattern-valid-regex
index 5024fcb..36a884c 100644
--- base/my-app-watch-pattern-valid-regex
+++ target/my-app-watch-pattern-valid-regex
@@ -2,21 +2,21 @@
 apiVersion: v1
 kind: Service
 metadata:
   labels:
     app.kubernetes.io/instance: my-app-watch-pattern-valid-regex
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     argocd.argoproj.io/instance: my-app-watch-pattern-valid-regex
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: experiment
   namespace: default
 spec:
   ports:
   - name: http
     port: 80
     protocol: TCP
     targetPort: http
   selector:
     app.kubernetes.io/instance: my-app-watch-pattern-valid-regex
     app.kubernetes.io/name: myApp
@@ -27,35 +27,35 @@ apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   labels:
     app.kubernetes.io/instance: my-app-watch-pattern-valid-regex
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     argocd.argoproj.io/instance: my-app-watch-pattern-valid-regex
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: experiment
   namespace: default
 
 ---
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   labels:
     app.kubernetes.io/instance: my-app-watch-pattern-valid-regex
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     argocd.argoproj.io/instance: my-app-watch-pattern-valid-regex
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: experiment
   namespace: default
 spec:
   replicas: 5
   selector:
     matchLabels:
       app.kubernetes.io/instance: my-app-watch-pattern-valid-regex
       app.kubernetes.io/name: myApp
   template:
     metadata:
       labels:
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
+      serviceAccountName: experiment
```

</details>
