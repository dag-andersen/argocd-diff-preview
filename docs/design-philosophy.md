# Design Philosophy

This page outlines the core design principles behind `argocd-diff-preview`. Understanding these principles will help you understand why the tool works the way it does and what trade-offs have been made.

!!! note "Have a different perspective? Let's talk!"
    These design principles reflect our current thinking, but we're always open to discussion. If you have a different perspective, a use case we haven't considered, or ideas for improvement, please [open an issue](https://github.com/dag-andersen/argocd-diff-preview/issues) on GitHub. We welcome the discussion!


## Use Argo CD to render manifests - Don't reinvent the wheel

Many alternative tools try to mimic or re-implement Argo CD's rendering logic. This is a losing battle because Argo CD's rendering logic is complex and constantly evolving. It supports Helm, Kustomize, plain YAML, Jsonnet, custom plugins, and combinations of all of these. Trying to replicate this logic outside of Argo CD inevitably leads to subtle differences that result in confusing diffs that don't reflect reality.

**The only way to ensure accurate diffs is to use Argo CD itself to render the manifests.**

## Compare desired states - Not actual states

Unlike some alternative tools or the `argocd diff` CLI command, `argocd-diff-preview` does **not** compare the desired state in Git with the actual state in Kubernetes. Instead, it compares the desired state of two branches - both stored in Git.

This is a deliberate design choice. Here's why:

### The problem with comparing to live state

When you compare a Git branch to the live state in a Kubernetes cluster, you introduce non-determinism and ambiguity. The actual state in Kubernetes can differ from the desired state for many reasons:

- **Temporary drift** - Someone made a manual change that will be reverted on next sync
- **Failed syncs** - An application failed to sync and is out of date
- **Sync delays** - Auto-sync hasn't run yet after a recent merge
- **External controllers** - Other controllers (like HPA or VPA) modified resources
- **Admission webhooks** - Webhooks injected sidecars or modified resources

This also raises confusing questions: If an application doesn't have auto-sync enabled and is currently out-of-sync, what should the diff show? Should it compare against the "not-yet-synced" desired state, or against the manifests that were applied last time the app was synced? These ambiguities make the output hard to trust and reason about.

If you've used alternative tools that compare to live state, you've likely experienced this: each time you run the tool, it may produce a different result if the cluster state changes often. This makes it hard to trust the output and leads to confusion during code review.

### The benefit of comparing two Git branches

By comparing two Git branches (typically `main` vs your PR branch), the output is **deterministic**. Running the tool twice with the same inputs will always produce the same output. This makes the diff:

- **Trustworthy** - Reviewers can rely on what they see
- **Reproducible** - You can re-run the tool and get the same result
- **Focused** - Shows only the changes introduced by the PR, not unrelated drift

### What about drift detection?

If you need to detect drift between Git and your cluster, that's a different problem with different tools. Argo CD itself has excellent drift detection built-in. The purpose of `argocd-diff-preview` is to help you review changes *before* they're merged - not to monitor the state of your cluster.

### GitOps best practices

This design assumes GitOps best practices where auto-sync is enabled and Git is the source of truth. If changes merged to `main` are automatically applied to your cluster, then you don't need to compare your PR branch to the live cluster state - you only need to compare it to `main`, because `main` *is* the live state (or will be shortly after merge).
