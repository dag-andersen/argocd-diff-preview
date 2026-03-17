## Argo CD Diff Preview

Summary:
```yaml
Added (2):
+ level-1c-staging-app (+8)
+ level-2c-app (+8)

Modified (6):
± level-1a-app (+13)
± level-1b-app (+1|-1)
± level-1c-prod-app (+1|-1)
± level-2a-app (+2|-1)
± level-2b-app (+1)
± root-app (+1)
```

<details>
<summary>level-1a-app (examples/app-of-apps/apps/root/level-1a.yaml)</summary>
<br>

#### Application: argocd/level-2c-app
```diff
+apiVersion: argoproj.io/v1alpha1
+kind: Application
+metadata:
+  name: level-2c-app
+  namespace: argocd
+spec:
+  destination:
+    name: in-cluster
+    namespace: default
+  project: default
+  source:
+    path: examples/app-of-apps/apps/level-2c
+    repoURL: https://github.com/dag-andersen/argocd-diff-preview
```
</details>

<details>
<summary>level-1b-app (examples/app-of-apps/apps/root/level-1b.yaml)</summary>
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
<summary>level-1c-prod-app (examples/app-of-apps/apps/root/level-1c-appset.yaml)</summary>
<br>

#### ConfigMap: default/level-1c-prod-config
```diff
 apiVersion: v1
 data:
   app: level-1c-prod
-  color: blue
+  color: purple
 kind: ConfigMap
 metadata:
   name: level-1c-prod-config
   namespace: default
```
</details>

<details>
<summary>level-2a-app (examples/app-of-apps/apps/level-1a/level-2a.yaml)</summary>
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
<summary>level-2b-app (examples/app-of-apps/apps/level-1a/level-2b.yaml)</summary>
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
<summary>root-app (examples/app-of-apps/root-app.yaml)</summary>
<br>

#### ApplicationSet: argocd/level-1c-appset
```diff
 apiVersion: argoproj.io/v1alpha1
 kind: ApplicationSet
 metadata:
   name: level-1c-appset
   namespace: argocd
 spec:
   generators:
   - list:
       elements:
       - env: prod
+      - env: staging
   template:
     metadata:
       name: level-1c-{{env}}-app
       namespace: argocd
     spec:
       destination:
         name: in-cluster
         namespace: default
       project: default
       source:
```
</details>

<details>
<summary>level-1c-staging-app (examples/app-of-apps/apps/root/level-1c-appset.yaml)</summary>
<br>

#### ConfigMap: default/level-1c-staging-config
```diff
+apiVersion: v1
+data:
+  app: level-1c-staging
+  color: orange
+kind: ConfigMap
+metadata:
+  name: level-1c-staging-config
+  namespace: default
```
</details>

<details>
<summary>level-2c-app (examples/app-of-apps/apps/level-1a/level-2c.yaml)</summary>
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
[Applications: 14], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
