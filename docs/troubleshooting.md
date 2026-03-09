## Debug Mode

If you are having trouble with the tool, you can enable debug mode to get more information about what is going wrong. To enable debug mode run the tool with the `--debug` flag.

If that doesn't help or you still have questions, please open an issue in the repository!

## Client-Side Throttling

By default, client-side rate limiting is disabled, and the tool relies on [API Priority and Fairness (APF)](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/) for server-side rate limiting (Kubernetes 1.20+). If you need to re-enable client-side throttling (QPS: 20, Burst: 40), you can do so with:

```bash
argocd-diff-preview --disable-client-throttling=false ...
```

## Applications being empty

If you are experiencing issues with Argo CD applications being marked as empty, try running the tool with a different `--render-method` to see if that resolves the issue. See the [Rendering Methods](rendering-methods.md) page for the available options. Otherwise please open an issue with details about your cluster setup and any error/warning messages you are seeing.

## Stale cache issues when connecting to an existing Argo CD instance

When connecting to an existing Argo CD instance, Argo CD caches git references to improve performance. However, if the cache becomes stale (for example, if branches are updated or new commits are pushed), you may see errors like:

```
unable to resolve git revision : unable to resolve 'refs/pull/833/merge' to a commit SHA
```

To avoid this issue, use commit SHAs instead of branch names when specifying revisions. Please refer to the [Branch Names vs Commit SHAs](reusing-clusters/branch-vs-sha.md) guide for more details on why this happens and how to configure your workflow correctly.