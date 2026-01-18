gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main
docker_file := Dockerfile
argocd_namespace := argocd-diff-preview
timeout := 90
line_count := 10
title := Argo CD Diff Preview
create_cluster := false
max_diff_length := 65536
ignore_invalid_watch_pattern := false
watch_if_no_watch_pattern_found := false
auto_detect_files_changed := false
hide_deleted_app_diff := false
use_argocd_api ?= false
update_expected ?= false

# Detect Docker API version if client is too new for server
DOCKER_API_VERSION ?= $(shell docker version 2>&1 | sed -n 's/.*Maximum supported API version is \([0-9.]*\).*/\1/p')
export DOCKER_API_VERSION

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
		--hide-deleted-app-diff="$(hide_deleted_app_diff)" \
		--watch-if-no-watch-pattern-found="$(watch_if_no_watch_pattern_found)" \
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
		--ignore-invalid-watch-pattern="$(ignore_invalid_watch_pattern)" \
		--keep-cluster-alive \
		--auto-detect-files-changed="$(auto_detect_files_changed)" \
		--ignore-resources="$(ignore_resources)" \
		--argocd-login-options="$(argocd_login_options)" \
		--use-argocd-api="$(use_argocd_api)"
	mkdir -p ./$(target_branch)$(suffix)/
	cat ./output/diff.md | sed 's/[0-9][0-9m]*s\]/Xs]/g' > ./output/diff-2.md
	cat ./output/diff.html | sed 's/[0-9][0-9m]*s\]/Xs]/g' > ./output/diff-2.html
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
	docker rm argocd-diff-preview || true
ifeq ($(use_argocd_api),true)
	$(eval ARGOCD_CONFIG_VOLUME := -v $(PWD)/createClusterRoles.yaml:/argocd-config/values.yaml)
else
	$(eval ARGOCD_CONFIG_VOLUME :=)
