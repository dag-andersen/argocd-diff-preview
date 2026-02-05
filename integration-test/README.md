# Integration Tests

This directory contains integration tests for `argocd-diff-preview`. These tests verify the tool works correctly end-to-end by running it against real Kubernetes clusters with ArgoCD installed.

## Running Tests (from repo root)

**All commands should be run from the repository root directory using `make`.**

### Quick Reference

```bash
# Build and run all integration tests with Go binary
make run-integration-tests-go

# Build and run all integration tests with Docker
make run-integration-tests-docker

# Run with ArgoCD API mode (instead of CLI)
make run-integration-tests-go-with-api
make run-integration-tests-docker-with-api

# Update expected output files after intentional changes
make update-integration-tests
make update-integration-tests-docker

# Pre-release check (lint + unit tests + integration tests)
make check-release
```

### What Each Target Does

| Make Target                             | Description                                                                          |
| --------------------------------------- | ------------------------------------------------------------------------------------ |
| `run-integration-tests-go`              | Builds Go binary, runs all 28 tests using CLI mode                                   |
| `run-integration-tests-docker`          | Builds Go binary, runs tests using Docker image                                      |
| `run-integration-tests-go-with-api`     | Runs tests forcing `--use-argocd-api=true`                                           |
| `run-integration-tests-docker-with-api` | Runs tests with Docker + API mode                                                    |
| `update-integration-tests`              | Regenerates expected output files (use after intentional changes)                    |
| `check-release`                         | Full pre-release validation: lint → unit tests → integration tests (Go + Docker/API) |
| `check-release-repeat`                  | Runs `check-release` in a loop until failure (for catching flaky tests)              |

### Running a Single Test

For faster iteration during development, run a single test case directly:

```bash
# From repo root, first build the binary
make go-build

# Then run a specific test (reuses existing cluster if available)
cd integration-test
TEST_CASE="branch-1/target-1" go test -v -timeout 10m -run TestSingleCase ./...

# Force new cluster creation
TEST_CASE="branch-1/target-1" go test -v -timeout 10m -run TestSingleCase -create-cluster ./...

# Run with Docker instead of Go binary
TEST_CASE="branch-1/target-1" go test -v -timeout 10m -run TestSingleCase -docker ./...
```

## Design Philosophy

### Reality-Based State Detection

The test harness doesn't track cluster state in variables. Instead, each test iteration **observes the actual cluster state**:

1. **Check if cluster exists** - `kind get clusters`
2. **Check if ArgoCD cluster roles exist** - `kubectl get clusterroles -l app.kubernetes.io/part-of=argocd`
3. **Compare with test requirements** - delete and recreate if there's a mismatch

This approach is more robust than state tracking because:
- Tests run in **randomized order** to catch implicit dependencies
- Some tests may exit early (e.g., no applications to process)
- Cluster state is always verified, never assumed

### Cluster Reuse for Speed

Creating a kind cluster + installing ArgoCD takes ~45 seconds. To speed up tests:
- Clusters are **reused across tests** when possible
- A new cluster is created every **8 tests** as a safeguard
- Clusters are **recreated when RBAC config changes** (CLI vs API mode)

### Two Execution Modes

Tests can run in two modes, controlled by the `--use-argocd-api` flag:

| Mode                   | Cluster Roles | How it works                          |
| ---------------------- | ------------- | ------------------------------------- |
| **CLI mode** (default) | Enabled       | Uses `argocd` CLI to render manifests |
| **API mode**           | Disabled      | Uses ArgoCD REST API directly         |

When switching between modes, the cluster is automatically deleted and recreated with the correct RBAC configuration.

## Directory Structure

```
integration-test/
├── integration_test.go      # Main test harness
├── auth_token_test.go       # Auth token generation tests
├── createClusterRoles.yaml  # Helm values for disabling cluster roles
├── localUserValues.yaml     # Helm values for local user auth
├── README.md                # This file
│
├── branch-1/                # Test case expected outputs
│   ├── target-1/
│   │   ├── output.md        # Expected markdown diff
│   │   └── output.html      # Expected HTML diff
│   ├── target-2/
│   └── ...
├── branch-2/
└── ...
```

### Test Branches

Test data lives in Git branches following the pattern `integration-test/branch-N/{base,target}`:
- `base` branch represents the current state (e.g., `main`)
- `target` branch represents the proposed changes (e.g., a PR)

Each `branch-N/target-M/` directory contains expected output files that the test compares against.

## Test Cases

### Branch 1: Basic Functionality
- `target-1`: Basic diff with custom line count and kind options
- `target-2`: Diff with `--diff-ignore=image`
- `target-3`: Diff with `--hide-deleted-app-diff` and ArgoCD UI URL
- `target-no-cluster-roles`: **Expected failure** - verifies proper error when `createClusterRoles: false` without API mode
- `target-invalid-token`: **Expected failure** - verifies auth token validation

### Branch 2-4: Core Features
- `branch-2`: Basic diff (no special options)
- `branch-3`: Another basic diff variant
- `branch-4`: Custom title

