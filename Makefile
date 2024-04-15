
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
TEST_OPTIONS=-race
BUILD_TAGS=-tags=fdo

GOLANGCI_LINT_COMMON_OPTIONS=\
			--enable=errcheck,gocritic,gofmt,goimports,gosec,gosimple,govet,ineffassign,revive,staticcheck,typecheck,unused,bodyclose \
			--fix=false \
			--go=1.20 \
			--max-same-issues=20 \
			--print-issued-lines=true \
			--print-linter-name=true \
			--sort-results=true \
			--timeout=5m0s \
			--uniq-by-line=false

EXCLUDE_DIRS=-e /test/ -e /cmd/db -e /cmd/kafka \
				-e /pkg/clients/imagebuilder/mock_imagebuilder \
				-e /pkg/imagebuilder/mock_imagebuilder \
				-e /pkg/clients/inventory/mock_inventory \
				-e /pkg/errors -e /pkg/services/mock_services -e /unleash \
				-e /api

CONTAINERFILE_NAME=Dockerfile

.PHONY:	all bonfire-config-local bonfire-config-github build-containers \
               build build-edge-api-container clean coverage coverage-html coverage-no-fdo \
               create-ns deploy-app deploy-env fmt generate-docs help lint pre-commit \
               restart-app scale-down scale-up scan_project test test-clean-no-fdo \
               test-no-fdo vet vet-no-fdo

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

build:
	mkdir -p build 2>/dev/null
	go build -o build/edge-api .
	go build -o build/edge-api-migrate cmd/migrate/main.go
	go build -o build/edge-api-wipe cmd/db/wipe.go
	go build -o build/edge-api-migrate-device cmd/db/updDb/set_account_on_device.go
	go build -o build/edge-api-migrate-repositories cmd/migraterepos/main.go
	go build -o build/edge-api-migrate-groups cmd/migrategroups/main.go
	go build -o build/edge-api-ibvents cmd/kafka/main.go
	go build -o build/edge-api-images-build pkg/services/images_build/main.go
	go build -o build/edge-api-isos-build pkg/services/images_iso/main.go
	go build -o build/edge-api-cleanup cmd/cleanup/main.go

clean:
	rm -rf build
	golangci-lint cache clean

coverage:
	go test $(BUILD_TAGS) $$(go list $(BUILD_TAGS) ./... | grep -v $(EXCLUDE_DIRS)) $(TEST_OPTIONS) -coverprofile=coverage.txt -covermode=atomic

coverage-output-no-fdo:
	go test $$(go list ./... | grep -v $(EXCLUDE_DIRS)) $(TEST_OPTIONS) -coverprofile=coverage.txt -covermode=atomic

coverage-html:
	go tool cover -html=coverage.txt -o coverage.html

coverage-no-fdo:
	go test $$(go list ./... | grep -v $(EXCLUDE_DIRS)) $(TEST_OPTIONS) -coverprofile=coverage.txt -covermode=atomic -json > coverage.json

create-ns:
	$(KUBECTL) create ns $(NAMESPACE)

deploy-app:
	bonfire deploy edge -n $(NAMESPACE)

deploy-env:
	bonfire deploy-env -n $(NAMESPACE)

fmt:
	go fmt $$(go list ./... | grep -v /vendor/)

generate-docs:
	~/go/bin/swag init --generalInfo api.go --o ./cmd/spec/ --dir pkg/models,pkg/routes --parseDependency
	go run ./cmd/swagger2openapi/main.go  cmd/spec/swagger.json cmd/spec/openapi.json

