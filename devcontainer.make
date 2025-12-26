# Devcontainer setup targets

devcontainer-setup-docker:
	sudo groupadd docker || true
	sudo usermod -aG docker $$(whoami)

devcontainer-setup-kind:
	curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.31.0/kind-linux-amd64
	chmod +x ./kind
	sudo mv ./kind /usr/local/bin/kind

devcontainer-setup-argocd:
	curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
	sudo install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
	rm argocd-linux-amd64

devcontainer-setup: devcontainer-setup-docker devcontainer-setup-kind devcontainer-setup-argocd
