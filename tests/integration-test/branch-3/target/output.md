## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Modified (1):
± my-service-staging ((1 lines added, 1 lines deleted))
```

<details>
<summary>my-service-staging (examples/kustomize/applications/my-service-staging.yaml)</summary>
<br>

```diff
@@ Application modified: my-service-staging (examples/kustomize/applications/my-service-staging.yaml) @@
 ---
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   annotations: {}
   labels:
     app: myapp
   name: staging-myapp
   namespace: default
 spec:
-  replicas: 2
+  replicas: 6
   selector:
     matchLabels:
       app: myapp
   template:
     metadata:
       labels:
         app: myapp
     spec:
       containers:
       - image: dag-andersen/myapp:latest
```

</details>

_Stats_:
[], [], [], [], []
