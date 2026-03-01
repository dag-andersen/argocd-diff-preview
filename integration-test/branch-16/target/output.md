## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Modified (1):
± my-app (+1|-1)
```

<details>
<summary>my-app (examples/helm-kustomize-postrender/application.yaml)</summary>
<br>

```diff
@@ Application modified: my-app (examples/helm-kustomize-postrender/application.yaml) @@
   template:
     metadata:
       labels:
         app: my-app
         foo: bar
     spec:
       containers:
       - image: nginx:1.25
         name: app
         ports:
-        - containerPort: 80
+        - containerPort: 8080
```

</details>

_Stats_:
[Applications: 2], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
