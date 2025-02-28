gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main
docker_file := Dockerfile
argocd_namespace := argocd
timeout := 120

pull-base:
	@rm -rf base-branch || true && mkdir -p base-branch
	cd base-branch   && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(base_branch)"   && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

pull-target:
	@rm -rf target-branch || true && mkdir -p target-branch
	cd target-branch && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

run-single-test: pull-base pull-target
	cargo run -- -b "$(base_branch)" \
		-t "$(target_branch)" \
		--repo $(github_org)/$(gitops_repo) \
		--argocd-namespace "$(argocd_namespace)" \
		--files-changed="$(files_changed)"
	diff ./output/diff.md ./$(target_branch)$(suffix)/output.md

docker-build:
	docker build -f ../$(docker_file) -t argocd-diff-preview ..

run-single-test-docker: pull-base pull-target
	docker run --network=host \
		-v ~/.kube:/root/.kube \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD)/base-branch:/base-branch \
		-v $(PWD)/target-branch:/target-branch \
		-v $(PWD)/output:/output \
		-v $(PWD)/secrets:/secrets \
		-e BASE_BRANCH=$(base_branch) \
		-e TARGET_BRANCH=$(target_branch) \
		-e REPO=$(github_org)/$(gitops_repo) \
		-e ARGOCD_NAMESPACE=$(argocd_namespace) \
		-e FILES_CHANGED=$(files_changed) \
		argocd-diff-preview
	diff ./output/diff.md ./$(target_branch)$(suffix)/output.md

run-test-all:
	$(MAKE) run-single-test target_branch=tool-test-branch-1 base_branch=tool-test-branch-1-og
	$(MAKE) run-single-test target_branch=tool-test-branch-2 base_branch=tool-test-branch-2-og
	$(MAKE) run-single-test target_branch=tool-test-branch-3 base_branch=tool-test-branch-3-og
	$(MAKE) run-single-test target_branch=tool-test-branch-4 base_branch=tool-test-branch-4-og
	$(MAKE) run-single-test target_branch=tool-test-branch-5 base_branch=tool-test-branch-5-og files_changed="examples/helm/values/filtered.yaml"
	$(MAKE) run-single-test target_branch=tool-test-branch-5 base_branch=tool-test-branch-5-og files_changed="something/else.yaml" suffix="-2"

run-test-all-docker: docker-build
	$(MAKE) run-single-test-docker target_branch=tool-test-branch-1 base_branch=tool-test-branch-1-og
	$(MAKE) run-single-test-docker target_branch=tool-test-branch-2 base_branch=tool-test-branch-2-og
	$(MAKE) run-single-test-docker target_branch=tool-test-branch-3 base_branch=tool-test-branch-3-og
	$(MAKE) run-single-test-docker target_branch=tool-test-branch-3 base_branch=tool-test-branch-4-og
	$(MAKE) run-single-test-docker target_branch=tool-test-branch-5 base_branch=tool-test-branch-5-og files_changed="examples/helm/values/filtered.yaml"
	$(MAKE) run-single-test-docker target_branch=tool-test-branch-5 base_branch=tool-test-branch-5-og files_changed="something/else.yaml" suffix="-2"
