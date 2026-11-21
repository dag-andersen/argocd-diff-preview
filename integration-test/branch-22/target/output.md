## Argo CD Diff Preview

Summary:
```yaml
Modified (1):
± template-patch-prod (+1|-1)
```

<details>
<summary>template-patch-prod (appset.yaml)</summary>
<br>

#### ConfigMap: default/template-patch-production
```diff
 apiVersion: v1
 data:
-  color: blue
+  color: purple
 kind: ConfigMap
 metadata:
   name: template-patch-production
   namespace: default
```
</details>

_Stats_:
[Applications: 4], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
