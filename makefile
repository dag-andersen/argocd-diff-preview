gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main
docker_file := Dockerfile_ARM64
timeout := 120

pull-repostory:
	@rm -rf base-branch || true && mkdir -p base-branch
	@rm -rf target-branch || true && mkdir -p target-branch
	cd base-branch   && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(base_branch)"   && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -
	cd target-branch && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

local-test-cargo: pull-repostory
	cargo run -- -b "$(base_branch)" \
		-t "$(target_branch)" \
		--repo $(github_org)/$(gitops_repo) \
		--debug  \
		-r "$(regex)" \
		--diff-ignore "$(diff-ignore)" \
		--timeout $(timeout) \
		-l "$(selector)" \
		--files-changed="$(files-changed)"

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
		-e BASE_BRANCH=$(base_branch) \
		-e TARGET_BRANCH=$(target_branch) \
		-e REPO=$(github_org)/$(gitops_repo) \
		-e FILE_REGEX="$(regex)" \
		-e DIFF_IGNORE="$(diff-ignore)" \
		-e TIMEOUT=$(timeout) \
		image
