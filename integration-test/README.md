# Integration Tests

This directory contains integration tests for `argocd-diff-preview`. These tests verify the tool works correctly end-to-end by running it against real Kubernetes clusters with ArgoCD installed.

## Running Tests (from repo root)

**All commands should be run from the repository root directory using `make`.**

### Quick Reference

```bash
# Build and run all integration tests with Go binary (CLI mode)
make run-integration-tests-go

# Build and run all integration tests with Docker
make run-integration-tests-docker

# Run with Argo CD server API mode
make run-integration-tests-go-with-api
make run-integration-tests-docker-with-api

# Run with Argo CD repo server API mode (experimental)
make run-integration-tests-go-with-repo-server-api
make run-integration-tests-docker-with-repo-server-api

# Update expected output files after intentional changes
make update-integration-tests
make update-integration-tests-docker

# Pre-release check (lint + unit tests + integration tests)
make check-release
```

### What Each Target Does

| Make Target                                    | Description                                                                                   |
| ---------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `run-integration-tests-go`                     | Builds Go binary, runs all tests in CLI mode                                                  |
| `run-integration-tests-docker`                 | Builds Docker image, runs all tests using Docker                                              |
| `run-integration-tests-go-with-api`            | Runs all tests forcing `--render-method=server-api`                                           |
| `run-integration-tests-docker-with-api`        | Runs all tests with Docker + `--render-method=server-api`                                     |
| `run-integration-tests-go-with-repo-server-api`| Runs all tests forcing `--render-method=repo-server-api`                                      |
| `run-integration-tests-docker-with-repo-server-api` | Runs all tests with Docker + `--render-method=repo-server-api`                          |
| `update-integration-tests`                     | Regenerates expected output files (use after intentional changes)                             |
| `check-release`                                | Full pre-release validation: lint → unit tests → Go (CLI) → Go (repo-server-api) → Docker (server-api) |
| `check-release-repeat`                         | Runs `check-release` in a loop until failure (for catching flaky tests)                       |

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

# Force a specific render method
TEST_CASE="branch-1/target-1" go test -v -timeout 10m -run TestSingleCase -render-method=server-api ./...
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

### Test Ordering to Minimize Cluster Recreations

Creating a kind cluster + installing ArgoCD takes ~45 seconds. To speed things up, tests are grouped before running:

1. Tests are **partitioned by RBAC configuration** (roles-enabled vs roles-disabled)
2. Within each group, tests are **shuffled randomly**
3. Tests that require `CreateCluster: "true"` are moved to the **front of their group** so they overlap with the cluster creation that already happens at group boundaries
4. The two groups are concatenated: roles-enabled first, then roles-disabled

This means only one cluster recreation is needed at the boundary between the two groups (in addition to any explicit `CreateCluster` tests).

### Periodic Cluster Recreation

As a safeguard against accumulated state, a new cluster is created every **8 tests** within each group.

### Three Render Methods

Tests can run in three modes, controlled by the `--render-method` flag:

| Mode                                   | Cluster Roles | How it works                                   |
| -------------------------------------- | ------------- | ---------------------------------------------- |
| **`cli`** (default)                    | Enabled       | Uses `argocd` CLI to render manifests           |
| **`server-api`**                       | Disabled      | Uses Argo CD REST API directly                 |
| **`repo-server-api`** (experimental)   | Disabled      | Calls the Argo CD repo-server gRPC API directly |

When switching between modes that require different RBAC configurations, the cluster is automatically deleted and recreated.

## Directory Structure