help:
	@echo "Please use \`make <target>' where <target> is one of:"
	@echo ""
	@echo "--- General Commands ---"
	@echo "bonfire-config-local      Create bonfire config for deploying from your local repository"
	@echo "bonfire-config-github     Create bonfire config for deploying from the github repository"
	@echo "build-containers          Builds all the container images"
	@echo "build-edge-api-container  Builds the edge-api container"
	@echo "build-test-container      Builds the test container"
	@echo "build                     Build all binaries into ./build"
	@echo "clean                     Removes binaries and cached golangci files"
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
	@echo "golangci-lint			 Runs 'golangci-lint' on the project"
	@echo "openapi                   Generates an openapi.{json,yaml} file in /cmd/spec/"
	@echo "pre-commit                Runs fmt, vet, lint, and clean on the project"
	@echo "restart-app				 Scales the edge-api-service deployment down to 0 then up to 1 in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "scale-down				 Scales the edge-api-service deployment down to 0 in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "scale-up					 Scales the edge-api-service deployment up to 1 in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "scan_project              Run security scan"
	@echo "swaggo                    Runs swaggo/swag and converts to openapi.json in /api"
	@echo "swaggo_setup"             Installs necessary packages to use swaggo
	@echo "test                      Runs 'go test' on the project"
	@echo "test-clean-no-fdo         Runs 'go test' on the project without FDO"
	@echo "test-no-fdo               Runs 'go test' on the project without FDO"
	@echo "vet                       Runs 'go vet' on the project"
	@echo "vet-no-fdo                Runs 'go vet' on the project without FDO"
	@echo ""

golangci-lint:
	if [ "$(GITHUB_ACTION)" != '' ];\
	then\
		OUT_FORMAT="--out-format=line-number";\
		TARGET_FILES=$$(go list $(BUILD_TAGS) ./... | grep -v /vendor/);\
    else\
		OUT_FORMAT="--out-format=colored-line-number";\
		TARGET_FILES=$$(go list -f '{{.Dir}}' ${BUILD_TAGS} ./... | grep -v '/vendor/');\
	fi;\
    golangci-lint run $(GOLANGCI_LINT_COMMON_OPTIONS) $(OUT_FORMAT) \
			$(TARGET_FILES)

golangci-lint-no-fdo:
	if [ "$(GITHUB_ACTION)" != '' ]; \
    then \
		OUT_FORMAT="--out-format=line-number"; \
		TARGET_FILES=$$(go list ./... | grep -v /vendor/);\
    else \
		OUT_FORMAT="--out-format=colored-line-number"; \
		TARGET_FILES=$$(go list -f '{{.Dir}}' ./... | grep -v '/vendor/');\
	fi;\
    golangci-lint run $(GOLANGCI_LINT_COMMON_OPTIONS) $(OUT_FORMAT) \
			$(TARGET_FILES)

lint:
	golint $$(go list $(BUILD_TAGS) ./... | grep -v /vendor/)

openapi:
	go run cmd/spec/main.go

pre-commit:
	$(MAKE) golangci-lint-no-fdo
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

swaggo_setup:
	go install github.com/swaggo/swag/cmd/swag@latest

test:
	go test $(BUILD_TAGS) $$(go list $(BUILD_TAGS) ./... | grep -v /test/) $(TEST_OPTIONS)

test-clean-no-fdo:
	go test -count=1 $$(go list ./... | grep -v /test/) $(TEST_OPTIONS)

test-no-fdo:
	go test $$(go list ./... | grep -v /test/) $(TEST_OPTIONS)

test_gha:
	go test ./...

vet:
	go mod tidy
	go install -a
	go vet $(BUILD_TAGS) $$(go list $(BUILD_TAGS) ./... | grep -v /vendor/)

vet-no-fdo:
	go vet $$(go list ./... | grep -v /vendor/)

pkg/services/mock_services/downloader.go: pkg/services/files/downloader.go
	mockgen -source=$< -destination=$@ -package=mock_services

pkg/services/mock_services/updates.go: pkg/services/updates.go
	mockgen -source=$< -destination=$@

pkg/services/mock_services/thirdpartyrepo.go: pkg/services/thirdpartyrepo.go
	mockgen -source=$< -destination=$@

pkg/services/mock_services/repobuilder.go: pkg/services/repobuilder.go
	mockgen -source=$< -destination=$@

pkg/services/mock_services/imagesets.go: pkg/services/imagesets.go
	mockgen -source=$< -destination=$@

pkg/services/mock_services/images.go: pkg/services/images.go
	mockgen -source=$< -destination=$@

