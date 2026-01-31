## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Modified (1):
± my-app (+3|-22)
```

<details>
<summary>my-app (live -> examples/helm/applications/my-app.yaml)</summary>
<br>

```diff
@@ Application modified: my-app (live -> examples/helm/applications/my-app.yaml) @@
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   labels:
     app.kubernetes.io/instance: my-app
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
   name: super-app-name
+  namespace: default
 spec:
   replicas: 1
   selector:
     matchLabels:
       app.kubernetes.io/instance: my-app
       app.kubernetes.io/name: myApp
   template:
     metadata:
       labels:
         app.kubernetes.io/instance: my-app
@@ skipped 17 lines (21 -> 37) @@
         readinessProbe:
           httpGet:
             path: /
             port: http
         resources: {}
         securityContext: {}
       securityContext: {}
       serviceAccountName: super-app-name
 ---
 apiVersion: v1
-kind: Pod
-metadata:
-  annotations:
-    helm.sh/hook: test
-  labels:
-    app.kubernetes.io/instance: my-app
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: myApp
-    app.kubernetes.io/version: 1.16.0
-    helm.sh/chart: myApp-0.1.0
-  name: super-app-name-test-connection
-spec:
-  containers:
-  - args:
-    - super-app-name:80
-    command:
-    - wget
-    image: busybox
-    name: wget
-  restartPolicy: Never
----
-apiVersion: v1
 kind: Service
 metadata:
   labels:
     app.kubernetes.io/instance: my-app
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
   name: super-app-name
+  namespace: default
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
     helm.sh/chart: myApp-0.1.0
   name: super-app-name
+  namespace: default
```

</details>

_Stats_:
[Applications: 2], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
