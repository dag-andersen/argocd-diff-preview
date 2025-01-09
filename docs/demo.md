# Try demo locally with 3 simple commands!

First, make sure Docker is running. Run `docker ps` to check if it's running.

Second, run the following 3 commands:

```bash
git clone https://github.com/dag-andersen/argocd-diff-preview base-branch --depth 1 -q 

git clone https://github.com/dag-andersen/argocd-diff-preview target-branch --depth 1 -q -b helm-example-3

docker run \
   --network host \
   -v /var/run/docker.sock:/var/run/docker.sock \
   -v $(pwd)/output:/output \
   -v $(pwd)/base-branch:/base-branch \
   -v $(pwd)/target-branch:/target-branch \
   -e TARGET_BRANCH=helm-example-3 \
   -e REPO=dag-andersen/argocd-diff-preview \
   dagandersen/argocd-diff-preview:v0.0.30
```

and the output would be something like this:

```
✨ Running with:
✨ - local-cluster-tool: Kind
✨ - base-branch: main
✨ - target-branch: helm-example-3
✨ - secrets-folder: ./secrets
✨ - output-folder: ./output
✨ - repo: dag-andersen/argocd-diff-preview
✨ - timeout: 180 seconds
🚀 Creating cluster...
🚀 Cluster created successfully
🦑 Installing Argo CD Helm Chart version: 'latest'
🦑 Installing Argo CD Helm Chart
🦑 Waiting for Argo CD to start...
🦑 Argo CD is now available
🦑 Logging in to Argo CD through CLI...
🦑 Argo CD installed successfully
🤷 No secrets found in ./secrets
🤖 Fetching all files in dir: base-branch
🤖 Patching applications for branch: main
🤖 Patching 4 Argo CD Application[Sets] for branch: main
🤖 Fetching all files in dir: target-branch
🤖 Patching applications for branch: helm-example-3
🤖 Patching 4 Argo CD Application[Sets] for branch: helm-example-3
🌚 Getting resources from base
⏳ Waiting for 4 out of 4 applications to become 'OutOfSync'. Retrying in 5 seconds. Timeout in 180 seconds...
🌚 Got all resources from 4 applications for base
🧼 Removing applications
🧼 Removed applications successfully
🌚 Getting resources from target
⏳ Waiting for 3 out of 4 applications to become 'OutOfSync'. Retrying in 5 seconds. Timeout in 180 seconds...
🌚 Got all resources from 4 applications for target
💥 Deleting cluster...
🔮 Generating diff between main and helm-example-3
🙏 Please check the ./output/diff.md file for differences
🎉 Done in 99 seconds
```

Finally, you can view the diff by running `cat ./output/diff.md`. The diff should look something like [this](https://github.com/dag-andersen/argocd-diff-preview/pull/16)

!!! important "Questions, issues, or suggestions"
    If you experience issues or have any questions, please open an issue in the repository! 🚀