pkg/services/mock_services/devices.go: pkg/services/devices.go
	mockgen -source=$< -destination=$@

pkg/services/mock_services/commits.go: pkg/services/commits.go
	mockgen -source=$< -destination=$@

pkg/services/mock_services/repo.go: pkg/services/repo.go
	mockgen -source=$< -destination=$@

pkg/services/mock_files/uploader.go: pkg/services/files/uploader.go
	mockgen -source=$< -destination=$@

# is a copy of the above, before this make target it was a manually created mess
pkg/services/mock_services/uploader.go: pkg/services/files/uploader.go
	mockgen -source=$< -destination=$@ -package=mock_services

pkg/services/mock_services/files.go: pkg/services/files.go
	mockgen -source=$< -destination=$@

pkg/services/mock_files/s3.go: pkg/services/files/s3.go
	mockgen -source=$< -destination=$@

pkg/services/mock_services/devicegroups.go: pkg/services/devicegroups.go
	mockgen -source=$< -destination=$@

pkg/services/mock_files/extrator.go: pkg/services/files/extractor.go
	mockgen -source=$< -destination=$@

pkg/common/kafka/mock_kafka/mock_topics.go: pkg/common/kafka/topics.go
	mockgen -source=$< -destination=$@

pkg/common/kafka/mock_kafka/mock_producer.go: pkg/common/kafka/producer.go
	mockgen -source=$< -destination=$@

pkg/common/kafka/mock_kafka/mock_kafkaconfigmap.go: pkg/common/kafka/kafkaconfigmap.go
	mockgen -source=$< -destination=$@

pkg/common/kafka/mock_kafka/mock_consumer.go: pkg/common/kafka/consumer.go
	mockgen -source=$< -destination=$@

pkg/clients/repositories/mock_repositories/client.go: pkg/clients/repositories/client.go
	mockgen -source=$< -destination=$@

pkg/clients/playbookdispatcher/mock_playbookdispatcher/playbookdispatcher.go: pkg/clients/playbookdispatcher/client.go
	mockgen -source=$< -destination=$@

pkg/clients/rbac/mock_rbac/client.go: pkg/clients/rbac/client.go
	mockgen -source=$< -destination=$@

pkg/clients/inventory/mock_inventory/inventory.go: pkg/clients/inventory/client.go
	mockgen -source=$< -destination=$@

pkg/clients/inventorygroups/mock_inventorygroups/client.go: pkg/clients/inventorygroups/client.go
	mockgen -source=$< -destination=$@

pkg/clients/imagebuilder/mock_imagebuilder/client.go: pkg/clients/imagebuilder/client.go
	mockgen -source=$< -destination=$@

mockgen: \
	pkg/services/mock_services/downloader.go \
	pkg/services/mock_services/updates.go \
	pkg/services/mock_services/thirdpartyrepo.go \
	pkg/services/mock_services/repobuilder.go \
	pkg/services/mock_services/imagesets.go \
	pkg/services/mock_services/images.go \
	pkg/services/mock_services/devices.go \
	pkg/services/mock_services/commits.go \
	pkg/services/mock_services/uploader.go \
	pkg/services/mock_services/repo.go \
	pkg/services/mock_files/uploader.go \
	pkg/services/mock_services/files.go \
	pkg/services/mock_files/s3.go \
	pkg/services/mock_services/devicegroups.go \
	pkg/services/mock_files/extrator.go \
	pkg/common/kafka/mock_kafka/mock_topics.go \
	pkg/common/kafka/mock_kafka/mock_producer.go \
	pkg/common/kafka/mock_kafka/mock_kafkaconfigmap.go \
	pkg/common/kafka/mock_kafka/mock_consumer.go \
	pkg/clients/repositories/mock_repositories/client.go \
	pkg/clients/playbookdispatcher/mock_playbookdispatcher/playbookdispatcher.go \
	pkg/clients/rbac/mock_rbac/client.go \
	pkg/clients/inventory/mock_inventory/inventory.go \
	pkg/clients/inventorygroups/mock_inventorygroups/client.go \
	pkg/clients/imagebuilder/mock_imagebuilder/client.go
