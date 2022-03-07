
BUILD_PATH     ?= $(shell pwd)
GO_MODULE      ?= github.com/finleap-connect/vaultoperator
GO             ?= go

GINKGO         ?= $(TOOLS_DIR)/ginkgo
GINKO_VERSION  ?= v1.16.4

LINTER 	   	   ?= $(TOOLS_DIR)/golangci-lint
LINTER_VERSION ?= v1.39.0

.PHONY: go-lint go-mod go-fmt go-vet go-test go-coverage

##@ Go

$(TOOLS_DIR)/ginkgo:
	@echo "Installing $@"
	@GOBIN=$(TOOLS_DIR) go install github.com/onsi/ginkgo/ginkgo@$(GINKO_VERSION)
	
$(TOOLS_DIR)/golangci-lint:
	@echo "Installing $@"
	@GOBIN=$(TOOLS_DIR) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINTER_VERSION)

go-mod: ## go mod download and verify
	$(GO) mod tidy
	$(GO) mod download
	$(GO) mod verify

go-fmt:  ## go fmt
	$(GO) fmt ./...

go-vet: ## go vet
	$(GO) vet ./...

go-lint: $(LINTER) ## go lint
	$(LINTER) run -v -E goconst -E misspell

go-test: $(GINKGO) $(KUBEBUILDER) $(VAULT) generate go-fmt go-vet manifests  ## run all tests
	@$(GINKGO) -r -v -cover --failFast -requireSuite -covermode count -outputdir=$(BUILD_PATH) -coverprofile=.coverprofile 

go-coverage: ## print coverage from coverprofiles
	@go tool cover -func .coverprofile 
