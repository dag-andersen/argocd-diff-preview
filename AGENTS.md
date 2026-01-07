# AGENTS.md - Guidelines for AI Coding Agents

ArgoCD Diff Preview is a Go CLI tool that generates diffs between Argo CD application manifests across Git branches. It creates ephemeral Kubernetes clusters (kind/k3d/minikube), deploys Argo CD, and renders application manifests to show what changes a PR would introduce.

## Build Commands

```bash
make go-build                    # Build Go binary to bin/argocd-diff-preview
make docker-build                # Build Docker image
make run-with-go target_branch=<branch>    # Run with Go
make run-with-docker target_branch=<branch> # Run with Docker
```

## Test Commands

```bash
# Unit tests
make run-unit-tests              # Run all unit tests
go test ./pkg/diff/...           # Run tests for specific package
go test -run TestDiff_prettyName ./pkg/diff/...  # Run single test by name
go test -race ./...              # Run with race detection

# Integration tests
make run-integration-tests-go    # Integration tests with Go binary
make run-integration-tests-docker # Integration tests with Docker
make run-integration-tests-docker update_expected=true  # Update expected outputs

# Pre-release check (lint + unit + integration)
make check-release
```

## Linting

```bash
make run-lint                    # Or: golangci-lint run
```

Enabled linters: `errcheck`, `unused`, `ineffassign`, `staticcheck`, `modernize`

## Code Style

### Error Handling

```go
// Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to create namespace: %w", err)
}

// Log before returning
if err != nil {
    log.Error().Err(err).Msg("❌ Failed to get ConfigMaps")
    return fmt.Errorf("failed to get ConfigMaps: %w", err)
}
```

## Project Structure

```
argocd-diff-preview/
├── cmd/           # CLI entry point (main.go, options.go)
├── pkg/           # Core logic
│   ├── argocd/    # Argo CD installation
│   ├── diff/      # Diff generation
│   ├── extract/   # Resource extraction
│   ├── cluster/   # Cluster provider interface
│   ├── kind/, k3d/, minikube/  # Cluster implementations
│   └── git/, utils/
├── tests/         # Integration tests
├── docs/          # MkDocs documentation
└── examples/      # Test fixtures
```

## Main Challenges when building a tool like this

- Naming collections of Argo CD Applications. A repository can contain multiple applications with the same name. We ALWAYS need to make sure the names are unique.

## Key Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration
- `github.com/rs/zerolog` - Structured logging
- `github.com/argoproj/argo-cd/v3` - Argo CD types
- `helm.sh/helm/v3` - Helm chart handling
- `k8s.io/client-go` - Kubernetes client

## Prerequisites

- Go 1.21+, Docker, Git, Make
- For Go mode: kind, kubectl, Helm, Argo CD CLI
