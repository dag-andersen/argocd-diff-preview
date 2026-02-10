## Argo CD Diff Preview

Summary:
```yaml
Total: 7 files changed

Modified (7):
± app1 (+1|-1)
± app1 (+1|-1)
± app2 (+1|-1)
± app2 (+1|-1)
± my-app -> my-super-app (+6|-6)
± my-service-staging (+1|-1)
± nginx-ingress (+1|-1)
```

<details>
<summary>app1 (examples/duplicate-names/app/app-set-1.yaml)</summary>
<br>

**Application modified: app1 (examples/duplicate-names/app/app-set-1.yaml)**
#### Deployment/deploy-from-folder-one
```diff
       app: myapp
   template:
     metadata:
       labels:
         app: myapp
     spec:
       containers:
       - image: dag-andersen/myapp:latest
         name: myapp
         ports:
-        - containerPort: 80
+        - containerPort: 8080
```
</details>

<details>
<summary>app1 (examples/duplicate-names/app/app-set-2.yaml)</summary>
<br>

**Application modified: app1 (examples/duplicate-names/app/app-set-2.yaml)**
#### Deployment/deploy-from-folder-one
```diff
       app: myapp
   template:
     metadata:
       labels:
         app: myapp
     spec:
       containers:
       - image: dag-andersen/myapp:latest
         name: myapp
         ports:
-        - containerPort: 80
+        - containerPort: 8080
```
</details>

<details>
<summary>app2 (examples/duplicate-names/app/app-set-1.yaml)</summary>
<br>

**Application modified: app2 (examples/duplicate-names/app/app-set-1.yaml)**
#### Deployment/deploy-from-folder-one
```diff
       app: myapp
   template:
     metadata:
       labels:
         app: myapp
     spec:
       containers:
       - image: dag-andersen/myapp:latest
         name: myapp
         ports:
-        - containerPort: 80
+        - containerPort: 8080
```
</details>

<details>
<summary>app2 (examples/duplicate-names/app/app-set-2.yaml)</summary>
<br>

**Application modified: app2 (examples/duplicate-names/app/app-set-2.yaml)**
#### Deployment/deploy-from-folder-one
```diff
       app: myapp
   template:
     metadata:
       labels:
         app: myapp
     spec:
       containers:
       - image: dag-andersen/myapp:latest
         name: myapp
         ports:
-        - containerPort: 80
+        - containerPort: 8080
```
</details>

<details>
<summary>my-app -> my-super-app (examples/helm/applications/my-app.yaml)</summary>
<br>

**Application modified: my-app -> my-super-app (examples/helm/applications/my-app.yaml)**
#### Deployment/super-app-name (default)
```diff
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   labels:
-    app.kubernetes.io/instance: my-app
+    app.kubernetes.io/instance: my-super-app
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
   name: super-app-name
   namespace: default
 spec:
   replicas: 1
   selector:
     matchLabels:
-      app.kubernetes.io/instance: my-app
+      app.kubernetes.io/instance: my-super-app
       app.kubernetes.io/name: myApp
   template:
     metadata:
       labels:
-        app.kubernetes.io/instance: my-app
+        app.kubernetes.io/instance: my-super-app
         app.kubernetes.io/managed-by: Helm
         app.kubernetes.io/name: myApp
         app.kubernetes.io/version: 1.16.0
         helm.sh/chart: myApp-0.1.0
     spec:
       containers:
       - image: nginx:1.16.0
         imagePullPolicy: IfNotPresent
         livenessProbe:
           httpGet:
@@ skipped 10 lines (34 -> 43) @@
             port: http
         resources: {}
         securityContext: {}
       securityContext: {}
       serviceAccountName: super-app-name
```
#### Service/super-app-name (default)
```diff
 apiVersion: v1
 kind: Service
 metadata:
   labels:
-    app.kubernetes.io/instance: my-app
+    app.kubernetes.io/instance: my-super-app
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
   name: super-app-name
   namespace: default
 spec:
   ports:
   - name: http
     port: 80
     protocol: TCP
     targetPort: http
   selector:
-    app.kubernetes.io/instance: my-app
+    app.kubernetes.io/instance: my-super-app
     app.kubernetes.io/name: myApp
   type: ClusterIP
```
#### ServiceAccount/super-app-name (default)
```diff
 apiVersion: v1
 automountServiceAccountToken: true
 kind: ServiceAccount
 metadata:
   labels:
-    app.kubernetes.io/instance: my-app
+    app.kubernetes.io/instance: my-super-app
     app.kubernetes.io/managed-by: Helm
     app.kubernetes.io/name: myApp
     app.kubernetes.io/version: 1.16.0
     helm.sh/chart: myApp-0.1.0
   name: super-app-name
   namespace: default
```
</details>

<details>
<summary>my-service-staging (examples/kustomize/applications/my-service-staging.yaml)</summary>
<br>

**Application modified: my-service-staging (examples/kustomize/applications/my-service-staging.yaml)**
#### Deployment/staging-myapp (default)
```diff
     spec:
       containers:
       - image: dag-andersen/myapp:latest
         name: myapp
         ports:
         - containerPort: 80
         resources:
           limits:
             memory: 256Mi
           requests:
-            memory: 128Mi
+            memory: 64Mi
```
#### Service/staging-myapp (default)
```diff
 apiVersion: v1
 kind: Service
 metadata:
   labels:
     app: myapp
   name: staging-myapp
   namespace: default
 spec:
   ports:
```
</details>

<details>
<summary>nginx-ingress (examples/helm/applications/nginx.yaml -> examples/helm/applications/nginx-new-path.yaml)</summary>
<br>

**Application modified: nginx-ingress (examples/helm/applications/nginx.yaml -> examples/helm/applications/nginx-new-path.yaml)**
#### Deployment/nginx-ingress-ingress-nginx-controller (default)
```diff
         app.kubernetes.io/part-of: ingress-nginx
         app.kubernetes.io/version: 1.10.0
         helm.sh/chart: ingress-nginx-4.10.0
     spec:
       containers:
       - args:
         - /nginx-ingress-controller
         - --publish-service=$(POD_NAMESPACE)/nginx-ingress-ingress-nginx-controller
         - --election-id=nginx-ingress-ingress-nginx-leader
         - --controller-class=k8s.io/ingress-nginx
-        - --ingress-class=test
+        - --ingress-class=new-test
         - --configmap=$(POD_NAMESPACE)/nginx-ingress-ingress-nginx-controller
         - --validating-webhook=:8443
         - --validating-webhook-certificate=/usr/local/certificates/cert
         - --validating-webhook-key=/usr/local/certificates/key
         - --enable-metrics=false
         env:
         - name: POD_NAME
           valueFrom:
             fieldRef:
               fieldPath: metadata.name
```
</details>

_Stats_:
[Applications: 36], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
