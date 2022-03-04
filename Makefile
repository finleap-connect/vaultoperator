# Directory, where all required tools are located (absolute path required)
BUILD_PATH ?= $(shell pwd)
TOOLS_DIR   ?= $(shell cd tools 2>/dev/null && pwd)

# Prerequisite tools
GO ?= go
DOCKER ?= docker
KUBECTL ?= kubectl
HELM ?= helm

# Tools managed by this project
GINKGO ?= $(TOOLS_DIR)/ginkgo
GINKO_VERSION  ?= v1.16.4

LINTER ?= $(TOOLS_DIR)/golangci-lint
LINTER_VERSION ?= v1.43.0

KIND ?= $(TOOLS_DIR)/kind
VAULT ?= $(TOOLS_DIR)/vault
CONTROLLER_GEN ?= $(TOOLS_DIR)/controller-gen
KUSTOMIZE ?= $(TOOLS_DIR)/kustomize
KUBEBUILDER ?= $(TOOLS_DIR)/kubebuilder
KUBEBUILDER_ASSETS ?= $(TOOLS_DIR)

# Variables
MANAGER_BIN ?= bin/manager

HELM_CHART_NAME ?= vault-operator
HELM_CHART_DIR ?= charts/$(HELM_CHART_NAME)
HELM_RELEASE_NAME ?= dev-vault-operator
HELM_NAMESPACE ?= default

export

.PHONY: all test lint fmt vet install uninstall deploy manifests

all: $(MANAGER_BIN)

$(MANAGER_BIN): generate fmt vet
	$(GO) build -o $(MANAGER_BIN) ./main.go

lint: $(LINTER) helm-lint go-lint

test: generate fmt vet manifests $(GINKGO) $(KUBEBUILDER) $(VAULT)
	@$(GINKGO) -r -v -cover --failFast -requireSuite -covermode count -outputdir=$(BUILD_PATH) -coverprofile=.coverprofile 

coverage: ## print coverage from coverprofiles
	@go tool cover -func .coverprofile 

go-lint: 
	$(GO) mod verify
	$(LINTER) run -v --no-config --deadline=5m

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

# Generate manifests e.g. CRD, RBAC etc.
manifests: $(CONTROLLER_GEN) $(KUSTOMIZE)
	$(CONTROLLER_GEN) crd:trivialVersions=false rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases
	echo "# Generated by 'make manifests'\n" > $(HELM_CHART_DIR)/templates/crds.yaml
	$(KUSTOMIZE) build config/crd-templates >> $(HELM_CHART_DIR)/templates/crds.yaml
	echo "# Generated by 'make manifests'\n" > $(HELM_CHART_DIR)/templates/rbac.yaml
	$(KUSTOMIZE) build config/rbac-templates >> $(HELM_CHART_DIR)/templates/rbac.yaml
	echo "# Generated by 'make manifests'\n" > $(HELM_CHART_DIR)/templates/webhook.yaml
	$(KUSTOMIZE) build config/webhook-templates >> $(HELM_CHART_DIR)/templates/webhook.yaml

# Generate code using controller-gen
generate: $(CONTROLLER_GEN)
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

helm-install: $(HELM)
	$(HELM) upgrade --install $(HELM_RELEASE_NAME) --namespace $(HELM_NAMESPACE) $(HELM_CHART_DIR)

helm-uninstall: $(HELM)
	$(HELM) uninstall --namespace $(HELM_NAMESPACE) $(HELM_RELEASE_NAME)

helm-lint: $(HELM)
	$(HELM) lint $(HELM_CHART_DIR)

# Phony target to install all required tools into ${TOOLS_DIR}
tools: $(TOOLS_DIR)/kind $(TOOLS_DIR)/ginkgo $(TOOLS_DIR)/controller-gen $(TOOLS_DIR)/kustomize $(TOOLS_DIR)/golangci-lint $(TOOLS_DIR)/kubebuilder

$(TOOLS_DIR)/kind:
	@echo "Installing $@"
	@GOBIN=$(TOOLS_DIR) go install sigs.k8s.io/kind@v0.7.0

$(TOOLS_DIR)/ginkgo:
	@echo "Installing $@"
	@GOBIN=$(TOOLS_DIR) go install github.com/onsi/ginkgo/ginkgo@$(GINKO_VERSION)

$(TOOLS_DIR)/controller-gen:
	@echo "Installing $@"
	@GOBIN=$(TOOLS_DIR) go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.2.5

$(TOOLS_DIR)/kustomize:
	@echo "Installing $@"
	@$(TOOLS_DIR)/install_kustomize.sh $(TOOLS_DIR)

$(TOOLS_DIR)/golangci-lint:
	@echo "Installing $@"
	@GOBIN=$(TOOLS_DIR) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINTER_VERSION)

$(TOOLS_DIR)/kubebuilder $(TOOLS_DIR)/kubectl $(TOOLS_DIR)/kube-apiserver $(TOOLS_DIR)/etcd:
	@$(TOOLS_DIR)/kubebuilder-install

$(TOOLS_DIR)/vault:
	@$(TOOLS_DIR)/vault-install
