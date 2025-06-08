## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Modified (1):
± my-app-watch-pattern-valid-regex
```

<details>
<summary>my-app-watch-pattern-valid-regex (examples/helm/applications/watch-pattern/valid-regex.yaml)</summary>
<br>

```diff
@@ Application modified: my-app-watch-pattern-valid-regex (examples/helm/applications/watch-pattern/valid-regex.yaml) @@
 apiVersion: v1
 kind: Service
 metadata:
   annotations: {}
   labels:
     app.kubernetes.io/instance: my-app-watch-pattern-valid-regex
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
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
   type: ClusterIP
 
 ---
 apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   annotations: {}
   labels:
     app.kubernetes.io/instance: my-app-watch-pattern-valid-regex
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: experiment
   namespace: default
 
 ---
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   annotations: {}
   labels:
     app.kubernetes.io/instance: my-app-watch-pattern-valid-regex
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
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
@@ skipped 15 lines (63 -> 77) @@
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

_Stats_:
[], [], [], [], []
