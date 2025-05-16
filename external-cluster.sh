kind delete cluster --name existing-cluster || true
kind create cluster --name existing-cluster
helm repo add argo https://argoproj.github.io/argo-helm
helm install my-argo-cd argo/argo-cd --version 8.0.3

sleep 30

rm -rf base-branch || true && mkdir -p base-branch
rm -rf target-branch || true && mkdir -p target-branch
cd base-branch   && git clone https://github.com/dag-andersen/argocd-diff-preview.git --depth=1 --branch "main" && cp -r argocd-diff-preview/. . && rm -rf .git && echo "*" > .gitignore && rm -rf argocd-diff-preview && cd -
cd target-branch && git clone https://github.com/dag-andersen/argocd-diff-preview.git --depth=1 --branch "helm-example-3" && cp -r argocd-diff-preview/. . && rm -rf .git && echo "*" > .gitignore && rm -rf argocd-diff-preview && cd -

docker run \
    --network=host \
    -v ~/.kube:/root/.kube \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v $(PWD)/base-branch:/base-branch \
    -v $(PWD)/target-branch:/target-branch \
    -v $(PWD)/output:/output \
    -e BASE_BRANCH=main \
    -e TARGET_BRANCH=helm-example-3 \
    -e REPO=dag-andersen/argocd-diff-preview \
    dagandersen/argocd-diff-preview:v0.1.9-pre-release-ae0b440 \
    --argocd-namespace=default \
    --create-cluster=false