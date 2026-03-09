## Argo CD Diff Preview

Summary:
```yaml
Deleted (1):
- folder2 (-19)

Modified (1):
± multi-source-app (+1|-1)
```

<details>
<summary>folder2 (examples/git-generator/app/app-set.yaml)</summary>
<br>

#### Deployment: deploy-from-folder-two
```diff
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

#### Deployment: default/backend
```diff
       app: backend
   template:
     metadata:
       labels:
         app: backend
     spec:
       containers:
       - image: my-org/backend:1.0.0
         name: backend
         ports:
-        - containerPort: 8080
+        - containerPort: 80
```
</details>

_Stats_:
[Applications: 39], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
