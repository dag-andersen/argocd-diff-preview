## Argo CD Diff Preview

Summary:
```yaml
Deleted (1):
- folder2

Modified (2):
± multi-source-app (+2|-2)
± nginx-ingress (+1|-1)
```

<details>
<summary>folder2 [<a href="https://argocd.example.com/applications/folder2">link</a>] (examples/git-generator/app/app-set.yaml)</summary>
<br>

_Diff hidden because `--hide-deleted-app-diff` is enabled_

</details>

<details>
<summary>multi-source-app [<a href="https://argocd.example.com/applications/multi-source-app">link</a>] (examples/multi-source/app.yaml)</summary>
<br>

#### Deployment: backend (default)
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
#### Deployment: frontend (default)
```diff
   replicas: 2
   selector:
     matchLabels:
       app: frontend
   template:
     metadata:
       labels:
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
<summary>nginx-ingress [<a href="https://argocd.example.com/applications/nginx-ingress">link</a>] (examples/external-chart/nginx.yaml)</summary>
<br>

#### Deployment: nginx-ingress-ingress-nginx-controller (default)
```diff
         - name: POD_NAME
           valueFrom:
             fieldRef:
               fieldPath: metadata.name
         - name: POD_NAMESPACE
           valueFrom:
             fieldRef:
               fieldPath: metadata.namespace
         - name: LD_PRELOAD
           value: /usr/local/lib/libmimalloc.so
-        image: registry.k8s.io/ingress-nginx/controller:v1.10.0@sha256:42b3f0e5d0846876b1791cd3afeb5f1cbbe4259d6f35651dcc1b5c980925379c
+        image: registry.k8s.io/ingress-nginx/controller:v1.11.1@sha256:e6439a12b52076965928e83b7b56aae6731231677b01e81818bce7fa5c60161a
         imagePullPolicy: IfNotPresent
         lifecycle:
           preStop:
             exec:
               command:
               - /wait-shutdown
         livenessProbe:
           failureThreshold: 5
           httpGet:
             path: /healthz
```
</details>

_Stats_:
[Applications: 39], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