### Branch 5: Filtering Options
- `target-1`: Filter by `--files-changed`
- `target-2`: Filter by watch pattern in application
- `target-3`: Files changed with no matching apps (empty result)
- `target-4`: Filter by `--selector=team=my-team`
- `target-5`: Filter by selector with no matches
- `target-6`: Filter by `--file-regex`
- `target-7`: File regex with no matches
- `target-8`: `--watch-if-no-watch-pattern-found` with files changed
- `target-9`: Auto-detect files changed with API mode

### Branch 6-8: ApplicationSets and Git Generators
- `branch-6`: ApplicationSet handling
- `branch-7`: Specific file changes with new cluster
- `branch-8`: Git generator with multiple file changes

### Branch 9: Output Limits
- `target-1`: Max diff length of 10000
- `target-2`: Max diff length of 900 (truncation test)

### Branch 10-12: Special Features
- `branch-10`: `ignoreDifferences` in Application spec
- `branch-11`: Auto-detect files changed (new app added)
- `branch-12`: Ignore annotations and specific resources

### Branch 13: Label Selectors
- `target-1`: Full diff without selector
- `target-2`: Filter by `--selector=team=your-team`

## Prerequisites

- Docker running
- Go 1.21+
- `kind`, `kubectl`, `helm` installed

## Test Flags

When running tests directly with `go test` (not via `make`), these flags are available:

| Flag              | Description                                              |
| ----------------- | -------------------------------------------------------- |
| `-update`         | Update expected output files with actual output          |
| `-docker`         | Use Docker image instead of Go binary                    |
| `-debug`          | Enable debug mode for the tool                           |
| `-create-cluster` | Force creation of a new cluster                          |
| `-use-argocd-api` | Force all tests to use ArgoCD API mode                   |
| `-binary`         | Path to Go binary (default: `./bin/argocd-diff-preview`) |
| `-image`          | Docker image name (default: `argocd-diff-preview`)       |

## Test Case Configuration

Each test case in `integration_test.go` can configure:

```go
type TestCase struct {
    Name                       string  // Test name (e.g., "branch-1/target-1")
    TargetBranch               string  // Git branch for target state
    BaseBranch                 string  // Git branch for base state
    Suffix                     string  // Suffix for multiple tests on same branch
    
    // Tool options
    LineCount                  string  // --line-count
    DiffIgnore                 string  // --diff-ignore
    FilesChanged               string  // --files-changed
    Selector                   string  // --selector
    FileRegex                  string  // --file-regex
    Title                      string  // --title
    MaxDiffLength              string  // --max-diff-length
    HideDeletedAppDiff         string  // --hide-deleted-app-diff
    IgnoreResources            string  // --ignore-resources
    WatchIfNoWatchPatternFound string  // --watch-if-no-watch-pattern-found
    AutoDetectFilesChanged     string  // --auto-detect-files-changed (uses git diff)
    IgnoreInvalidWatchPattern  string  // --ignore-invalid-watch-pattern
    ArgocdUIURL                string  // --argocd-ui-url
    
    // Cluster/Auth options
    KindOptions                string  // --kind-options
    CreateCluster              string  // Force new cluster for this test
    ArgocdLoginOptions         string  // --argocd-login-options
    ArgocdAuthToken            string  // --argocd-auth-token
    UseArgocdApi               string  // "true"/"false"/"" for --use-argocd-api
    DisableClusterRoles        string  // Install ArgoCD with createClusterRoles: false
    
    ExpectFailure              bool    // Test should fail (for error case testing)
}
```

## RBAC Configuration Handling

### The Problem

ArgoCD can be installed with or without cluster-wide RBAC roles:
- **With cluster roles**: ArgoCD can list resources across all namespaces (needed for CLI mode)
- **Without cluster roles**: ArgoCD is restricted to its namespace (requires API mode)

### The Solution

The test harness automatically detects RBAC mismatches:

```go
// Each iteration checks actual cluster state
clusterExists := kindClusterExists()
if clusterExists {
    clusterHasRoles := clusterHasArgocdClusterRoles()
    // testNeedsRolesDisabled is true for API mode tests
    rbacMismatch := testNeedsRolesDisabled == clusterHasRoles
    if rbacMismatch {
        deleteKindCluster()  // Force recreation with correct config
    }
}
```

### Test Requirements

| Test Config                   | Needs Roles Disabled | Reason                                   |
| ----------------------------- | -------------------- | ---------------------------------------- |
| `DisableClusterRoles: "true"` | Yes                  | Explicitly testing restricted mode       |
| `UseArgocdApi: "true"`        | Yes                  | API mode doesn't need cluster roles      |
| Global `-use-argocd-api` flag | Yes                  | Unless test sets `UseArgocdApi: "false"` |
| Default                       | No                   | CLI mode requires cluster roles          |

## Troubleshooting

### Test fails with RBAC error
The cluster may have wrong RBAC config. Force recreation:
```bash
kind delete cluster --name argocd-diff-preview
# Re-run test
```

### Docker not running
```
failed to connect to the docker API at unix:///...
```
Start Docker Desktop or the Docker daemon.

### Test output doesn't match
If the tool output changed intentionally:
```bash
make update-integration-tests
git diff integration-test/  # Review changes
```

### Cluster keeps getting recreated
Check if tests are alternating between CLI and API mode. The harness will detect RBAC mismatches and recreate as needed - this is expected behavior.
