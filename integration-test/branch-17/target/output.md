## Argo CD Diff Preview

Summary:
```yaml
Added (1):
+ level-2c-app (+8)

Modified (3):
± level-1b-app (+1|-1)
± level-2a-app (+2|-1)
± level-2b-app (+1)
```

<details>
<summary>level-1b-app (examples/app-of-apps/root-app.yaml)</summary>
<br>

#### ConfigMap: default/level-1b-config
```diff
 apiVersion: v1
 data:
   app: level-1b
-  color: blue
+  color: purple
 kind: ConfigMap
 metadata:
   name: level-1b-config
   namespace: default
```
</details>

<details>
<summary>level-2a-app (examples/app-of-apps/root-app.yaml)</summary>
<br>

#### ConfigMap: default/level-2a-config
```diff
 apiVersion: v1
 data:
   app: level-2a
-  color: green
+  color: yellow
+  environment: production
 kind: ConfigMap
 metadata:
   name: level-2a-config
   namespace: default
```
</details>

<details>
<summary>level-2b-app (examples/app-of-apps/root-app.yaml)</summary>
<br>

#### ConfigMap: default/level-2b-config
```diff
 apiVersion: v1
 data:
   app: level-2b
   color: red
+  replicas: "3"
 kind: ConfigMap
 metadata:
   name: level-2b-config
   namespace: default
```
</details>

<details>
<summary>level-2c-app (examples/app-of-apps/root-app.yaml)</summary>
<br>

#### ConfigMap: default/level-2c-config
```diff
+apiVersion: v1
+data:
+  app: level-2c
+  color: orange
+kind: ConfigMap
+metadata:
+  name: level-2c-config
+  namespace: default
```
</details>

_Stats_:
[Applications: 2], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
