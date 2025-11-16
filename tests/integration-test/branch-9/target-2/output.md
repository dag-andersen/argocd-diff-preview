## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Deleted (1):
- nginx-ingress (-490)
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
-  labels:
-    app.kubernetes.io/component: controller
-    app.kubernetes.io/instance: nginx-ingress
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: ingress-nginx
-    app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes.io/version:
🚨 Diff is too long
```

</details>

⚠️⚠️⚠️ Diff exceeds max length of 900 characters. Truncating to fit. This can be adjusted with the `--max-diff-length` flag

_Stats_:
[], [], [], [], []
