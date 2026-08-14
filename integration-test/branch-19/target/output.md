## Argo CD Diff Preview

Summary:
```yaml
Modified (1):
± kustomize-build-options (+2|-2)
```

<details>
<summary>kustomize-build-options (examples/kustomize-build-options/application.yaml)</summary>
<br>

#### Deployment: default/kustomize-build-options
```diff
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   name: kustomize-build-options
   namespace: default
 spec:
-  replicas: 1
+  replicas: 3
   selector:
     matchLabels:
       app: kustomize-build-options
   template:
     metadata:
       labels:
         app: kustomize-build-options
     spec:
       containers:
-      - image: nginx:1.25
+      - image: nginx:1.27
         name: app
```
</details>

_Stats_:
[Applications: 2], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
