gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main
docker_file := Dockerfile
argocd_namespace := argocd
timeout := 90
line_count := 10
title := Argo CD Diff Preview
create_cluster := false
max_diff_length := 65536
update_expected ?= false

pull-base:
	@rm -rf base-branch || true && mkdir -p base-branch
	cd base-branch   && git clone https://github.com/$(github_org)/$(gitops_repo).git --depth=1 --branch "$(base_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

pull-target:
	@rm -rf target-branch || true && mkdir -p target-branch
	cd target-branch && git clone https://github.com/$(github_org)/$(gitops_repo).git --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

clean:
	rm -rf base-branch || true
	rm -rf target-branch || true
	rm -rf output || true
	rm -rf temp || true

run-single-test-go: clean pull-base pull-target
	../bin/argocd-diff-preview \
		--base-branch "$(base_branch)" \
		--target-branch "$(target_branch)" \
		--repo $(github_org)/$(gitops_repo) \
		--argocd-namespace "$(argocd_namespace)" \
		--files-changed="$(files_changed)" \
		--selector="$(selector)" \
		--file-regex="$(file_regex)" \
		--diff-ignore="$(diff_ignore)" \
		--line-count="$(line_count)" \
		--timeout=$(timeout) \
		--kind-options="$(kind_options)" \
		--title="$(title)" \
		--create-cluster="$(create_cluster)" \
		--max-diff-length="$(max_diff_length)" \
		--keep-cluster-alive
	mkdir -p ./$(target_branch)$(suffix)/
	cat ./output/diff.md | sed 's/\[[^][]*\]/[]/g' > ./output/diff-2.md
	cat ./output/diff.html | sed 's/\[[^][]*\]/[]/g' > ./output/diff-2.html
ifeq ($(update_expected),true)
	mv ./output/diff-2.md ./$(target_branch)$(suffix)/output.md
	mv ./output/diff-2.html ./$(target_branch)$(suffix)/output.html
	@echo "UPDATED EXPECTED FILES: $(target_branch)$(suffix) 📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝"
else
	diff ./output/diff-2.md ./$(target_branch)$(suffix)/output.md
	diff ./output/diff-2.html ./$(target_branch)$(suffix)/output.html
	@echo "COMPLETED TEST: $(target_branch)$(suffix) ✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅"
endif

docker-build:
	docker build -f ../$(docker_file) -t argocd-diff-preview ..

run-single-test-docker: clean pull-base pull-target
	docker run --network=host \
		-v ~/.kube:/root/.kube \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD)/base-branch:/base-branch \
		-v $(PWD)/target-branch:/target-branch \
		-v $(PWD)/output:/output \
		-v $(PWD)/secrets:/secrets \
		-v $(PWD)/temp:/temp \
		-v $(PWD)/kind-config:/kind-config \
		-e BASE_BRANCH="$(base_branch)" \
		-e TARGET_BRANCH="$(target_branch)" \
		-e FILE_REGEX="$(file_regex)" \
		-e DIFF_IGNORE="$(diff_ignore)" \
		-e LINE_COUNT="$(line_count)" \
		-e REPO="$(github_org)/$(gitops_repo)" \
		-e ARGOCD_NAMESPACE="$(argocd_namespace)" \
		-e ARGOCD_CHART_NAME="$(argocd_chart_name)" \
		-e ARGOCD_CHART_URL="$(argocd_chart_url)" \
		-e FILES_CHANGED="$(files_changed)" \
		-e SELECTOR="$(selector)" \
		-e TIMEOUT="$(timeout)" \
		-e KIND_OPTIONS="$(kind_options)" \
		-e TITLE="$(title)" \
		-e CREATE_CLUSTER="$(create_cluster)" \
		-e MAX_DIFF_LENGTH="$(max_diff_length)" \
		-e KEEP_CLUSTER_ALIVE=true \
		argocd-diff-preview
	mkdir -p ./$(target_branch)$(suffix)/
	cat ./output/diff.md | sed 's/\[[^][]*\]/[]/g' > ./output/diff-2.md
	cat ./output/diff.html | sed 's/\[[^][]*\]/[]/g' > ./output/diff-2.html
ifeq ($(update_expected),true)
	mv ./output/diff-2.md ./$(target_branch)$(suffix)/output.md
	mv ./output/diff-2.html ./$(target_branch)$(suffix)/output.html
	@echo "UPDATED EXPECTED FILES: $(target_branch)$(suffix) 📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝📝"
else
	diff ./output/diff-2.md ./$(target_branch)$(suffix)/output.md
	diff ./output/diff-2.html ./$(target_branch)$(suffix)/output.html
	@echo "COMPLETED TEST: $(target_branch)$(suffix) ✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅✅"
endif

run-test-all-with-%:
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-1/target base_branch=integration-test/branch-1/base diff_ignore="image" line_count=3 kind_options="--name tests --config=./kind-config/options.yaml" create_cluster=true
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-2/target base_branch=integration-test/branch-2/base
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-3/target base_branch=integration-test/branch-3/base
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-4/target base_branch=integration-test/branch-4/base title="integration-test/branch-4"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target base_branch=integration-test/branch-5/base files_changed="examples/helm/values/filtered.yaml" suffix="-1"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target base_branch=integration-test/branch-5/base files_changed="examples/helm/applications/watch-pattern/valid-regex.yaml" suffix="-2"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target base_branch=integration-test/branch-5/base files_changed="something/else.yaml" suffix="-3"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target base_branch=integration-test/branch-5/base selector="team=my-team" suffix="-4"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target base_branch=integration-test/branch-5/base selector="team=other-team" suffix="-5" title="integration-test/branch-5"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target base_branch=integration-test/branch-5/base file_regex=".*labels\.yaml" suffix="-6"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target base_branch=integration-test/branch-5/base file_regex="this-does-not-exist\.yaml" suffix="-7"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-6/target base_branch=integration-test/branch-6/base
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-7/target base_branch=integration-test/branch-7/base files_changed="examples/helm/values/filtered.yaml" create_cluster=true
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-8/target base_branch=integration-test/branch-8/base files_changed="examples/git-generator/resources/folder2/deployment.yaml,examples/git-generator/resources/folder3/deployment.yaml"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-9/target base_branch=integration-test/branch-9/base max_diff_length=10000

run-test-all-docker: docker-build
	$(MAKE) run-test-all-with-docker

run-test-all-go:
	$(MAKE) run-test-all-with-go