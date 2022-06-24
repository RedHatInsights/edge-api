
OS := $(shell uname)
UNAME_S := $(shell uname -s)
OS_SED :=
ifeq ($(UNAME_S),Darwin)
	OS_SED += ""
endif

OCI_TOOL=$(shell command -v podman || command -v docker)

# Match logic in build_deploy.sh
IMAGE_TAG=$(shell git rev-parse --short=7 HEAD)

EDGE_API_CONTAINER_TAG="quay.io/cloudservices/edge-api:$(IMAGE_TAG)"

TEST_CONTAINER_TAG="quay.io/fleet-management/libfdo-data:$(IMAGE_TAG)"

KUBECTL=kubectl
NAMESPACE=default
TEST_OPTIONS="-race"
BUILD_TAGS=-tags=fdo

CONTAINERFILE_NAME=Dockerfile

.PHONY: all build-containers clean help test

bonfire-config-local:
	@cp default_config.yaml.local.example config.yaml
	@sed -i $(OS_SED) 's|REPO|$(PWD)|g' config.yaml

bonfire-config-github:
	@cp default_config.yaml.github.example config.yaml

build-containers: build-edge-api-container build-test-container

build-edge-api-container:
	$(OCI_TOOL) build \
		--file "$(CONTAINERFILE_NAME)" \
		--no-cache \
		--tag "$(EDGE_API_CONTAINER_TAG)" \
		.

build-test-container:
	cd test-container;	\
	$(OCI_TOOL) build \
		--file "$(CONTAINERFILE_NAME)" \
		--no-cache \
		--tag "$(TEST_CONTAINER_TAG)" \
		.

coverage:
	go test $(BUILD_TAGS) $$(go list $(BUILD_TAGS) ./... | grep -v /test/) $(TEST_OPTIONS) -coverprofile=coverage.txt -covermode=atomic

coverage-html:
	go tool cover -html=coverage.txt -o coverage.html

coverage-no-fdo:
	go test $$(go list ./... | grep -v /test/) $(TEST_OPTIONS) -coverprofile=coverage.txt -covermode=atomic

create-ns:
	$(KUBECTL) create ns $(NAMESPACE)

deploy-app:
	bonfire deploy edge -n $(NAMESPACE)

deploy-env:
	bonfire deploy-env -n $(NAMESPACE)

fmt:
	go fmt $$(go list ./... | grep -v /vendor/)

generate-docs:
	go run cmd/spec/main.go

help:
	@echo "Please use \`make <target>' where <target> is one of:"
	@echo ""
	@echo "--- General Commands ---"
	@echo "bonfire-config-local      Create bonfire config for deploying from your local repository"
	@echo "bonfire-config-github     Create bonfire config for deploying from the github repository"
	@echo "build-containers          Builds all the container images"
	@echo "build-edge-api-container  Builds the edge-api container"
	@echo "build-test-container      Builds the test container"
	@echo "coverage                  Runs 'go test' coverage on the project"
	@echo "coverage-html             Create HTML version of coverage report"
	@echo "coverage-no-fdo           Runs 'go test' coverage on the project without FDO"
	@echo "create-ns                 Creates a namespace in kubernetes"
	@echo "                             @param NAMESPACE - (optional) the namespace to use"
	@echo "deploy-app				 Deploys the edge app in the given namespace"
	@echo "                             @param NAMESPACE - (optional) the namespace to use"
	@echo "deploy-env				 Creates a ClowdEnvironment in the given namespace"
	@echo "                             @param NAMESPACE - (optional) the namespace to use"
	@echo "fmt                       Runs 'go fmt' on the project"
	@echo "generate-docs             Creates OpenAPI specification for the project"
	@echo "help                      Show this message"
	@echo "lint                      Runs 'golint' on the project"
	@echo "pre-commit                Runs fmt, vet, lint, and clean on the project"
	@echo "restart-app				 Scales the edge-api-service deployment down to 0 then up to 1 in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "scale-down				 Scales the edge-api-service deployment down to 0 in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "scale-up					 Scales the edge-api-service deployment up to 1 in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "scan_project              Run security scan"
	@echo "test                      Runs 'go test' on the project"
	@echo "test-clean-no-fdo         Runs 'go test' on the project without FDO"
	@echo "test-no-fdo               Runs 'go test' on the project without FDO"
	@echo "vet                       Runs 'go vet' on the project"
	@echo "vet-no-fdo                Runs 'go vet' on the project without FDO"
	@echo ""

lint:
	golint $$(go list $(BUILD_TAGS) ./... | grep -v /vendor/)

pre-commit:
	$(MAKE) fmt
	$(MAKE) vet-no-fdo
	$(MAKE) lint
	$(MAKE) test-clean-no-fdo

restart-app:
	$(MAKE) scale-down NAMESPACE=$(NAMESPACE)
	sleep 5
	$(MAKE) scale-up NAMESPACE=$(NAMESPACE)

scale-down:
	$(KUBECTL) scale --replicas=0 deployment/edge-api-service -n $(NAMESPACE)

scale-up:
	$(KUBECTL) scale --replicas=1 deployment/edge-api-service -n $(NAMESPACE)

scan_project:
	./sonarqube.sh

test:
	go test $(BUILD_TAGS) $$(go list $(BUILD_TAGS) ./... | grep -v /test/) $(TEST_OPTIONS)

test-clean-no-fdo:
	go test -count=1 $$(go list ./... | grep -v /test/) $(TEST_OPTIONS)

test-no-fdo:
	go test $$(go list ./... | grep -v /test/) $(TEST_OPTIONS)

vet:
	go vet $(BUILD_TAGS) $$(go list $(BUILD_TAGS) ./... | grep -v /vendor/)

vet-no-fdo:
	go vet $$(go list ./... | grep -v /vendor/)