```
integration-test/
├── integration_test.go      # Main test harness
├── auth_token_test.go       # Auth token generation tests
├── no-cluster-roles/
│   └── values.yaml          # Helm values for disabling cluster roles (createClusterRoles: false)
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

Each `branch-N/target[-suffix]/` directory contains expected output files (`output.md` and `output.html`) that the test compares against actual tool output.

## Test Cases

### Branch 1: Basic Functionality
- `target-1`: Basic diff with custom line count and kind options; forces a new cluster
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
- `target-4`: Filter by `--selector=team=my-team` with ArgoCD UI URL
- `target-5`: Filter by selector with no matches; custom title
- `target-6`: Filter by `--file-regex`
- `target-7`: File regex with no matches
- `target-8`: `--watch-if-no-watch-pattern-found` with files changed
- `target-9`: `--watch-if-no-watch-pattern-found` + `--auto-detect-files-changed` with `server-api` render method

### Branch 6-8: ApplicationSets and Git Generators
- `branch-6`: ApplicationSet handling
- `branch-7`: Specific file changes with `--files-changed`
- `branch-8`: Git generator with multiple file changes and ArgoCD UI URL

### Branch 9: Output Limits
- `target-1`: Max diff length of 10000
- `target-2`: Max diff length of 900 (truncation test) with `--files-changed`

### Branch 10-12: Special Features
- `branch-10`: `ignoreDifferences` in Application spec with `--files-changed`
- `branch-11`: Auto-detect files changed (`--auto-detect-files-changed`); new app added
- `branch-12/target-1`: `--auto-detect-files-changed` + `--diff-ignore=annotations`
- `branch-12/target-2`: `--auto-detect-files-changed` + `--diff-ignore=annotations` + `--ignore-resources`

### Branch 13: Label Selectors
- `target-1`: Full diff without selector
- `target-2`: Filter by `--selector=team=your-team`

### Branch 15: Additional Coverage
- `target`: Basic diff (no special options)

## Prerequisites

- Docker running
- Go 1.21+
- `kind`, `kubectl`, `helm` installed (for Go binary mode)

## Test Flags

When running tests directly with `go test` (not via `make`), these flags are available:

| Flag              | Description                                                                  |
| ----------------- | ---------------------------------------------------------------------------- |
| `-update`         | Update expected output files with actual output (golden file mode)           |
| `-docker`         | Use Docker image instead of Go binary                                        |
| `-debug`          | Enable debug mode for the tool                                               |
| `-create-cluster` | Force creation of a new cluster (deletes existing one) - for `TestSingleCase` |
| `-render-method`  | Force all tests to use a specific render mode: `cli`, `server-api`, or `repo-server-api` |
| `-binary`         | Path to Go binary (default: `./bin/argocd-diff-preview`)                    |
| `-image`          | Docker image name (default: `argocd-diff-preview`)                          |

## Test Case Configuration

Each test case in `integration_test.go` is defined as a `TestCase` struct:

```go
type TestCase struct {
    Name                       string // Test name (e.g., "branch-1/target-1")
    TargetBranch               string // Git branch for target state
    BaseBranch                 string // Git branch for base state
    Suffix                     string // Suffix for multiple tests on same branch

    // Tool options
    LineCount                  string // --line-count
    DiffIgnore                 string // --diff-ignore
    FilesChanged               string // --files-changed
    Selector                   string // --selector
    FileRegex                  string // --file-regex
    Title                      string // --title
    MaxDiffLength              string // --max-diff-length
    HideDeletedAppDiff         string // --hide-deleted-app-diff
    IgnoreResources            string // --ignore-resources
    WatchIfNoWatchPatternFound string // --watch-if-no-watch-pattern-found
    AutoDetectFilesChanged     string // --auto-detect-files-changed (uses git diff)
    IgnoreInvalidWatchPattern  string // --ignore-invalid-watch-pattern
    ArgocdUIURL                string // --argocd-ui-url

    // Cluster/Auth options
    KindOptions                string // --kind-options
    CreateCluster              string // "true" to force a new cluster for this test
    ArgocdLoginOptions         string // --argocd-login-options
    ArgocdAuthToken            string // --argocd-auth-token (used instead of username/password login)
    RenderMethod               string // "cli", "server-api", "repo-server-api", or "" to use global -render-method flag
    DisableClusterRoles        string // "true" to install ArgoCD with createClusterRoles: false

    ExpectFailure              bool   // If true, the test is expected to fail
}
```

`RenderMethod` on a test case overrides the global `-render-method` flag for that specific test, allowing individual tests to pin a render mode regardless of how the suite is invoked.

## RBAC Configuration Handling

### The Problem

ArgoCD can be installed with or without cluster-wide RBAC roles:
- **With cluster roles**: ArgoCD can list resources across all namespaces (needed for CLI mode)
- **Without cluster roles**: ArgoCD is restricted to its namespace (required for API modes)

### The Solution

The test harness automatically detects and resolves RBAC mismatches before each test:

```go
// Check RBAC state vs. what the test needs
clusterHasRoles := clusterHasArgocdClusterRoles()
// testNeedsRolesDisabled is true for API mode tests and DisableClusterRoles tests
rbacMismatch := testNeedsRolesDisabled == clusterHasRoles
if rbacMismatch {
    deleteKindCluster() // Force recreation with correct config
}
```

For the Go binary, RBAC is controlled via `--argocd-config-dir`, pointing at `./integration-test/no-cluster-roles` (which contains a `values.yaml` with `createClusterRoles: false`). For Docker, the same file is bind-mounted into the container.

### Test Requirements

| Test Config                          | Needs Roles Disabled | Reason                                     |
| ------------------------------------ | -------------------- | ------------------------------------------ |
| `DisableClusterRoles: "true"`        | Yes                  | Explicitly testing restricted mode         |
| `RenderMethod: "server-api"`         | Yes                  | API mode doesn't need cluster roles        |
| `RenderMethod: "repo-server-api"`    | Yes                  | API mode doesn't need cluster roles        |
| Global `-render-method=server-api`   | Yes                  | Unless test sets its own `RenderMethod`    |
| Default / `RenderMethod: "cli"`      | No                   | CLI mode requires cluster roles            |

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
Check if tests are alternating between CLI and API mode. The harness will detect RBAC mismatches and recreate as needed - this is expected behavior when the render method changes.

### Too much output from integration tests
Integration tests produce a lot of output (cluster creation, ArgoCD deployment, etc.). Pipe to `tail` to avoid overflow:
```bash
make run-integration-tests-go 2>&1 | tail -50
```
