## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Modified (1):
± my-app-labels (+4|-4)
```

<details>
<summary>my-app-labels (examples/helm/applications/label-selectors/my-app-labels.yaml)</summary>
<br>

@@ Application modified: my-app-labels (examples/helm/applications/label-selectors/my-app-labels.yaml) @@
#### Deployment/experiment (default)
```diff
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   labels:
     app.kubernetes.io/instance: my-app-labels
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
       app.kubernetes.io/instance: my-app-labels
       app.kubernetes.io/name: myApp
   template:
     metadata:
       labels:
@@ skipped 15 lines (21 -> 35) @@
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
#### Service/experiment (default)
```diff
 apiVersion: v1
 kind: Service
 metadata:
   labels:
     app.kubernetes.io/instance: my-app-labels
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
     app.kubernetes.io/instance: my-app-labels
     app.kubernetes.io/name: myApp
   type: ClusterIP
```
#### ServiceAccount/experiment (default)
```diff
 apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   labels:
     app.kubernetes.io/instance: my-app-labels
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
-  name: super-app-name
+  name: experiment
   namespace: default
```
</details>

_Stats_:
[Applications: 22], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
