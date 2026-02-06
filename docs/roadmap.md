# Roadmap

This page outlines upcoming changes to `argocd-diff-preview`. We encourage users to update their configurations in advance to ensure a smooth transition when these changes are released.

---

## Upcoming Default Changes

The following flags will have their **default values changed** in a future release:

### `--use-argocd-api`

| Current Default | Future Default |
|-----------------|----------------|
| `false`         | `true`         |

**What it does:** Uses the Argo CD API instead of the CLI to render manifests.

**Why the change:** 

- The API is more reliable because we can import the Argo CD types directly, eliminating issues with parsing CLI output.
- Makes the Docker image smaller by removing the Argo CD CLI dependency.
- API is often faster than the CLI

---

### `--auto-detect-files-changed`

| Current Default | Future Default |
|-----------------|----------------|
| `false`         | `true`         |

**What it does:** Automatically detects which files changed between branches to optimize application selection.

**Why the change:**

- Just a better experience for users compared to providing the tool with `--files-changed` manually.

---

### `--watch-if-no-watch-pattern-found`

| Current Default | Future Default |
|-----------------|----------------|
| `false`         | `true`         |

**What it does:** Renders applications that do not have a `watch-pattern` annotation. Currently, applications without this annotation are skipped unless this flag is enabled.

**Why the change:**

- This is just a helper option to avoid issues related to people accidentally forgetting to add the `watch-pattern` annotation to their applications. By enabling this by default, we can ensure that all applications are rendered and compared, even if they don't have the annotation. This is just a failsafe. If users intentionally want to skip all applications without a watch pattern, they can set this flag to `false`.

---

### Recommended Migration

To prepare for these changes, we recommend updating your CI/CD configuration now to explicitly use the new defaults:

By adopting these values early, you can:

1. **Test the new behavior** before it becomes the default
2. **Avoid surprises** when the defaults change
3. **Provide feedback** if you encounter any issues

---

## Improved Diff Output

After KubeCon EU 2025, we may explore improving the diff output to make it clearer which Kubernetes resources actually changed. Currently (as of v0.1.23), the diff is displayed as one long YAML file per application, which can sometimes be difficult to read.

No concrete plans yet - just an idea for the future.

---

## Feedback

If you have concerns about these changes or encounter issues while testing the new defaults, please [open an issue](https://github.com/dag-andersen/argocd-diff-preview/issues) on GitHub.
