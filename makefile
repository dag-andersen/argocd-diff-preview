gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main

local-test-cargo:
	# Pull repos
	@rm -rf base-branch || true && mkdir -p base-branch
	@rm -rf target-branch || true && mkdir -p target-branch
	cd base-branch && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(base_branch)" && cp -r $(gitops_repo)/. base-branch && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -
	cd target-branch && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. target-branch && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

	cargo run -- -b "$(base_branch)" -t "$(target_branch)" -g "https://github.com/$(github_org)/$(gitops_repo).git" -r "$(regex)" --debug

local-test-docker:
	# Pull repos
	@rm -rf "$(base_branch)" || true && mkdir -p "$(base_branch)"
	@rm -rf "$(target_branch)" || true && mkdir -p "$(target_branch)"
	cd "$(base_branch)"   && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(base_branch)"   && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -
	cd "$(target_branch)" && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

	docker build . -f Dockerfile_ARM64 -t image-arm64
	docker run \
		--network=host \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD)/$(base_branch):/base-branch \
		-v $(PWD)/$(target_branch):/target-branch \
		-v $(PWD)/output:/output \
		-e BASE_BRANCH=$(base_branch) \
		-e TARGET_BRANCH=$(target_branch) \
		-e GIT_REPO="https://github.com/$(github_org)/$(gitops_repo).git" \
		-e FILE_REGEX=$(regex) \
		image-arm64
