## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Modified (1):
± my-app
```

<details>
<summary>Diff:</summary>
<br>

```diff
@@ Application modified: my-app @@
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
   type: ClusterIP
 
 ---
 apiVersion: v1
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
@@ skipped 12 lines (68 -> 79) @@
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
