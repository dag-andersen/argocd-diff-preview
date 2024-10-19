gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main
docker_file := Dockerfile
timeout := 120

pull-repostory:
	@rm -rf base-branch || true && mkdir -p base-branch
	@rm -rf target-branch || true && mkdir -p target-branch
	cd base-branch   && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(base_branch)"   && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -
	cd target-branch && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

local-kind: pull-repostory
	docker build . -f $(docker_file) -t diff-image
	kind delete cluster --name kind || true
	kind create cluster --name kind --config kindify/kind.yaml
	kind load docker-image diff-image --name kind
	cat kindify/deploy.yaml | sed "s|replace-me|$(target_branch)|g" | kubectl apply -f -
	sleep 5
	kubectl logs -f job/diff-preview
