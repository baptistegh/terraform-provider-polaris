# Terraform Provider for Apache Polaris

[![CI](https://github.com/baptistegh/terraform-provider-polaris/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/baptistegh/terraform-provider-polaris/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/baptistegh/terraform-provider-polaris)](https://github.com/baptistegh/terraform-provider-polaris/releases/latest)
[![Terraform Registry](https://img.shields.io/badge/registry-baptistegh%2Fpolaris-7B42BC?logo=terraform)](https://registry.terraform.io/providers/baptistegh/polaris/latest/docs)

Manage [Apache Polaris](https://polaris.apache.org/), the open-source [Iceberg REST catalog](https://iceberg.apache.org/concepts/catalog/), as code: catalogs, namespaces, policies, principals, roles and their grants.

📖 **[Documentation on the Terraform Registry](https://registry.terraform.io/providers/baptistegh/polaris/latest/docs)**: every resource and data source, with examples (sources in [`docs/`](docs/)).

## Usage

Requires [Terraform](https://developer.hashicorp.com/terraform/install) >= 1.13.

```hcl
terraform {
  required_providers {
    polaris = {
      source  = "baptistegh/polaris"
      version = "~> 0.2"
    }
  }
}

provider "polaris" {
  base_url      = "https://polaris.example.com"
  client_id     = "my-client-id"
  client_secret = "my-client-secret"
}
```

## Try it locally

You need Docker, Go and Terraform.

```bash
make dev-up    # Polaris on http://localhost:8181 (realm POLARIS, credentials root / s3cr3t)
make install   # build the provider and install it in ~/.terraform.d/plugins

cd playground
terraform init && terraform apply

make dev-down  # when you are done
```

[`playground/`](playground/) is already configured for the local server. Edit `main.tf` to try any resource. `make dev-token` prints a token for calling the Polaris API yourself.

## Development

Requires Go >= 1.26 and Docker.

| Command | What it does |
|---|---|
| `make build` | Build the provider into `bin/` |
| `make test` | Unit tests (no Polaris needed) |
| `make testacc` | Acceptance tests against the local Polaris (`make dev-up` first) |
| `make lint` | golangci-lint, in Docker |
| `make fmt` | Format the code |
| `make update-spec` | Refresh the vendored Polaris API specs in [`spec/`](spec/) |
| `make generate` | Regenerate the management API client and the docs (CI checks they are up to date) |

To run a single acceptance test:

```bash
TF_ACC=1 POLARIS_BASE_URL=http://localhost:8181 POLARIS_CLIENT_ID=root POLARIS_CLIENT_SECRET=s3cr3t POLARIS_REALM=POLARIS \
  go test ./internal/provider/ -run TestAccResourcePrincipal_basic -v
```

[`AGENTS.md`](AGENTS.md) describes the architecture and the conventions for adding resources (file naming, mandatory acceptance tests).

## Contributing

Issues and pull requests are welcome. Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, ...), because [release-please](https://github.com/googleapis/release-please) builds the changelog and the releases from them. See the [changelog](CHANGELOG.md) for what changed in each version.
