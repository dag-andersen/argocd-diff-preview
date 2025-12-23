## Argo CD Diff Preview

Summary:
```yaml
Total: 1 files changed

Deleted (1):
- nginx-ingress (-480)
```

<details>
<summary>nginx-ingress (examples/external-chart/nginx.yaml)</summary>
<br>

```diff
@@ Application deleted: nginx-ingress (examples/external-chart/nginx.yaml) @@
-apiVersion: admissionregistration.k8s.io/v1
-kind: ValidatingWebhookConfiguration
-metadata:
-  labels:
-    app.kubernetes.io/component: admission-webhook
-    app.kubernetes.io/instance: nginx-ingress
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: ingress-nginx
-    app.kubernetes.io/part-of: ingress-nginx
-    app.kubernetes
🚨 Diff is too long
```

</details>

⚠️⚠️⚠️ Diff exceeds max length of 900 characters. Truncating to fit. This can be adjusted with the `--max-diff-length` flag

_Stats_:
[Applications: 1], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
