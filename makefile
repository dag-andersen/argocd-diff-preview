gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main
docker_file := Dockerfile_ARM64
timeout := 120

pull-repostory:
	@rm -rf base-branch || true && mkdir -p base-branch
	@rm -rf target-branch || true && mkdir -p target-branch
	cd base-branch   && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(base_branch)"   && cp -r $(gitops_repo)/. . && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -
	cd target-branch && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

local-test-cargo: pull-repostory
	cargo run -- -r "$(regex)" --debug --diff-ignore "$(diff-ignore)" --timeout $(timeout) -l "$(selector)"

local-test-docker: pull-repostory
	docker build . -f $(docker_file) -t image
	docker run \
		--network=host \
		-v ~/.kube:/root/.kube \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD)/base-branch:/base-branch \
		-v $(PWD)/target-branch:/target-branch \
		-v $(PWD)/output:/output \
		-v $(PWD)/secrets:/secrets \
		-e FILE_REGEX="$(regex)" \
		-e DIFF_IGNORE="$(diff-ignore)" \
		-e TIMEOUT=$(timeout) \
		image
