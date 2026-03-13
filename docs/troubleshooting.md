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

## Repo server connection lost or refused during rendering

When using `--render-method=repo-server-api`, you may see the repo-server pod crash in the middle of a run. This typically manifests as:

```
Port forward failed error="lost connection to pod"
⚠️ Transient gRPC Unavailable error from repo server; will retry error="rpc error: code = Unavailable desc = error reading from server: EOF" (app: my-app) (attempt: 1)
...
❌ Failed to render application via repo server: error="... repo server unavailable after 5 attempts: rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing: dial tcp [::1]:8083: connect: connection refused\""
```

To confirm that the repo-server pod actually restarted, look for a log line like:

```
⚠️ Container 'argocd-repo-server' in pod 'argocd-repo-server-xxx' has restarted (restarts: 0 -> 1). This may cause rendering failures or timeouts.
```

The most common cause is the repo-server running out of memory when rendering many applications concurrently. You can address this in two ways:

1. **Reduce concurrency** to put less pressure on the repo server:
   ```bash
   argocd-diff-preview --render-method=repo-server-api --concurrency=10 ...
   ```

2. **Increase the memory limit** for the repo-server by passing custom Helm values when installing Argo CD (see the [Helm values](https://github.com/argoproj/argo-helm/tree/main/charts/argo-cd) for available options).

## Stale cache issues when connecting to an existing Argo CD instance

When connecting to an existing Argo CD instance, Argo CD caches git references to improve performance. However, if the cache becomes stale (for example, if branches are updated or new commits are pushed), you may see errors like:

```
unable to resolve git revision : unable to resolve 'refs/pull/833/merge' to a commit SHA
```

To avoid this issue, use commit SHAs instead of branch names when specifying revisions. Please refer to the [Branch Names vs Commit SHAs](reusing-clusters/branch-vs-sha.md) guide for more details on why this happens and how to configure your workflow correctly.