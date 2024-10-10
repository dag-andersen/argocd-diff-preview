# Ignore specific lines in the diff preview

Since this tool only highlights diffs between branches, it is important to stay up to date with your main branch. If your main branch is updated often with new tags for you container images, it can be hard to keep up with the newest changes.

You might see a lot of previews including simple changes like `image: my-image:v1.0.0` to `image: my-image:v1.0.1`.

*Example:*

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

This will ignore changes like in the example above.

`argocd-diff-preview` uses `git diff` for generating the diff. For more information on how the lines are ignored, read their docs: [git-diff](https://git-scm.com/docs/git-diff).