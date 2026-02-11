# Conversation Context for AI Agents

## Overall Goal

The `print-resource-in-diff` branch implements a significant improvement to how diffs are displayed: **splitting diffs by individual Kubernetes resources** instead of showing one monolithic diff per Argo CD Application.

### The Problem (Before)

When an Argo CD Application contains multiple Kubernetes resources (e.g., a Deployment, Service, ConfigMap), the diff output showed everything in one big block. This made it hard to understand which specific resources changed.

### The Solution (After)

Each Kubernetes resource now gets its own header (e.g., `#### Deployment/my-app (default)`) followed by just that resource's diff. This makes PR reviews much easier because reviewers can quickly see exactly which resources are affected.

---

## What Was Built on This Branch

### Commit History (oldest → newest)

| Commit | Description |
|--------|-------------|
| `3d34f561` | First attempt - basic resource splitting |
| `1cb6b94f` | Each resource split by code fence blocks |
| `a07b41ea` | Simple refactor |
| `d54101a2` | Fixed "skipped resource" handling, removed `---` separators from output |
| `6fe6e36f` | Performance: pre-compile ignore pattern regex |
| `b89a3315` | HTML: moved resource headers outside diff blocks for better styling |
| `7b17f1b4` | Changed header format from `@@ @@ ` to bold markdown (`####`) |
| `dd6dcdf8` | Removed redundant action headers (latest) |

### New Files Created

- **`pkg/diff/resource_index.go`** - Parses YAML to extract Kind/Name/Namespace and maps line numbers to resources
- **`pkg/diff/resource_index_test.go`** - Comprehensive tests for the resource parser

### Key Changes to Existing Files

- **`pkg/diff/format.go`** - Now outputs `ResourceBlock` structs instead of raw strings
- **`pkg/diff/generator.go`** - Builds resource index from target branch content
- **`pkg/diff/markdown.go`** - Renders resource headers as `#### Kind/Name (namespace)`
- **`pkg/diff/html.go`** - Renders resource headers as `<h4 class="resource_header">...</h4>`

---

## Sample Output Comparison

### Before (main branch)
```markdown
<details>
<summary>my-app (path/to/app.yaml)</summary>

**Application modified: my-app (path/to/app.yaml)**
```diff
-old deployment line
+new deployment line
---
-old service line
+new service line
```
</details>
```

### After (this branch)
```markdown
<details>
<summary>my-app (path/to/app.yaml)</summary>

#### Deployment/my-app (default)
```diff
-old deployment line
+new deployment line
```

#### Service/my-app (default)
```diff
-old service line
+new service line
```
</details>
```

---

## Latest Session Summary

In the most recent conversation, we:

1. **Identified an issue**: The integration test markdown files still had old "action headers" (`**Application modified: ...**`) that should have been removed

2. **Found the root cause**: The `argocd-config/values.yaml` had been accidentally modified to `createClusterRoles: false`, causing RBAC errors that prevented tests from running properly

3. **Fixed the issue**: Reverted `values.yaml` and re-ran integration tests with `-update` flag

4. **Committed and pushed**: 
   - Commit: `dd6dcdf8` - "Feat | Remove redundant action header from diff output"
   - 48 files changed (source code + integration test expected outputs)
   - Pushed to `origin/print-resource-in-diff`

---

## Current State

✅ **All tests pass:**
- Unit tests: `make run-unit-tests` 
- Linter: `make run-lint` (0 issues)
- Integration tests: Updated and passing

✅ **Branch is pushed** to `origin/print-resource-in-diff`

✅ **Ready for PR** at: https://github.com/dag-andersen/argocd-diff-preview/pull/new/print-resource-in-diff

---

## How the Resource Splitting Works

### Architecture

```
YAML Content → ResourceIndex → formatDiff() → ResourceBlocks → Markdown/HTML
```

1. **`BuildResourceIndex(yamlContent)`** parses multi-document YAML:
   - Splits on `---` separators
   - Extracts `kind`, `metadata.name`, `metadata.namespace` from each document
   - Maps line numbers to resources

2. **`formatDiff()`** uses the index while processing diffs:
   - When crossing a `---` boundary, looks up the next resource
   - Creates a new `ResourceBlock` with the resource header
   - Accumulates diff lines into that block

3. **Output formatters** render the blocks:
   - Markdown: `#### Kind/Name (namespace)` + code fence
   - HTML: `<h4 class="resource_header">` + styled table

### Key Data Structures

```go
// A single resource's identifying info
type ResourceInfo struct {
    Kind      string  // "Deployment"
    Name      string  // "my-app"  
    Namespace string  // "default"
}

// A resource's diff content
type ResourceBlock struct {
    Header  string  // "Deployment/my-app (default)"
    Content string  // "+line\n-line\n line"
}
```

---

## Key Files

| File | Purpose |
|------|---------|
| `pkg/diff/resource_index.go` | **NEW** - Parses YAML, builds line→resource mapping |
| `pkg/diff/format.go` | Creates ResourceBlocks from diffs |
| `pkg/diff/generator.go` | Entry point, builds resource index |
| `pkg/diff/markdown.go` | Markdown output with `####` headers |
| `pkg/diff/html.go` | HTML output with styled headers |

---

## Useful Commands

```bash
# Build & test
make go-build
make run-unit-tests
make run-lint

# Integration tests (pipe to tail - lots of output!)
make run-integration-tests-go 2>&1 | tail -50
make update-integration-tests 2>&1 | tail -50

# Single test (faster iteration)
cd integration-test && TEST_CASE="branch-1/target-1" go test -v -timeout 10m -run TestSingleCase ./...
cd integration-test && TEST_CASE="branch-1/target-1" go test -v -timeout 10m -run TestSingleCase -create-cluster -update ./...
```

---

## Gotchas & Lessons Learned

1. **Don't modify `argocd-config/values.yaml`** - Setting `createClusterRoles: false` breaks integration tests with RBAC errors

2. **Integration tests produce massive output** - Always pipe to `tail -50` or similar

3. **Use `-create-cluster` flag** when cluster state might be stale

4. **Resource headers use target (new) content** - For modified files, the resource index is built from the new version, not the old

---

## Commit Style

```
<Prefix> | <message>
```

- `Feat` - New features
- `Fix` - Bug fixes  
- `Docs` - Documentation
- `Test` - Test changes
- `Chore` - Maintenance

**Don't add `(#123)` to commits** - GitHub adds PR numbers automatically when merging.
