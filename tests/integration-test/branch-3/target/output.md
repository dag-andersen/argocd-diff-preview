## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Modified (1):
± my-service-staging
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
   labels:
     app: myapp
     argocd.argoproj.io/instance: my-service-staging
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
