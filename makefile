gitops_repo ?= argocd-diff-preview
github_org ?= dag-andersen
base_branch := main
docker_file := Dockerfile
argocd_namespace := argocd-diff-preview
timeout := 60
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
use_argocd_api ?= false

GO_TEST_FLAGS ?=

# Detect Docker API version if client is too new for server
DOCKER_API_VERSION ?= $(shell docker version 2>&1 | sed -n 's/.*Maximum supported API version is \([0-9.]*\).*/\1/p')
export DOCKER_API_VERSION

go-build:
	go build -ldflags="-X 'main.Version=$(VERSION)' -X 'main.Commit=$(COMMIT)' -X 'main.BuildDate=$(BUILD_DATE)'" -o bin/argocd-diff-preview ./cmd

pull-repository:
	@rm -rf base-branch || true && mkdir -p base-branch
	@rm -rf target-branch || true && mkdir -p target-branch
	cd base-branch   && git clone https://github.com/$(github_org)/$(gitops_repo).git --depth=1 --branch "$(base_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -
	cd target-branch && git clone https://github.com/$(github_org)/$(gitops_repo).git --depth=1 --branch "$(target_branch)" && cp -r $(gitops_repo)/. . && rm -rf .git && echo "*" > .gitignore && rm -rf $(gitops_repo) && cd -

docker-build:
	docker build . -f $(docker_file) -t image

docker-build-release:
	docker build . -f $(docker_file) -t image --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE)

run-with-go: go-build pull-repository
	./bin/argocd-diff-preview \
		--base-branch="$(base_branch)" \
		--target-branch="$(target_branch)" \
		--repo="$(github_org)/$(gitops_repo)" \
		--keep-cluster-alive \
		--file-regex="$(regex)" \
		--diff-ignore="$(diff_ignore)" \
		--timeout=$(timeout) \
		--selector="$(selector)" \
		--argocd-namespace="$(argocd_namespace)" \
		--files-changed="$(files_changed)" \
		--line-count="$(line_count)" \
		--redirect-target-revisions="HEAD" \
		--use-argocd-api="$(use_argocd_api)"

run-with-docker: pull-repository docker-build
	docker rm argocd-diff-preview || true
	docker run \
		--name="argocd-diff-preview" \
		--network=host \
		-v ~/.kube:/root/.kube \
		-v /var/run/docker.sock:/var/run/docker.sock \
		$(if $(DOCKER_API_VERSION),-e DOCKER_API_VERSION=$(DOCKER_API_VERSION)) \
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
		-e ARGOCD_CHART_NAME="$(argocd_chart_name)" \
		-e ARGOCD_CHART_URL="$(argocd_chart_url)" \
		-e LINE_COUNT="$(line_count)" \
		-e MAX_DIFF_LENGTH="$(max_diff_length)" \
		image \
		--argocd-namespace="$(argocd_namespace)" \
		--use-argocd-api="$(use_argocd_api)"

mkdocs:
	python3 -m venv venv \
	&& source venv/bin/activate \
	&& pip3 install mkdocs-material \
	&& mkdocs serve --watch-theme --open

run-lint:
	golangci-lint run

run-unit-tests:
	go test $(GO_TEST_FLAGS) ./...
	go test $(GO_TEST_FLAGS) -race ./...
	go test $(GO_TEST_FLAGS) -cover ./...
# go test -coverprofile=coverage.out ./...
# go tool cover -html=coverage.out

run-integration-tests-docker:
	cd tests && $(MAKE) run-test-all-docker

run-integration-tests-go: go-build
	cd tests && $(MAKE) run-test-all-go

# Run before release	
check-release: run-lint run-unit-tests
	$(MAKE) run-integration-tests-go use_argocd_api=false
	$(MAKE) run-integration-tests-docker use_argocd_api=true

# Loop the above commands until one fails
check-release-repeat:
	@i=1; while true; do \
		echo "⭐⭐⭐⭐⭐ Iteration $$i ⭐⭐⭐⭐⭐"; \
		$(MAKE) run-integration-tests-go use_argocd_api=false || exit 1; \
		$(MAKE) run-integration-tests-docker use_argocd_api=true || exit 1; \
		i=$$((i + 1)); \
	done
