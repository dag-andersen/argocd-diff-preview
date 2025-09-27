# Previewing Argo CD Applications on Pull Requests in seconds


> TLDR; You can now render previews of changes in a PR using an existing cluster instead of spinning up a new one each run. This results in a very short preview time while still being very accurate.

This is a continuation of the first blogpost: [Rendering the TRUE Argo CD diff on your PRs](https://dev.to/dag-andersen/rendering-the-true-argo-cd-diff-on-your-prs-10bk). That article describes how you can render previews of changes in a PR using ephemral Kuberentes clusters inside your CI/CD pipeline.

![](./assets/flow_dark.png)

This gives very accurate results compared to alternatives solutions, but it has one main limitation: Speed.

## Speed

This design of spinning up an ephemeral cluster has the natural consequence of having to spend time waiting for the cluster to get ready and install Argo CD on it - every single time you want to render a preview.

So the only way to speed up the preview process, is to reuse the same cluster for multiple runs.

Therefore the tool now supports connecting to an existing cluster, saving around 60-90 seconds per run.

The tool works exactly the same way as in the blog post, but now you don't have to wait for the ephemeral cluster.

`argocd-diff-preview` just needs to access to a KubeConfig file or service account credentials to the cluster with Argo CD installed.

### Concurrency Runs

`argocd-diff-preview` scans the two branches and applies the Applications to the existing cluster. This means that if you look at the Argo CD UI you will see Applications being created and deleted after a few seconds. To avoid naming conflicts with other running previews, each run will have a unique ID, which elimiate the problem.
Each run made by `argocd-diff-preview` gets a unique ID, so there will be no naming collisions, and you can run as many preview is parallel as you want. One could even argue that is great to run multiple previews in parallel, because then Argo CD may have cached the repository you are referencing in your Argo CD Applications, resulting in even faster renders.

### Use a dedicated Argo CD instance

But since applications are being created and delete in Argo CD, i would not recommend using your _real_ Argo CD instance for this. Instead, install a dedicated Argo CD instance next to your production Argo CD instance. This instance is not suppose to sync anything. It is just as a standby instance, that the `argocd-diff-preview` tool can connect to.

The architecture i am suggesting is something like this:

![](./assets/existing-cluster.png)

The main downside here is that you need to add credentials to your CI/CD pipeline to be able to connect to the cluster. Some organizations consider this a security risk. 

### Use a self-hosted runner

If you are using Github Actions, you can use the self-hosted runner, to run your pipeline from inside the cluster and thus avoid adding credentials to your pipeline.

![](./assets/existing-cluster-self-hosted-runner.png)

This runner can then run `kubectl get secrets -n argocd` to access the secrets from the host cluster, and then use them form `argocd-diff-preview`.

Rendering previews like this ensures a very accurate diff (because Argo CD itself is doing the rendering), and it is as fast as possible, because you are not waiting for a cluster to get ready.

## Speeding up the rendering process further.

If your repo contains many applications (think 100+) the rendering process can still take 1min+ (because Argo CD itself is not faster). In these cases i suggest that you take a look at `watch-pattern` annotations or `....`. This helps `argocd-diff-preview` skip all the Applications that are not affected by the changes in the PR.

So instead of rendering 100+ applications, you may only render 6-10 applications resulting in the rendering process taking around 10 seconds to complete.

## How do i set this up?

1. Install a cluster

```bash 
kind create cluster --name cluster-name
```

2. Install Argo CD in a dedicated namespace (e.g., `argocd-diff-preview`)

```bash
helm install argo-cd argo/argo-cd --namespace argocd-diff-preview
```

3. Copy the secrets in `argocd` to `argocd-diff-preview`
4. Install a self hosted runner in the same cluster as Argo CD
5. Give 

> I work for Egmont. Our repoistory contains around 600 applications, and this solution let us do a render in less than 10 seconds.