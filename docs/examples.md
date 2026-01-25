# Examples

The following examples are a selection of possible `argocd-diff-preview` use-cases and demonstrate the functionality of the tool. Every example has a [linked pull request in the repository](https://github.com/dag-andersen/argocd-diff-preview/pulls?q=is%3Apr+is%3Aopen+example) where `argocd-diff-preview` is executed to visualize the outcome as pull request comment.

## Collection
| Name | Task | Result |
|-|-|-|
| [example-helm-external-chart](https://github.com/dag-andersen/argocd-diff-preview/pull/15) | Modify the target revision of an ArgoCD application that deploys an external helm chart. Check what changed in the external helm chart by reviewing the rendered application. | `argocd-diff-preview` shows the differences of the helm chart that happend "under the hood". |
| [example-applicationset-refactoring](https://github.com/dag-andersen/argocd-diff-preview/pull/296) | Modify an existing applicationset and move from a List Generator to a Git Generator to derive the environments. Ensure your changes work by reviewing the rendered applications. | This example demonstrates a "Zero change" pull request, so the refactoring is successful when it yields the same ArgoCD applications. The analysis of `argocd-diff-preview` states **No changes found** which confirms the expected result. |

## Run the examples locally

**Prerequisites**

* Docker
* Git

You can play through the examples locally by setting the respective `TARGET_BRANCH` variable to point to an example branch

```bash
TARGET_BRANCH=example-applicationset-refactoring
```

and then execute the following commands:


```bash
TARGET_BRANCH=example-applicationset-refactoring

git clone https://github.com/dag-andersen/argocd-diff-preview base-branch --depth 1 -q 

git clone https://github.com/dag-andersen/argocd-diff-preview target-branch --depth 1 -q -b $TARGET_BRANCH

docker run \
   --network host \
   -v /var/run/docker.sock:/var/run/docker.sock \
   -v $(pwd)/output:/output \
   -v $(pwd)/base-branch:/base-branch \
   -v $(pwd)/target-branch:/target-branch \
   -e TARGET_BRANCH=$TARGET_BRANCH \
   -e REPO=dag-andersen/argocd-diff-preview \
   dagandersen/argocd-diff-preview:v0.1.21
```