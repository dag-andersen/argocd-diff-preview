gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main
docker_file := Dockerfile
argocd_namespace := argocd-diff-preview
timeout := 120

pull-base:
	@rm -rf base-branch || true && mkdir -p base-branch
	cd base-branch   && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(base_branch)"   && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

pull-target:
	@rm -rf target-branch || true && mkdir -p target-branch
	cd target-branch && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

run-single-test: pull-base
	$(MAKE) pull-target target_branch=$(target_branch)
	cargo run -- -b "$(base_branch)" \
		-t "$(target_branch)" \
		--repo $(github_org)/$(gitops_repo)
	diff ./output/diff.md ./$(target_branch)/output.md

run-test-all:
	$(MAKE) run-single-test target_branch=tool-test-branch-1 base_branch=tool-test-branch-1-og
	$(MAKE) run-single-test target_branch=tool-test-branch-2 base_branch=tool-test-branch-2-og
	$(MAKE) run-single-test target_branch=tool-test-branch-3 base_branch=tool-test-branch-3-og
