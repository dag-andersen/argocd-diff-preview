## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Deleted (1):
- nginx-ingress
```

<details>
<summary>nginx-ingress (examples/external-chart/nginx.yaml)</summary>
<br>

```diff
@@ Application deleted: nginx-ingress (examples/external-chart/nginx.yaml) @@
-apiVersion: v1
-data:
-  allow-snippet-annotations: "false"
-kind: ConfigMap
-metadata:
-  annotations: {}
-  labels:
-    app.kubernetes.io/component: controller
-    app.kubernetes.io/instance: nginx-ingress
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: ingress-nginx
-    app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version: 1.10.0
-    helm.sh/chart: ingress-nginx-4.10.0
-  name: nginx-ingress-ingress-nginx-controller
-  namespace: default
-
----
-apiVersio
```

⚠️⚠️⚠️ Diff is too long. Truncated to 1000 characters. This can be adjusted with the `--max-diff-length` flag

_Stats_:
[], [], [], [], []
