# Ignore specific lines in the diff preview

Since this tool only highlights diffs between branches, it is important to stay up to date with your main branch. If your main branch is updated often with new tags for you container images, it can be hard to keep up with the newest changes.

### *Example 1:*

You might see a lot of previews including simple changes like `image: my-image:v1.0.0` to `image: my-image:v1.0.1`.

```diff
diff --git base/deployment target/deployment
@@ -3,38 +3,38 @@ template:
    spec:
      containers:
        - name: my-app
-         image: dag-andersen/my-app:v1.0.1
+         image: dag-andersen/my-app:v1.0.2
          ports:
            - containerPort: 80
```

To avoid this, you can ignore lines in the diff by using the `--diff-ignore` option.

```bash
argocd-diff-preview --diff-ignore="v[1,9]+.[1,9]+.[1,9]+"
```

### *Example 2:*

In some cases, Helm Charts generate new values each time they are installed. When this happens, the diff will appear in every pull request. To avoid this, you can ignore these values by using the `--diff-ignore` option.

```diff
   name: example-name
 webhooks:
 - admissionReviewVersions:
   - v1
   clientConfig:
-    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURLR ...
+    caBundle: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURKe ...
     service:
       name: example-name
       port: 443
   failurePolicy: Fail
   name: vbinding.kb.io
```

```bash
argocd-diff-preview --diff-ignore="caBundle"
```

This will hide all lines that contain the word `caBundle`.