endif
	docker run --network=host \
		--name="argocd-diff-preview" \
		-v ~/.kube:/root/.kube \
		-v /var/run/docker.sock:/var/run/docker.sock \
		$(if $(DOCKER_API_VERSION),-e DOCKER_API_VERSION=$(DOCKER_API_VERSION)) \
		-v $(PWD)/base-branch:/base-branch \
		-v $(PWD)/target-branch:/target-branch \
		-v $(PWD)/output:/output \
		-v $(PWD)/secrets:/secrets \
		-v $(PWD)/temp:/temp \
		-v $(PWD)/kind-config:/kind-config \
		$(ARGOCD_CONFIG_VOLUME) \
		-e BASE_BRANCH="$(base_branch)" \
		-e TARGET_BRANCH="$(target_branch)" \
		-e FILE_REGEX="$(file_regex)" \
		-e DIFF_IGNORE="$(diff_ignore)" \
		-e LINE_COUNT="$(line_count)" \
		-e REPO="$(github_org)/$(gitops_repo)" \
		-e WATCH_IF_NO_WATCH_PATTERN_FOUND="$(watch_if_no_watch_pattern_found)" \
		-e AUTO_DETECT_FILES_CHANGED="$(auto_detect_files_changed)" \
		-e IGNORE_INVALID_WATCH_PATTERN="$(ignore_invalid_watch_pattern)" \
		-e HIDE_DELETED_APP_DIFF="$(hide_deleted_app_diff)" \
		-e ARGOCD_NAMESPACE="$(argocd_namespace)" \
		-e FILES_CHANGED="$(files_changed)" \
		-e SELECTOR="$(selector)" \
		-e TIMEOUT="$(timeout)" \
		-e KIND_OPTIONS="$(kind_options)" \
		-e TITLE="$(title)" \
		-e CREATE_CLUSTER="$(create_cluster)" \
		-e MAX_DIFF_LENGTH="$(max_diff_length)" \
		-e USE_ARGOCD_API="$(use_argocd_api)" \
		-e KEEP_CLUSTER_ALIVE=true \
		-e IGNORE_RESOURCES="$(ignore_resources)" \
		-e ARGOCD_LOGIN_OPTIONS="$(argocd_login_options)" \
		argocd-diff-preview
	mkdir -p ./$(target_branch)$(suffix)/
	cat ./output/diff.md | sed 's/[0-9][0-9m]*s\]/Xs]/g' > ./output/diff-2.md
	cat ./output/diff.html | sed 's/[0-9][0-9m]*s\]/Xs]/g' > ./output/diff-2.html
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
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-1/target  base_branch=integration-test/branch-1/base  suffix="-1" line_count=3 kind_options="--name tests --config=./kind-config/options.yaml" create_cluster=true
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-1/target  base_branch=integration-test/branch-1/base  suffix="-2" diff_ignore="image"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-1/target  base_branch=integration-test/branch-1/base  suffix="-3" hide_deleted_app_diff="true" argocd_login_options="--insecure"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-2/target  base_branch=integration-test/branch-2/base
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-3/target  base_branch=integration-test/branch-3/base
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-4/target  base_branch=integration-test/branch-4/base  title="integration-test/branch-4"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target  base_branch=integration-test/branch-5/base  suffix="-1" files_changed="examples/helm/values/filtered.yaml"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target  base_branch=integration-test/branch-5/base  suffix="-2" files_changed="examples/helm/applications/watch-pattern/valid-regex.yaml"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target  base_branch=integration-test/branch-5/base  suffix="-3" files_changed="something/else.yaml"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target  base_branch=integration-test/branch-5/base  suffix="-4" selector="team=my-team"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target  base_branch=integration-test/branch-5/base  suffix="-5" selector="team=other-team" title="integration-test/branch-5"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target  base_branch=integration-test/branch-5/base  suffix="-6" file_regex=".*labels\.yaml"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target  base_branch=integration-test/branch-5/base  suffix="-7" file_regex="this-does-not-exist\.yaml"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target  base_branch=integration-test/branch-5/base  suffix="-8" watch_if_no_watch_pattern_found="true" files_changed="something/else.yaml"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-5/target  base_branch=integration-test/branch-5/base  suffix="-9" watch_if_no_watch_pattern_found="true" auto_detect_files_changed="true" use_argocd_api="true"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-6/target  base_branch=integration-test/branch-6/base
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-7/target  base_branch=integration-test/branch-7/base  files_changed="examples/helm/values/filtered.yaml" create_cluster=true
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-8/target  base_branch=integration-test/branch-8/base  files_changed="examples/git-generator/resources/folder2/deployment.yaml,examples/git-generator/resources/folder3/deployment.yaml"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-9/target  base_branch=integration-test/branch-9/base  suffix="-1" max_diff_length=10000
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-9/target  base_branch=integration-test/branch-9/base  suffix="-2" files_changed="examples/external-chart/nginx.yaml" max_diff_length=900
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-10/target base_branch=integration-test/branch-10/base suffix="-1" files_changed="examples/ignore-differences/app.yaml"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-11/target base_branch=integration-test/branch-11/base suffix="-1" auto_detect_files_changed="true"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-12/target base_branch=integration-test/branch-12/base suffix="-1" auto_detect_files_changed="true" diff_ignore="annotations"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-12/target base_branch=integration-test/branch-12/base suffix="-2" auto_detect_files_changed="true" diff_ignore="annotations" ignore_resources="*:CustomResourceDefinition:*,:ConfigMap:argocd-cm"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-13/target base_branch=integration-test/branch-13/base suffix="-1"
	$(MAKE) run-single-test-$* target_branch=integration-test/branch-13/target base_branch=integration-test/branch-13/base suffix="-2" selector="team=your-team"

run-test-all-docker: docker-build
	$(MAKE) run-test-all-with-docker

run-test-all-go:
	$(MAKE) run-test-all-with-go