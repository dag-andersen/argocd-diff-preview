## Argo CD Diff Preview

Summary:
```yaml
Modified (1):
± ignore-array-order (+5|-5)
```

<details>
<summary>ignore-array-order (examples/ignore-array-order/application.yaml)</summary>
<br>

#### Deployment: default/ignore-array-order
```diff
   selector:
     matchLabels:
       app: ignore-array-order
   template:
     metadata:
       labels:
         app: ignore-array-order
     spec:
       containers:
       - args:
-        - --first
         - --second
+        - --first
         env:
+        - name: DB_PORT
+          value: "5433"
         - name: LOG_LEVEL
           value: info
         - name: DB_HOST
           value: postgres
-        - name: DB_PORT
-          value: "5432"
         image: nginx:1.27
         name: app
         volumeMounts:
         - mountPath: /etc/config
           name: config
         - mountPath: /var/cache
           name: cache
       volumes:
       - emptyDir: {}
-        name: config
-      - emptyDir: {}
         name: cache
+      - emptyDir: {}
+        name: config
```
</details>

_Stats_:
[Applications: 2], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
