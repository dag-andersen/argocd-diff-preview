## Argo CD Diff Preview

Summary:
```yaml
Modified (1):
± order-change-example (+5|-4)
```

<details>
<summary>order-change-example (examples/order-change/app/app-set.yaml)</summary>
<br>

#### Deployment → StatefulSet: order-change-example-deploy-1 → order-change-example-sfs-1 (default)
```diff
 apiVersion: apps/v1
-kind: Deployment
+kind: StatefulSet
 metadata:
-  name: order-change-example-deploy-1
+  name: order-change-example-sfs-1
   namespace: default
 spec:
   replicas: 2
   selector:
     matchLabels:
-      app: example-deploy-1
+      app: example-sfs-1
+  serviceName: order-change-example-sfs-1
   template:
     metadata:
       labels:
-        app: example-deploy-1
+        app: example-sfs-1
     spec:
       containers:
       - image: dag-andersen/myapp:latest
         name: myapp
         ports:
         - containerPort: 80
```
</details>

_Stats_:
[Applications: 36], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
