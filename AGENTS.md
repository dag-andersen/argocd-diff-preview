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

## How to Create an Integration Test

This is the stuff that would have saved time when getting started. An integration
test runs the real tool against a real kind cluster + Argo CD and compares the
generated diff against a saved "golden" file.

### How it works (the mental model)

- Each test case clones **two real Git branches** from the upstream repo
  (`dag-andersen/argocd-diff-preview`): a `base` branch and a `target` branch.
  These branches are full snapshots of the whole repo. The tool diffs them.
- The test data lives in branches named `integration-test/branch-N/base` and
  `integration-test/branch-N/target`. The example apps the tool renders live
  under `examples/` on those branches.
- The expected output lives **in this repo** (not on the data branches), under
  `integration-test/branch-N/target<suffix>/{output.md,output.html}`. This is
  the golden file the actual run is compared against.
- Test cases are defined as `TestCase` structs in
  `integration-test/integration_test.go` (see the big `testCases` slice).

### Steps to add a new test

1. **Pick the next free branch number.** Run
   `git ls-remote --heads origin 'refs/heads/integration-test/*'` and pick the
   next `branch-N`. (Note: some numbers may exist on the remote but not in
   `testCases` - check both.)

2. **Create the two data branches from `main`, then edit the example files.**
   The branches are just `main` with example files changed:
   ```bash
   git checkout main
   git checkout -b integration-test/branch-N/base
   # add/modify files under examples/ that the test should render
   git commit -am "Tests | Add branch-N base: <what it tests>"

   git checkout -b integration-test/branch-N/target   # branches off base
   # change ONLY the file(s) whose diff you want to verify
   git commit -am "Tests | Add branch-N target: <what changed>"

   git push -u origin integration-test/branch-N/base integration-test/branch-N/target
   ```
   Keep `base` and `target` nearly identical - they should differ only in the
   file(s) under test, so the diff output stays small and focused. (Look at how
   an existing pair like `branch-16` differs by a single file.)

3. **Add the `TestCase`** to the `testCases` slice in `integration_test.go`.
   The expected-output directory is derived from `TargetBranch` + `Suffix`
   (e.g. `TargetBranch: integration-test/branch-N/target`, `Suffix: "-1"` ->
   `branch-N/target-1/`). Useful fields:
   - `RenderMethod`: `"cli"`, `"server-api"`, or `"repo-server-api"` (omit to use the default).
   - `FileRegex` / `FilesChanged`: limit which changed files are considered.
   - `WatchIfNoWatchPatternFound: "false"` + an `argocd-diff-preview/watch-pattern`
     annotation on the app limits rendering to **just that app**, even though the
     branch is a full repo snapshot. This is the easiest way to keep output focused.

4. **Generate the golden output** by running the single test once with `-update`.
   The first run for a new case must use `-update` (there is no golden file to
   compare against yet). This needs Docker running:
   ```bash
   make go-build
   cd integration-test
   TEST_CASE="branch-N/target" go test -v -timeout 20m -run TestSingleCase -update ./...
   ```
   This writes `output.md` / `output.html`. Read them and sanity-check the diff.
   Timing values are auto-normalized to `Xs`, so the golden file is stable.

5. **Verify it actually passes** in compare mode (no `-update`):
   ```bash
   TEST_CASE="branch-N/target" go test -v -timeout 20m -run TestSingleCase ./...
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

## Main Challenges when building a tool like this

- A repository can contain multiple applications and applications with the same name. We ALWAYS need to make sure the names are unique.
- We can not make any assumption of how the code i structured and how many applications are stored in the same file.


## Key Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/spf13/viper` - Configuration
- `github.com/rs/zerolog` - Structured logging
- `github.com/argoproj/argo-cd/v3` - Argo CD types
- `k8s.io/client-go` - Kubernetes client
