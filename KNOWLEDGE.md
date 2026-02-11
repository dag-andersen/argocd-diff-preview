# Knowledge Base

This document captures known limitations, design decisions, and troubleshooting notes for future reference.

## Diff Output: Resource Index Limitations

### Problem

The diff output uses a **resource index** built from the **target (new) content only** to determine which Kubernetes resource each diff line belongs to. This creates issues when:

1. **A resource is deleted** - deleted lines have no corresponding resource in the target content
2. **A resource changes kind** - e.g., ConfigMap → Secret
3. **Content moves between resources** - e.g., data from a deleted resource appears in a remaining resource

### Current Behavior

#### Resource Kind Change (ConfigMap → Secret)

When a resource changes kind, we detect `-kind: X` and `+kind: Y` in the diff content and update the header to show the transformation:

```
#### ConfigMap → Secret/my-config (default)
```diff
 apiVersion: v1
-kind: ConfigMap
+kind: Secret
 ...
```

This is handled by `detectAndUpdateKindChange()` in `pkg/diff/format.go`.

#### Deleted Resource with Content Moved

When a resource is deleted and some content moves to another resource, the deleted resource's lines appear as deletions within the remaining resource's block:

**Base content:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  key: value
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: other-config
data:
  keyOne: "1"
```

**Target content:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  keyOne: "1"
```

**Current output:**
```
#### ConfigMap/my-config (default)
```diff
 apiVersion: v1
 kind: ConfigMap
 metadata:
   name: my-config
 data:
-  key: value
-apiVersion: v1
-kind: ConfigMap
-metadata:
-  name: other-config
-data:
   keyOne: "1"
```

The deleted `other-config` resource doesn't get its own header - its deletion is shown within `my-config`'s diff block.

### Why This Happens

In `pkg/diff/generator.go`, only the new content's resource index is used:

```go
// Use the new content's resource index for displaying resource headers
resourceIndex := BuildResourceIndex(newContent)
```

This means:
- Lines that exist in target content (additions, unchanged) → correctly attributed to their resource
- Lines that only exist in base content (deletions) → attributed to whatever resource comes next in target content

### Potential Fix

To properly handle deleted resources, we would need to:

1. Build two resource indexes (old and new content)
2. Track separate line numbers for old and new content during diff processing
3. Use the old resource index for deletions, new resource index for additions
4. Detect when a deleted resource should have its own block

This is a non-trivial change that would require significant refactoring of `formatDiff()` in `pkg/diff/format.go`.

### Related Tests

See `pkg/diff/generator_test.go`:
- `TestGenerateGitDiff_ResourceKindChange` - tests the kind change header transformation
- `TestGenerateGitDiff_ResourceDeletedWithContentMoved` - documents the current limitation

### References

- `pkg/diff/format.go` - `formatDiff()`, `detectAndUpdateKindChange()`
- `pkg/diff/generator.go` - `generateGitDiff()`
- `pkg/diff/resource_index.go` - `BuildResourceIndex()`, `GetResourceForLine()`
