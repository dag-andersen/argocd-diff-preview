## Argo CD Diff Preview

Summary:
```yaml
Total: 3 files changed

Deleted (1):
- folder2 (-19)

Modified (2):
± multi-source-app (+2|-2)
± nginx-ingress (+1|-1)
```

<details>
<summary>folder2 (examples/git-generator/app/app-set.yaml)</summary>
<br>

```diff
@@ Application deleted: folder2 (examples/git-generator/app/app-set.yaml) @@
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  name: deploy-from-folder-two
-spec:
-  replicas: 2
-  selector:
-    matchLabels:
-      app: myapp
-  template:
-    metadata:
-      labels:
-        app: myapp
-    spec:
-      containers:
-      - image: dag-andersen/myapp:latest
-        name: myapp
-        ports:
-        - containerPort: 80
```

</details>

<details>
<summary>multi-source-app (examples/multi-source/app.yaml)</summary>
<br>

```diff
@@ Application modified: multi-source-app (examples/multi-source/app.yaml) @@
       - image: my-org/backend:1.0.0
         name: backend
         ports:
-        - containerPort: 8080
+        - containerPort: 80
 ---
 apiVersion: apps/v1
 kind: Deployment
@@ skipped 11 lines (24 -> 34) @@
         app: frontend
     spec:
       containers:
-      - image: nginx:1.25
+      - image: nginx:1.26
         name: frontend
         ports:
         - containerPort: 80
```

</details>

<details>
<summary>nginx-ingress (examples/external-chart/nginx.yaml)</summary>
<br>

```diff
@@ Application modified: nginx-ingress (examples/external-chart/nginx.yaml) @@
               fieldPath: metadata.namespace
         - name: LD_PRELOAD
           value: /usr/local/lib/libmimalloc.so
-        image: registry.k8s.io/ingress-nginx/controller:v1.10.0@sha256:42b3f0e5d0846876b1791cd3afeb5f1cbbe4259d6f35651dcc1b5c980925379c
+        image: registry.k8s.io/ingress-nginx/controller:v1.11.1@sha256:e6439a12b52076965928e83b7b56aae6731231677b01e81818bce7fa5c60161a
         imagePullPolicy: IfNotPresent
         lifecycle:
           preStop:
```

</details>

_Stats_:
[Applications: 47], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
