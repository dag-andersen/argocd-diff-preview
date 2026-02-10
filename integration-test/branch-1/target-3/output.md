## Argo CD Diff Preview

Summary:
```yaml
Total: 2 files changed

Deleted (1):
- folder2

Modified (1):
± nginx-ingress (+1|-1)
```

<details>
<summary>folder2 [<a href="https://argocd.example.com/applications/folder2">link</a>] (examples/git-generator/app/app-set.yaml)</summary>
<br>

@@ Application deleted: folder2 (examples/git-generator/app/app-set.yaml) @@
```diff
Diff content omitted because '--hide-deleted-app-diff' is enabled.
```
</details>

<details>
<summary>nginx-ingress [<a href="https://argocd.example.com/applications/nginx-ingress">link</a>] (examples/helm/applications/nginx.yaml)</summary>
<br>

@@ Application modified: nginx-ingress (examples/helm/applications/nginx.yaml) @@
#### Deployment/nginx-ingress-ingress-nginx-controller (default)
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
[Applications: 25], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
