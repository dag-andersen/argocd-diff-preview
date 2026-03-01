# AGENTS.md - Guidelines for AI Coding Agents

ArgoCD Diff Preview is a Go CLI tool that generates diffs between Argo CD application manifests across Git branches.
It uses Argo CD to do the rendering.
The applications are applied to the cluster. But never synced, so the tracked resources are never actually created in the cluster.
This allows us to get the rendered manifests with all the transformations applied, without actually creating any resources in the cluster.

## Build Commands

```bash
make go-build                    # Build Go binary to bin/argocd-diff-preview
make docker-build                # Build Docker image
make run-with-go target_branch=<branch>    # Run with Go
make run-with-docker target_branch=<branch> # Run with Docker
```

## Test Commands

```bash
# Unit tests (runs on cmd/ and pkg/ directories only)
make run-unit-tests              # Run all unit tests
go test ./pkg/diff/...           # Run tests for specific package
go test -run TestDiff_prettyName ./pkg/diff/...  # Run single test by name

# Integration tests
make run-integration-tests-go    # Integration tests with Go binary
make run-integration-tests-docker # Integration tests with Docker

# Run a single integration test (useful for debugging)
# Reuses existing cluster if available, otherwise creates a new one
cd integration-test && TEST_CASE="branch-1/target-1" go test -v -timeout 10m -run TestSingleCase ./...
cd integration-test && TEST_CASE="branch-1/target-1" go test -v -timeout 10m -run TestSingleCase -docker ./...

# Force all tests to use the ArgoCD server API instead of CLI
cd integration-test && go test -v -timeout 60m -run TestIntegration -render-method=server-api ./...
cd integration-test && TEST_CASE="branch-1/target-1" go test -v -timeout 10m -run TestSingleCase -render-method=server-api ./...

# Force new cluster creation for single test
cd integration-test && TEST_CASE="branch-1/target-1" go test -v -timeout 10m -run TestSingleCase -create-cluster ./...

# Update expected outputs (when test output changes intentionally)
make update-integration-tests    # Update with Go binary

# NOTE FOR AI AGENTS: Integration tests produce a LOT of output (cluster creation, 
# ArgoCD deployment, etc). Always pipe to `tail -50` or similar to avoid output overflow:
make update-integration-tests 2>&1 | tail -50
make run-integration-tests-go 2>&1 | tail -50
```

### Commit Messages and PR Titles

Use the format: `<Prefix> | <message>`

Prefixes:
- `Feat` - New features or enhancements
- `Fix` - Bug fixes
- `Docs` - Documentation changes
- `Tests` - Test additions or changes
- `Chore` - Maintenance, refactoring, dependencies

```
# Examples
Feat | Add support for ApplicationSets
Fix | Resolve namespace sorting in diff output
Docs | Update lockdown mode configuration guide
Tests | Add integration tests for multi-app scenarios
Chore | Update ArgoCD dependency to v3.0
```

**Do NOT add issue numbers in parentheses to commit messages or PR titles.** The `(#123)` format is what GitHub automatically adds when merging/squashing PRs (and it refers to the PR number, not the issue).

## Project Structure

```
argocd-diff-preview/
├── cmd/               # CLI entry point (main.go, options.go)
├── pkg/               # Core go logic
├── integration-test/  # Integration tests and expected outputs
├── docs/              # MkDocs documentation
└── examples/          # Test fixtures
```

## Main Challenges when building a tool like this

- A repository can contain multiple applications and applications with the same name. We ALWAYS need to make sure the names are unique.
- We can not make any assumption of how the code i structured and how many applications are stored in the same file.


## Key Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration
- `github.com/rs/zerolog` - Structured logging
- `github.com/argoproj/argo-cd/v3` - Argo CD types
- `k8s.io/client-go` - Kubernetes client
