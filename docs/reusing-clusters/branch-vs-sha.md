# Branch Names vs Commit SHAs

When specifying git references in your workflow, you have two options: **branch names** or **commit SHAs**. Understanding the difference is important for avoiding cache-related issues.

!!! note
    This is ONLY relevant when connecting to a pre-configured Argo CD instance. When running in an ephemeral cluster, the cache is always empty and there are no staleness issues.

## Branch Name vs SHA: Performance and Correctness

Argo CD caches git references for performance. This caching behaves differently depending on whether you use a **branch name** or a **commit SHA**.

| Aspect             | Branch Name (`main`)                 | Commit SHA (`abc123...`)        |
| ------------------ | ------------------------------------ | ------------------------------- |
| **Correctness**    | ⚠️ Can be stale up to 3 minutes       | ✅ Always exact commit           |
| **Speed**          | ~20-60ms (when cached)               | ~15-55ms (slightly faster)      |
| **Staleness Risk** | ⚠️ **YES** - May reference old commit | ✅ **NO** - Always deterministic |

!!! warning "SHA Must Be Exactly 40 Characters"
    It is critical that the SHA is exactly 40 characters long (e.g., `c71161b819f3fc1ad0673fb928229fcfae47316f`). Shortened SHAs will not bypass the cache and may still encounter resolution issues.

## Why Commit SHAs Are Recommended

Using commit SHAs instead of branch names provides:

- ✅ **No staleness** - Always references the exact commit
- ✅ **Faster** - Skips git ls-remote call entirely
- ✅ **Deterministic** - Same SHA always produces same result
- ✅ **More reliable** - Avoids branch resolution errors in large repositories

## Example: GitHub Actions Context Variables

If you're using GitHub Actions, you can access commit SHAs using the following context variables:

| Variable                                    | Type           | Example               | Description                        |
| ------------------------------------------- | -------------- | --------------------- | ---------------------------------- |
| `${{ github.event.pull_request.head.sha }}` | SHA (40 chars) | `4e22a3cb21fa...`     | ✅ PR branch HEAD commit            |
| `${{ github.event.pull_request.base.sha }}` | SHA (40 chars) | `abc123def456...`     | ✅ Base branch commit               |
| `${{ github.sha }}`                         | SHA (40 chars) | `fed654cba321...`     | ✅ Merge commit SHA (for PR events) |
| `${{ github.event.pull_request.head.ref }}` | Branch name    | `feature-branch`      | ⚠️ PR branch name                   |
| `${{ github.event.pull_request.base.ref }}` | Branch name    | `main`                | ⚠️ Base branch name                 |
| `${{ github.ref }}`                         | Full ref       | `refs/pull/123/merge` | ⚠️ Special commit (for PR events)   |

For other CI/CD platforms (GitLab CI, Jenkins, CircleCI, etc.), consult their documentation for equivalent variables that provide the full commit SHA.

## Example Configuration

**Use commit SHAs for PR previews:**

```yaml
- name: Generate Diff
  run: |
    docker run \
      --network=host \
      -v /var/run/docker.sock:/var/run/docker.sock \
      -v $(pwd)/main:/base-branch \
      -v $(pwd)/pull-request:/target-branch \
      -v $(pwd)/output:/output \
      -e TARGET_BRANCH=${{ github.event.pull_request.head.sha }}  # ✅ Use SHA
      -e REPO=${{ github.repository }} \
      dagandersen/argocd-diff-preview:v0.2.2
```
