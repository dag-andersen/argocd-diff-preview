## Argo CD Diff Preview

Summary:
```yaml
Total: 9 files changed

Deleted (9):
- app1 (-19)
- app1 (-19)
- app2 (-19)
- app2 (-19)
- custom-target-revision-example (-14)
- my-app-set-dev (-79)
- my-app-set-prod (-79)
- my-app-set-staging (-79)
- nginx-ingress (-470)
```

<details>
<summary>app1 (examples/duplicate-names/app/app-set-1.yaml)</summary>
<br>

@@ Application deleted: app1 (examples/duplicate-names/app/app-set-1.yaml) @@
#### Deployment/deploy-from-folder-one
```diff
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  name: deploy-from-folder-one
-spec:
-  replicas: 2
-  selector:
-    matchLabels:
-      app: myapp
-  template:
-    metadata:
-      labels:
-        app: myapp
-    spec:
-      containers:
-      - image: dag-andersen/myapp:latest
-        name: myapp
-        ports:
-        - containerPort: 80
```
</details>

<details>
<summary>app1 (examples/duplicate-names/app/app-set-2.yaml)</summary>
<br>

@@ Application deleted: app1 (examples/duplicate-names/app/app-set-2.yaml) @@
#### Deployment/deploy-from-folder-one
```diff
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  name: deploy-from-folder-one
-spec:
-  replicas: 2
-  selector:
-    matchLabels:
-      app: myapp
-  template:
-    metadata:
-      labels:
-        app: myapp
-    spec:
-      containers:
-      - image: dag-andersen/myapp:latest
-        name: myapp
-        ports:
-        - containerPort: 80
```
</details>

<details>
<summary>app2 (examples/duplicate-names/app/app-set-1.yaml)</summary>
<br>

@@ Application deleted: app2 (examples/duplicate-names/app/app-set-1.yaml) @@
#### Deployment/deploy-from-folder-one
```diff
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  name: deploy-from-folder-one
-spec:
-  replicas: 2
-  selector:
-    matchLabels:
-      app: myapp
-  template:
-    metadata:
-      labels:
-        app: myapp
-    spec:
-      containers:
-      - image: dag-andersen/myapp:latest
-        name: myapp
-        ports:
-        - containerPort: 80
```
</details>

<details>
<summary>app2 (examples/duplicate-names/app/app-set-2.yaml)</summary>
<br>

@@ Application deleted: app2 (examples/duplicate-names/app/app-set-2.yaml) @@
#### Deployment/deploy-from-folder-one
```diff
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  name: deploy-from-folder-one
-spec:
-  replicas: 2
-  selector:
-    matchLabels:
-      app: myapp
-  template:
-    metadata:
-      labels:
-        app: myapp
-    spec:
-      containers:
-      - image: dag-andersen/myapp:latest
-        name: myapp
-        ports:
-        - containerPort: 80
```
</details>

<details>
<summary>custom-target-revision-example (examples/custom-target-revision/app/app.yaml)</summary>
<br>

@@ Application deleted: custom-target-revision-example (examples/custom-target-revision/app/app.yaml) @@
#### Deployment/my-deployment (default)
```diff
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  name: my-deployment
-  namespace: default
-spec:
-  replicas: 2
-  template:
-    spec:
-      containers:
-      - image: dag-andersen/myapp:latest
-        name: my-deployment
-        ports:
-        - containerPort: 80
```
</details>

<details>
<summary>my-app-set-dev (examples/basic-appset/my-app-set.yaml)</summary>
<br>

@@ Application deleted: my-app-set-dev (examples/basic-appset/my-app-set.yaml) @@
#### Deployment/super-app-name (default)
```diff
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  labels:
-    app.kubernetes.io/instance: my-app-set-dev
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: myApp
-    app.kubernetes.io/version: 1.16.0
-    helm.sh/chart: myApp-0.1.0
-  name: super-app-name
-  namespace: default
-spec:
-  replicas: 1
-  selector:
-    matchLabels:
-      app.kubernetes.io/instance: my-app-set-dev
-      app.kubernetes.io/name: myApp
-  template:
-    metadata:
-      labels:
-        app.kubernetes.io/instance: my-app-set-dev
-        app.kubernetes.io/managed-by: Helm
-        app.kubernetes.io/name: myApp
-        app.kubernetes.io/version: 1.16.0
-        helm.sh/chart: myApp-0.1.0
-    spec:
-      containers:
-      - image: nginx:1.16.0
-        imagePullPolicy: IfNotPresent
-        livenessProbe:
-          httpGet:
-            path: /
-            port: http
-        name: myApp
-        ports:
-        - containerPort: 80
-          name: http
-          protocol: TCP
-        readinessProbe:
-          httpGet:
-            path: /
-            port: http
-        resources: {}
-        securityContext: {}
-      securityContext: {}
-      serviceAccountName: super-app-name
```
---
```diff
-apiVersion: v1
-kind: Service
-metadata:
-  labels:
-    app.kubernetes.io/instance: my-app-set-dev
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: myApp
-    app.kubernetes.io/version: 1.16.0
-    helm.sh/chart: myApp-0.1.0
-  name: super-app-name
-  namespace: default
-spec:
-  ports:
-  - name: http
-    port: 80
-    protocol: TCP
-    targetPort: http
-  selector:
-    app.kubernetes.io/instance: my-app-set-dev
-    app.kubernetes.io/name: myApp
-  type: ClusterIP
```
---
```diff
-apiVersion: v1
-automountServiceAccountToken: true
-kind: ServiceAccount
-metadata:
-  labels:
-    app.kubernetes.io/instance: my-app-set-dev
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: myApp
-    app.kubernetes.io/version: 1.16.0
-    helm.sh/chart: myApp-0.1.0
-  name: super-app-name
-  namespace: default
```
</details>

