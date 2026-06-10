## Argo CD Diff Preview

Summary:
```yaml
Modified (1):
± out-of-chart-values (+2|-2)
```

<details>
<summary>out-of-chart-values (examples/out-of-chart-values/application.yaml)</summary>
<br>

#### Deployment: default/out-of-chart-values
```diff
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   name: out-of-chart-values
   namespace: default
 spec:
-  replicas: 2
+  replicas: 3
   selector:
     matchLabels:
       app: out-of-chart-values
   template:
     metadata:
       labels:
         app: out-of-chart-values
     spec:
       containers:
-      - image: nginx:1.25
+      - image: nginx:1.27
         name: app
         ports:
         - containerPort: 80
```
</details>

_Stats_:
[Applications: 2], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
