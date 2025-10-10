argocd_namespace := argocd-diff-preview
github_org := dag-andersen
gitops_repo := argocd-diff-preview
base_branch := main
helm-example-3 := main
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

pull-repository:
	@rm -rf base-branch || true && mkdir -p base-branch
	@rm -rf target-branch || true && mkdir -p target-branch
	cd base-branch   && git clone https://github.com/$(github_org)/$(gitops_repo).git --depth=1 --branch "$(base_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -
	cd target-branch && git clone https://github.com/$(github_org)/$(gitops_repo).git --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

docker-build:
	docker build . -f ../Dockerfile -t image --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE)

start-cluster:
	kind delete cluster --name argocd-diff-preview || true
	kind create cluster --name argocd-diff-preview

stop-cluster:
	kind delete cluster --name argocd-diff-preview


install-argocd:
	helm install argo-cd argo/argo-cd --version 8.0.3 -n $(argocd_namespace) --create-namespace -f ../argocd-config/values.yaml -f ../argocd-config/values-override.yaml -f values.yaml
	sleep 5

	argocd login --core
	kubectl config set-context --current --namespace=$(argocd_namespace)
	argocd admin initial-password
	argocd account update-password --account alice

docker-build:
	cd .. && $(MAKE) docker-build


run-tool:
	docker run \
		--network=host \
		-v ~/.kube:/root/.kube \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD)/base-branch:/base-branch \
		-v $(PWD)/target-branch:/target-branch \
		-v $(PWD)/output:/output \
		-v $(PWD)/secrets:/secrets \
		-e BASE_BRANCH=$(base_branch) \
		-e TARGET_BRANCH=$(target_branch) \
		-e REPO=$(github_org)/$(gitops_repo) \
		image \
		--argocd-namespace="$(argocd_namespace)" \
		--argocd-username=admin \
		--argocd-password="hejmeddig" \
		--create-cluster=false \
		--debug

run: pull-repository start-cluster install-argocd run-tool