<details>
<summary>my-app-set-prod (examples/basic-appset/my-app-set.yaml)</summary>
<br>

@@ Application deleted: my-app-set-prod (examples/basic-appset/my-app-set.yaml) @@
#### Deployment/super-app-name (default)
```diff
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  labels:
-    app.kubernetes.io/instance: my-app-set-prod
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: myApp
-    app.kubernetes.io/version: 1.16.0
-    helm.sh/chart: myApp-0.1.0
-  name: super-app-name
-  namespace: default
-spec:
-  replicas: 1
-  selector:
-    matchLabels:
-      app.kubernetes.io/instance: my-app-set-prod
-      app.kubernetes.io/name: myApp
-  template:
-    metadata:
-      labels:
-        app.kubernetes.io/instance: my-app-set-prod
-        app.kubernetes.io/managed-by: Helm
-        app.kubernetes.io/name: myApp
-        app.kubernetes.io/version: 1.16.0
-        helm.sh/chart: myApp-0.1.0
-    spec:
-      containers:
-      - image: nginx:1.16.0
-        imagePullPolicy: IfNotPresent
-        livenessProbe:
-          httpGet:
-            path: /
-            port: http
-        name: myApp
-        ports:
-        - containerPort: 80
-          name: http
-          protocol: TCP
-        readinessProbe:
-          httpGet:
-            path: /
-            port: http
-        resources: {}
-        securityContext: {}
-      securityContext: {}
-      serviceAccountName: super-app-name
```
---
```diff
-apiVersion: v1
-kind: Service
-metadata:
-  labels:
-    app.kubernetes.io/instance: my-app-set-prod
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: myApp
-    app.kubernetes.io/version: 1.16.0
-    helm.sh/chart: myApp-0.1.0
-  name: super-app-name
-  namespace: default
-spec:
-  ports:
-  - name: http
-    port: 80
-    protocol: TCP
-    targetPort: http
-  selector:
-    app.kubernetes.io/instance: my-app-set-prod
-    app.kubernetes.io/name: myApp
-  type: ClusterIP
```
---
```diff
-apiVersion: v1
-automountServiceAccountToken: true
-kind: ServiceAccount
-metadata:
-  labels:
-    app.kubernetes.io/instance: my-app-set-prod
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: myApp
-    app.kubernetes.io/version: 1.16.0
-    helm.sh/chart: myApp-0.1.0
-  name: super-app-name
-  namespace: default
```
</details>

<details>
<summary>my-app-set-staging (examples/basic-appset/my-app-set.yaml)</summary>
<br>

@@ Application deleted: my-app-set-staging (examples/basic-appset/my-app-set.yaml) @@
#### Deployment/super-app-name (default)
```diff
-apiVersion: apps/v1
-kind: Deployment
-metadata:
-  labels:
-    app.kubernetes.io/instance: my-app-set-staging
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: myApp
-    app.kubernetes.io/version: 1.16.0
-    helm.sh/chart: myApp-0.1.0
-  name: super-app-name
-  namespace: default
-spec:
-  replicas: 1
-  selector:
-    matchLabels:
-      app.kubernetes.io/instance: my-app-set-staging
-      app.kubernetes.io/name: myApp
-  template:
-    metadata:
-      labels:
-        app.kubernetes.io/instance: my-app-set-staging
-        app.kubernetes.io/managed-by: Helm
-        app.kubernetes.io/name: myApp
-        app.kubernetes.io/version: 1.16.0
-        helm.sh/chart: myApp-0.1.0
-    spec:
-      containers:
-      - image: nginx:1.16.0
-        imagePullPolicy: IfNotPresent
-        livenessProbe:
-          httpGet:
-            path: /
-            port: http
-        name: myApp
-        ports:
-        - containerPort: 80
-          name: http
-          protocol: TCP
-        readinessProbe:
-          httpGet:
-            path: /
-            port: http
-        resources: {}
-        securityContext: {}
-      securityContext: {}
-      serviceAccountName: super-app-name
```
---
```diff
-apiVersion: v1
-kind: Service
-metadata:
-  labels:
-    app.kubernetes.io/instance: my-app-set-staging
-    app.kubernetes.io/managed-by: Helm
-    app.kubernetes.io/name: myApp
-    app.kubernetes.io/version: 1.16.0
-    helm.sh/chart: myApp-0.1.0
-  name: super-app-name
-  namespace: default
-spec:
-  ports:
-  - name: http
-    port: 80
-    protocol: TCP
-    targetPort: http
-  selector:
-    app.kubernetes.io/instance: my-app-set-staging
-    app.kubernetes.io/na
🚨 Diff is too long
</details>

⚠️⚠️⚠️ Diff exceeds max length of 10000 characters. Truncating to fit. This can be adjusted with the `--max-diff-length` flag

_Stats_:
[Applications: 31], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
