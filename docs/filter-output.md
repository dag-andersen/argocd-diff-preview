# Filter Diff Output

This page explains how to reduce noise in your diff output by ignoring specific lines or entire resources.

---

## Ignore specific lines

Use the `--diff-ignore` option to hide lines matching a regex pattern.

### Example: Image tag changes

Since this tool highlights diffs between branches, it's important to stay up to date with your main branch. If your main branch is updated frequently with new container image tags, you'll see a lot of noise in the diff - changes like `image: my-image:v1.0.0` → `image: my-image:v1.0.1` will appear in every PR:

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

To hide these, use a regex that matches version strings:

```bash
argocd-diff-preview --diff-ignore="v[0-9]+\.[0-9]+\.[0-9]+"
```

---

### Example: Helm-generated values

Some Helm charts generate new values on every install (certificates, random strings, etc.). These appear in every PR even when nothing meaningful changed:

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

This hides all lines containing the word `caBundle`.

---

## Ignore resources

Use `--ignore-resources` to exclude entire resources from the diff. The format is:

```
group:kind:name
```

| Component | Description |
|-----------|-------------|
| `group` | API group (empty = core group, `*` = any) |
| `kind` | Resource kind (`*` = any) |
| `name` | Resource name (`*` = any) |

### Example

```bash
argocd-diff-preview --ignore-resources="apps:Deployment:my-deploy,*:CustomResourceDefinition:*,:ConfigMap:argocd-cm"
```

This hides:

- The Deployment named `my-deploy` in the `apps` group
- All CustomResourceDefinitions (any group)
- The ConfigMap named `argocd-cm` in the core group

