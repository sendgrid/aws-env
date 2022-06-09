GO_VERSION ?= 1.18
GO_CI_VERSION = v1.46.1
BINARIES = aws-env
WD ?= $(shell pwd)
NAMESPACE=sendgrid
APPNAME=aws-env

export GIT_COMMIT ?= $(shell git rev-parse --verify HEAD)
export BUILD_DATE ?= $(shell date -u)
export VER_VERSION ?= 0.0.1
export BUILD_NUMBER ?= $(or $(BUILDKITE_BUILD_NUMBER),0)

GO_FILES = $(shell find . -type f -name "*.go")

all: test lint build

.PHONY: build
build: $(BINARIES)

$(BINARIES): $(GO_FILES)
	@echo "[$@]\n\tVersion: $(VER_VERSION)\n\tBuild Date: $(BUILD_DATE)\n\tGit Commit: $(GIT_COMMIT)"
	@go build -mod readonly -a -tags netgo \
		-ldflags '-w -X "main.version=$(VER_VERSION)" -X "main.builtAt=$(BUILD_DATE)" -X "main.gitHash=$(GIT_COMMIT)" -extldflags -static' \
		./cmd/$@

.PHONY: build-docker
build-docker:
	@docker build -t aws-env \
		--build-arg GIT_COMMIT \
		--build-arg BUILD_DATE \
		--build-arg VER_VERSION \
		--build-arg BUILD_NUMBER \
		.
	@docker tag aws-env docker.sendgrid.net/sendgrid/aws-env

.PHONY: push
push: 
	docker push docker.sendgrid.net/$(NAMESPACE)/$(APPNAME)

.PHONY: push-tagged
push-tagged:
	docker tag \
		"docker.sendgrid.net/$(NAMESPACE)/$(APPNAME)" \
		"docker.sendgrid.net/$(NAMESPACE)/$(APPNAME):$(VER_DOCKER_TAG)"

	docker push "docker.sendgrid.net/$(NAMESPACE)/$(APPNAME):$(VER_DOCKER_TAG)"

.PHONY: push-pre-tagged
push-pre-tagged:
	docker tag docker.sendgrid.net/$(NAMESPACE)/$(APPNAME) docker.sendgrid.net/$(NAMESPACE)/$(APPNAME):v0.0.0-alpha-${BUILD_NUMBER}
	docker push docker.sendgrid.net/$(NAMESPACE)/$(APPNAME):v0.0.0-alpha-${BUILD_NUMBER}

.PHONY: artifact
artifact: build-docker
	@docker run -v $(WD):/dist --rm aws-env cp /usr/local/bin/aws-env /dist/

.PHONY: clean
clean: 
	@rm -rf $(BINARIES) .image coverage.* 

.PHONY: test
test: coverage.txt
coverage.txt: $(GO_FILES)
	@docker run --rm \
		-v "$(WD):/code" \
		-w /code \
		"golang:$(GO_VERSION)" \
		sh -c "\
		go test -mod readonly -v -race -coverprofile=coverage.out ./... && \
		go tool cover -html=coverage.out -o coverage.html && \
		go tool cover -func=coverage.out | tail -n1 > coverage.txt"

.PHONY: report
report: coverage.txt
ifeq ($(BUILDKITE_PULL_REQUEST),)
	@echo "Not reporting coverage, not running in BuildKite"
else ifeq ($(BUILDKITE_PULL_REQUEST),false)
	@echo "Not reporting coverage, no PR"
else
	@echo "Reporting coverage"
	@docker run \
		-e GHI_TOKEN=$(OPSBOT_GITHUB_KEY) \
		docker.sendgrid.net/sendgrid/ghi \
		comment -m "**Code coverage result**: $(shell cat coverage.txt)" $(BUILDKITE_PULL_REQUEST) -- $(BUILDKITE_ORGANIZATION_SLUG)/$(BUILDKITE_PIPELINE_SLUG)
endif

.PHONY: lint
lint:
	@docker run --rm \
		-v $(WD):/code \
		-w /code \
		golang:$(GO_VERSION) \
		sh -c "\
		curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /go/bin $(GO_CI_VERSION) && \
		golangci-lint -v run --exclude-use-default=false --deadline 5m \
			-E golint \
			-E gosec \
			-E unconvert \
			-E unparam \
			-E gocyclo \
			-E misspell \
			-E gocritic \
			-E maligned"

.PHONY: release
release: 
ifeq ($(BUILDKITE_PULL_REQUEST),)
	@echo "Not releasing, not running in BuildKite"
else
	@docker run \
		-v $(WD):/code \
		buildkite/github-release \
		"$(VERSION)" "code/build/$(BINARIES)" \
			--commit main \
			--tag "$(VERSION)" \
			--github-repository "$(NAMESPACE)/$(APPNAME)" \
			--github-access-token "$(OPSBOT_GITHUB_KEY)"
endif
