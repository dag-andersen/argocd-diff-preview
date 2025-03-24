gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main
docker_file := Dockerfile
argocd_namespace := argocd-diff-preview
timeout := 120
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

go-build:
	go build -ldflags="-X 'main.Version=$(VERSION)' -X 'main.Commit=$(COMMIT)' -X 'main.BuildDate=$(BUILD_DATE)'" -o bin/argocd-diff-preview ./cmd

pull-repository:
	@rm -rf base-branch || true && mkdir -p base-branch
	@rm -rf target-branch || true && mkdir -p target-branch
	cd base-branch   && git clone https://github.com/$(github_org)/$(gitops_repo).git --depth=1 --branch "$(base_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -
	cd target-branch && git clone https://github.com/$(github_org)/$(gitops_repo).git --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

docker-build:
	docker build . -f $(docker_file) -t image --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE)

run-with-go: go-build pull-repository
	./bin/argocd-diff-preview \
		--base-branch="$(base_branch)" \
		--target-branch="$(target_branch)" \
		--repo="$(github_org)/$(gitops_repo)" \
		--debug \
		--keep-cluster-alive \
		--file-regex="$(regex)" \
		--diff-ignore="$(diff_ignore)" \
		--timeout=$(timeout) \
		--selector="$(selector)" \
		--argocd-namespace="$(argocd_namespace)" \
		--files-changed="$(files_changed)" \
		--line-count="$(line_count)" \
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
		-e LINE_COUNT="$(line_count)" \
		-e MAX_DIFF_LENGTH="$(max_diff_length)" \
		image \
		--argocd-namespace="$(argocd_namespace)" --debug

mkdocs:
	python3 -m venv venv \
	&& source venv/bin/activate \
	&& pip3 install mkdocs-material \
	&& open http://localhost:8000 \
	&& mkdocs serve

run-unit-tests:
	go test ./...
	go test -race ./...
	go test -cover ./...
# go test -coverprofile=coverage.out ./...
# go tool cover -html=coverage.out

run-integration-tests-docker:
	cd tests && $(MAKE) run-test-all-docker

run-integration-tests-go: go-build
	cd tests && $(MAKE) run-test-all-go
