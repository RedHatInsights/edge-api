
OS := $(shell uname)
UNAME_S := $(shell uname -s)
OS_SED :=
ifeq ($(UNAME_S),Darwin)
	OS_SED += ""
endif

ifeq ($(OS),Darwin)
	CONTAINERER=docker
else
	CONTAINERER=podman
endif
CONTAINER_TAG="quay.io/cloudservices/edge-api"

KUBECTL=kubectl
NAMESPACE=default
TEST_OPTIONS="-race"

help:
	@echo "Please use \`make <target>' where <target> is one of:"
	@echo ""
	@echo "--- General Commands ---"
	@echo "help                     show this message"
	@echo "lint                     runs go lint on the project"
	@echo "vet                      runs go vet on the project"
	@echo "test                     runs go test on the project"
	@echo "build                    builds the container image"
	@echo "bonfire-config-local     create bonfire config for deploying from your local repository"
	@echo "bonfire-config-github    create bonfire config for deploying from the github repository"
	@echo "create-ns                creates a namespace in kubernetes"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "deploy-env				creates a ClowdEnvironment in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "deploy-app				deploys the edge app in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "scale-down				scales the edge-api-service deployment down to 0 in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "scale-up					scales the edge-api-service deployment up to 1 in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo "restart-app				scales the edge-api-service deployment down to 0 then up to 1 in the given namespace"
	@echo "                            @param NAMESPACE - (optional) the namespace to use"
	@echo ""


test:
	go test $$(go list ./... | grep -v /test/) $(TEST_OPTIONS)

vet:
	go vet $$(go list ./... | grep -v /vendor/)

lint:
	golint $$(go list ./... | grep -v /vendor/)

build:
	$(CONTAINERER) build . -t $(CONTAINER_TAG)

bonfire-config-local:
	@cp default_config.yaml.local.example config.yaml
	@sed -i ${OS_SED} 's|REPO|$(PWD)|g' config.yaml

bonfire-config-github:
	@cp default_config.yaml.github.example config.yaml

create-ns:
	$(KUBECTL) create ns $(NAMESPACE)

deploy-env:
	bonfire deploy-env -n $(NAMESPACE)

deploy-app:
	bonfire deploy edge -n $(NAMESPACE)

scale-down:
	$(KUBECTL) scale --replicas=0 deployment/edge-api-service -n $(NAMESPACE)

scale-up:
	$(KUBECTL) scale --replicas=1 deployment/edge-api-service -n $(NAMESPACE)

restart-app:
	$(MAKE) scale-down NAMESPACE=$(NAMESPACE)
	sleep 5
	$(MAKE) scale-up NAMESPACE=$(NAMESPACE)

.PHONY: help build