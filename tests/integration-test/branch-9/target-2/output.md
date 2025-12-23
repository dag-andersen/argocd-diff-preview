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
-    app.kubernetes.io/managed-by: H
🚨 Diff is too long
```

</details>

⚠️⚠️⚠️ Diff exceeds max length of 900 characters. Truncating to fit. This can be adjusted with the `--max-diff-length` flag

_Skipped resources_: 
- Applications: `10` (base) -> `9` (target)
- ApplicationSets: `6` (base) -> `3` (target)


_Stats_:
[Applications: 1], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
