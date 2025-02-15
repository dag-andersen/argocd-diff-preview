## Argo CD Diff Preview

Summary:
```bash
 {base => target}/my-service-staging | 2 +-
 1 file changed, 1 insertion(+), 1 deletion(-)
```

<details>
<summary>Diff:</summary>
<br>

```diff
diff --git base/my-service-staging target/my-service-staging
index 7af0462..395b870 100644
--- base/my-service-staging
+++ target/my-service-staging
@@ -16,21 +16,21 @@ spec:
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
