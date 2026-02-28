## Debug Mode

If you are having trouble with the tool, you can enable debug mode to get more information about what is going wrong. To enable debug mode run the tool with the `--debug` flag.

If that doesn't help or you still have questions, please open an issue in the repository!

## Client-Side Throttling

By default, the Kubernetes client uses client-side rate limiting (QPS: 20, Burst: 40) to avoid overwhelming the API server. If you're running against a cluster with [API Priority and Fairness (APF)](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/) enabled (Kubernetes 1.20+), you can disable client-side throttling and let the server handle rate limiting instead:

```bash
argocd-diff-preview --disable-client-throttling ...
```

This can improve performance when the cluster's APF configuration allows higher request rates than the client-side defaults.

## Applications being empty

If you are experiencing issues with Argo CD applications being marked as empty, try running the tool with `--render-method=server-api` to see if that resolves the issue. Otherwise please open an issue with details about your cluster setup and any error/warning messages you are seeing.
