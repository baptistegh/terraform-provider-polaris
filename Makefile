BINARY        := bin/terraform-provider-polaris
VERSION       ?= 0.1.0
OS_ARCH       := $(shell go env GOOS)_$(shell go env GOARCH)
INSTALL_DIR   := $(HOME)/.terraform.d/plugins/registry.terraform.io/baptistegh/polaris/$(VERSION)/$(OS_ARCH)

POLARIS_BASE_URL      ?= http://localhost:8181
POLARIS_CLIENT_ID     ?= root
POLARIS_CLIENT_SECRET ?= s3cr3t
POLARIS_REALM         ?= POLARIS

# renovate: datasource=docker depName=golangci/golangci-lint
GOLANGCI_LINT_VERSION ?= v2.15.0

# renovate: datasource=docker depName=openapitools/openapi-generator-cli
OPENAPI_GENERATOR_VERSION ?= v7.27.0
CLIENT_DIR := pkg/polarismanagement

# Upstream Polaris release the vendored spec/ files come from. Keep it in line with the
# apache/polaris image in docker-compose.yml, then run `make update-spec generate`.
POLARIS_SPEC_TAG ?= apache-polaris-1.9.0
POLARIS_SPEC_URL := https://raw.githubusercontent.com/apache/polaris/$(POLARIS_SPEC_TAG)/spec

default: build

.PHONY: build
build:
	go build -o $(BINARY) .

.PHONY: install
install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)

.PHONY: test
test:
	go test -v ./... -count=1

.PHONY: testacc
testacc:
	TF_ACC=1 \
	POLARIS_BASE_URL=$(POLARIS_BASE_URL) \
	POLARIS_CLIENT_ID=$(POLARIS_CLIENT_ID) \
	POLARIS_CLIENT_SECRET=$(POLARIS_CLIENT_SECRET) \
	POLARIS_REALM=$(POLARIS_REALM) \
	go test ./internal/provider/ -v -count=1 -timeout 120m

.PHONY: dev-up
dev-up:
	docker compose up -d --wait

.PHONY: dev-down
dev-down:
	docker compose down

.PHONY: dev-token
dev-token:
	@curl -sf -X POST $(POLARIS_BASE_URL)/api/catalog/v1/oauth/tokens \
		-H "Content-Type: application/x-www-form-urlencoded" \
		-H "Polaris-Realm: $(POLARIS_REALM)" \
		-d "grant_type=client_credentials&client_id=$(POLARIS_CLIENT_ID)&client_secret=$(POLARIS_CLIENT_SECRET)&scope=PRINCIPAL_ROLE:ALL" \
		| python3 -m json.tool

# Downloads the management spec and the bundled catalog spec (Iceberg REST + Polaris-native
# catalog APIs) from POLARIS_SPEC_TAG. See spec/README.md.
.PHONY: update-spec
update-spec:
	curl -sSfL -o spec/polaris-management-service.yml $(POLARIS_SPEC_URL)/polaris-management-service.yml
	curl -sSfL -o spec/bundled-polaris-catalog-service.yaml $(POLARIS_SPEC_URL)/generated/bundled-polaris-catalog-service.yaml

.PHONY: generate
generate: generate-client docs

# Regenerates the management API client from spec/ with openapi-generator (Docker).
.PHONY: generate-client
generate-client:
	find $(CLIENT_DIR) -name '*.go' -delete
	rm -rf $(CLIENT_DIR)/api $(CLIENT_DIR)/.openapi-generator
	docker run --rm -u $$(id -u):$$(id -g) -v $(CURDIR):/local -w /local \
		openapitools/openapi-generator-cli:$(OPENAPI_GENERATOR_VERSION) generate \
		-i spec/polaris-management-service.yml \
		-c $(CLIENT_DIR)/openapi-generator.yaml \
		-o $(CLIENT_DIR) \
		--global-property apis,models,supportingFiles=client.go:configuration.go:response.go:utils.go,apiDocs=false,modelDocs=false,apiTests=false,modelTests=false
	rm -rf $(CLIENT_DIR)/api $(CLIENT_DIR)/.openapi-generator
	gofmt -s -w $(CLIENT_DIR)

.PHONY: docs
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name polaris

.PHONY: lint
lint:
	docker run --rm -t \
		-v $(CURDIR):/app -w /app \
		-v $(HOME)/.cache/golangci-lint:/root/.cache \
		golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) golangci-lint run

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: clean
clean:
	rm -f $(BINARY)
