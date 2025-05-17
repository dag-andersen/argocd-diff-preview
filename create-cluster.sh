kind delete cluster --name existing-cluster || true
kind create cluster --name existing-cluster
helm repo add argo https://argoproj.github.io/argo-helm
helm install my-argo-cd argo/argo-cd --version 8.0.3