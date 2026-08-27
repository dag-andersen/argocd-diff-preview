## Argo CD Diff Preview

Summary:
```yaml
Modified (4):
± cluster-a-cluster-rbac (-25)
± kustomize-build-options (+2|-2)
± owner-a (-12)
± owner-b (+12)
```

<details>
<summary>cluster-a-cluster-rbac (examples/cluster-rbac-repro/deployment-config/applicationsets/cluster-rbac.yaml)</summary>
<br>

#### ClusterRole: sympozium-agent-node-reader
```diff
-apiVersion: rbac.authorization.k8s.io/v1
-kind: ClusterRole
-metadata:
-  name: sympozium-agent-node-reader
-rules:
-- apiGroups:
-  - ""
-  resources:
-  - nodes
-  verbs:
-  - get
-  - list
-  - watch
```
#### ClusterRoleBinding: sympozium-agent-node-reader
```diff
-apiVersion: rbac.authorization.k8s.io/v1
-kind: ClusterRoleBinding
-metadata:
-  name: sympozium-agent-node-reader
-roleRef:
-  apiGroup: rbac.authorization.k8s.io
-  kind: ClusterRole
-  name: sympozium-agent-node-reader
-subjects:
-- apiGroup: rbac.authorization.k8s.io
-  kind: Group
-  name: aks:jwt:platform-cluster:system:serviceaccount:automation:automation-agent
```
</details>

<details>
<summary>kustomize-build-options (examples/kustomize-build-options/application.yaml)</summary>
<br>

#### Deployment: default/kustomize-build-options
```diff
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   name: kustomize-build-options
   namespace: default
 spec:
-  replicas: 1
+  replicas: 3
   selector:
     matchLabels:
       app: kustomize-build-options
   template:
     metadata:
       labels:
         app: kustomize-build-options
     spec:
       containers:
-      - image: nginx:1.25
+      - image: nginx:1.27
         name: app
```
</details>

<details>
<summary>owner-a (examples/resource-application-boundary/owner-a/application.yaml)</summary>
<br>

#### ClusterRole: moved-resource
```diff
-apiVersion: rbac.authorization.k8s.io/v1
-kind: ClusterRole
-metadata:
-  name: moved-resource
-rules:
-- apiGroups:
-  - ""
-  resources:
-  - nodes
-  verbs:
-  - get
-  - list
```
</details>

<details>
<summary>owner-b (examples/resource-application-boundary/owner-b/application.yaml)</summary>
<br>

#### ClusterRole: moved-resource
```diff
+apiVersion: rbac.authorization.k8s.io/v1
+kind: ClusterRole
+metadata:
+  name: moved-resource
+rules:
+- apiGroups:
+  - ""
+  resources:
+  - nodes
+  verbs:
+  - get
+  - list
```
</details>

_Stats_:
[Applications: 14], [Full Run: Xs], [Rendering: Xs], [Cluster: Xs], [Argo CD: Xs]
