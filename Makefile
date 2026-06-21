BINARY        := bin/terraform-provider-polaris
VERSION       ?= 0.1.0
OS_ARCH       := $(shell go env GOOS)_$(shell go env GOARCH)
INSTALL_DIR   := $(HOME)/.terraform.d/plugins/registry.terraform.io/baptistegh/polaris/$(VERSION)/$(OS_ARCH)

POLARIS_BASE_URL      ?= http://localhost:8181
POLARIS_CLIENT_ID     ?= root
POLARIS_CLIENT_SECRET ?= s3cr3t
POLARIS_REALM         ?= POLARIS

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
	go test ./... -count=1

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

.PHONY: generate
generate:
	go generate ./...

.PHONY: docs
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate --provider-name polaris

.PHONY: lint
lint:
	golangci-lint run

.PHONY: fmt
fmt:
	gofmt -s -w .

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: changelog
changelog:
	git-cliff -o CHANGELOG.md

.PHONY: clean
clean:
	rm -f $(BINARY)
