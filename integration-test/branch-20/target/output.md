## Argo CD Diff Preview

Summary:
```yaml
Modified (1):
± kind-level-capabilities (+12|-1)
```

<details>
<summary>kind-level-capabilities (examples/kind-level-capabilities/application.yaml)</summary>
<br>

#### ServiceMonitor: default/kind-level-capabilities
```diff
+apiVersion: monitoring.coreos.com/v1
+kind: ServiceMonitor
+metadata:
+  name: kind-level-capabilities
+  namespace: default
+spec:
+  endpoints:
+  - port: metrics
+  selector:
+    matchLabels:
+      app.kubernetes.io/name: kind-level-capabilities
```
#### ConfigMap: default/kind-level-capabilities
```diff
 apiVersion: v1
 data:
-  serviceMonitorEnabled: "false"
+  serviceMonitorEnabled: "true"
 kind: ConfigMap
 metadata:
   name: kind-level-capabilities
   namespace: default
```
</details>

_Stats_:
[Applications: 2], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
