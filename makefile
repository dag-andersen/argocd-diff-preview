gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main
docker_file := Dockerfile
argocd_namespace := argocd-diff-preview
timeout := 120

pull-repository:
	@rm -rf base-branch || true && mkdir -p base-branch
	@rm -rf target-branch || true && mkdir -p target-branch
	cd base-branch   && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(base_branch)"   && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -
	cd target-branch && gh repo clone $(github_org)/$(gitops_repo) -- --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

docker-build:
	docker build . -f $(docker_file) -t image

run-with-cargo: pull-repository
	cargo run -- -b "$(base_branch)" \
		-t "$(target_branch)" \
		--repo $(github_org)/$(gitops_repo) \
		--debug \
		--keep-cluster-alive \
		-r "$(regex)" \
		--diff-ignore "$(diff_ignore)" \
		--timeout $(timeout) \
		-l "$(selector)" \
		--argocd-namespace "$(argocd_namespace)" \
		--files-changed="$(files_changed)" \
		--redirect-target-revisions="HEAD"

run-with-docker: pull-repository docker-build
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
		-e DIFF_IGNORE="$(diff_ignore)" \
		-e TIMEOUT=$(timeout) \
		-e SELECTOR="$(selector)" \
		-e FILES_CHANGED="$(files_changed)" \
		image

mkdocs:
	pip install mkdocs-material \
	&& python3 -m venv venv \
	&& source venv/bin/activate \
	&& open http://localhost:8000 \
	&& mkdocs serve
