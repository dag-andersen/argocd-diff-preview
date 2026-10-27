# Filter Diff Output

This page explains how to reduce noise in your diff output by ignoring specific lines or entire resources.

---

## Ignore specific lines

Use the `--diff-ignore` option to hide lines matching a regex pattern.

The tool also hides common noisy generated lines by default, including `caBundle:`, `ca.crt:`, and common Helm checksum annotations such as `checksum/secret:` and `checksum/secrets:`.

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

## Ignore array order

Reordering list entries (e.g. environment variables after refactoring a Helm template) shows up as a diff, even though often nothing meaningful changed. Use `--ignore-array-order` with a list of JSONPath expressions (comma or newline separated). Every array matched by one of the paths is sorted into a canonical order before the diff is generated:

- If every element has a `name` field (e.g. `env`, `ports`, `volumes`, `volumeMounts`), the array is sorted by `name`.
- Otherwise, elements are sorted by their JSON representation.

```bash
# Environment variables of all containers in Deployments, StatefulSets, DaemonSets, Jobs, ...
argocd-diff-preview --ignore-array-order='.spec.template.spec.containers[*].env'

# Environment variables of Deployments only
argocd-diff-preview --ignore-array-order="\$[?(@.kind == 'Deployment')].spec.template.spec.containers[*].env"

# Environment variables of any container type, at any depth (also Pods, CronJobs, CRDs)
argocd-diff-preview --ignore-array-order='..containers[*].env,..initContainers[*].env'

# Volumes and volume mounts
argocd-diff-preview --ignore-array-order='.spec.template.spec.volumes,..containers[*].volumeMounts'
```

Supported JSONPath syntax (a subset of [kubectl JSONPath](https://kubernetes.io/docs/reference/kubectl/jsonpath/)):

| Syntax         | Meaning                                                  |
| -------------- | -------------------------------------------------------- |
| `.field`       | Child field                                              |
| `..field`      | Field at any depth (recursive descent)                   |
| `[*]` / `.*`   | All array elements / all map values                      |
| `[n]`          | Array element at index `n`                               |
| `['a.b/c']`    | Field whose name contains special characters             |
| `[?(@.kind == 'Deployment')]` | Root manifest string-field equality filter |

A leading `$` and kubectl-style braces (`{.spec...}`) are accepted. A root filter tests the current manifest, not an array of manifests. It supports `==` on a direct string field (such as `kind` or `apiVersion`), with single- or double-quoted string values. Nested filters, other operators, compound conditions, slices and unions are not supported. Paths that don't match anything in a resource are ignored.

Real changes (added, removed or modified elements) are still shown. The sorting only affects the diff - files written with `--output-app-manifests` or `--output-branch-manifests` keep the original order.

!!! warning
    Array order is not always irrelevant. For example, Kubernetes only expands `$(VAR_NAME)` references to environment variables defined **earlier** in the list, and the order of `args`, `command` or `initContainers` matters. A reordering of a matched array will not show up in the diff.

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
