# terraform-provider-polaris

Terraform provider for [Apache Polaris](https://polaris.apache.org/), the open-source implementation of the [Iceberg REST Catalog](https://iceberg.apache.org/concepts/catalog/) specification.

**Documentation:** [Terraform Registry](https://registry.terraform.io/providers/baptistegh/polaris/latest/docs) (resources, data sources and examples; sources in [`docs/`](docs/)).

## Usage

Requires [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.13.

```hcl
terraform {
  required_providers {
    polaris = {
      source  = "baptistegh/polaris"
      version = "~> 0.1"
    }
  }
}

provider "polaris" {
  base_url      = "https://polaris.example.com"
  client_id     = "my-client-id"
  client_secret = "my-client-secret"
}
```

Every setting can come from the environment instead (`POLARIS_BASE_URL`, `POLARIS_CLIENT_ID`, `POLARIS_CLIENT_SECRET`, `POLARIS_TOKEN`, `POLARIS_REALM`). See the [provider documentation](https://registry.terraform.io/providers/baptistegh/polaris/latest/docs) for all authentication options.

## Development

Requires Go >= 1.26 and Docker.

| Command | What it does |
|---|---|
| `make build` / `make install` | Build the provider / install it under `~/.terraform.d/plugins` |
| `make dev-up` / `make dev-down` | Start / stop a local Polaris on `http://localhost:8181` (credentials `root` / `s3cr3t`, realm `POLARIS`) |
| `make test` | Unit tests |
| `make testacc` | Acceptance tests against the local Polaris |
| `make lint` | golangci-lint (in Docker) |
| `make generate` | Regenerate the API client from `spec/` (OpenAPI Generator, in Docker) and the docs |
