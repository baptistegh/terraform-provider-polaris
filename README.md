# terraform-provider-polaris

Terraform provider for [Apache Polaris](https://polaris.apache.org/), the open-source implementation of the [Iceberg REST Catalog](https://iceberg.apache.org/concepts/catalog/) specification.

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) >= 1.13
- [Go](https://golang.org/doc/install) >= 1.22 (to build from source)

## Usage

```hcl
terraform {
  required_providers {
    polaris = {
      source  = "baptistegh/polaris"
      version = "~> 0.1"
    }
  }
}
```

### Authentication

**OAuth2 client credentials** (recommended — the provider fetches the token automatically):

```hcl
provider "polaris" {
  base_url      = "https://polaris.example.com"
  client_id     = "my-client-id"
  client_secret = "my-client-secret"
}
```

**Pre-fetched Bearer token:**

```hcl
provider "polaris" {
  base_url = "https://polaris.example.com"
  token    = "eyJ..."
}
```

**Multi-tenant deployment:**

```hcl
provider "polaris" {
  base_url      = "https://polaris.example.com"
  client_id     = "my-client-id"
  client_secret = "my-client-secret"
  realm         = "my-realm"
}
```

Credentials can also be supplied via environment variables to avoid hardcoding secrets:

| Variable | Description |
|---|---|
| `POLARIS_BASE_URL` | Base URL of the Polaris server |
| `POLARIS_CLIENT_ID` | OAuth2 client ID |
| `POLARIS_CLIENT_SECRET` | OAuth2 client secret |
| `POLARIS_TOKEN` | Pre-fetched Bearer token |
| `POLARIS_REALM` | Realm for multi-tenant deployments |

## Resources and data sources

### Resources

| Resource | Description |
|---|---|
| `polaris_catalog` | Manages a Polaris catalog |
| `polaris_principal` | Manages a principal (service account) |
| `polaris_principal_role` | Manages a principal role |
| `polaris_catalog_role` | Manages a catalog role |
| `polaris_principal_role_assignment` | Assigns a principal role to a principal |
| `polaris_catalog_role_assignment` | Assigns a catalog role to a principal role |
| `polaris_catalog_grant` | Grants a catalog-level privilege to a catalog role |
| `polaris_namespace_grant` | Grants a namespace-level privilege to a catalog role |
| `polaris_table_grant` | Grants a table-level privilege to a catalog role |
| `polaris_view_grant` | Grants a view-level privilege to a catalog role |
| `polaris_policy_grant` | Grants a policy-level privilege to a catalog role |

### Data sources

| Data source | Description |
|---|---|
| `polaris_realm` | Reads the realm from the provider configuration |
| `polaris_principal` | Looks up a principal by name |

## Example

```hcl
resource "polaris_principal" "alice" {
  name = "alice"
}

resource "polaris_principal_role" "data_engineer" {
  name = "data-engineer"
}

resource "polaris_principal_role_assignment" "alice" {
  principal      = polaris_principal.alice.name
  principal_role = polaris_principal_role.data_engineer.name
}

resource "polaris_catalog_role" "reader" {
  catalog = "prod"
  name    = "reader"
}

resource "polaris_catalog_role_assignment" "data_engineer" {
  principal_role = polaris_principal_role.data_engineer.name
  catalog        = polaris_catalog_role.reader.catalog
  catalog_role   = polaris_catalog_role.reader.name
}

resource "polaris_table_grant" "orders" {
  catalog      = polaris_catalog_role.reader.catalog
  catalog_role = polaris_catalog_role.reader.name
  namespace    = ["sales"]
  table_name   = "orders"
  privilege    = "TABLE_READ_DATA"
}
```

## Development

### Prerequisites

- Go >= 1.22
- Docker (for running Polaris locally)
- [`golangci-lint`](https://golangci-lint.run/usage/install/) (for linting)

### Start a local Polaris instance

```bash
make dev-up
```

This starts Polaris on `http://localhost:8181` with bootstrap credentials `root` / `s3cr3t` in realm `POLARIS`.

### Build and install locally

```bash
make install
```

Installs the provider under `~/.terraform.d/plugins` so you can reference it in a local Terraform configuration.

### Run tests

```bash
# Unit tests
make test

# Acceptance tests (requires Polaris running)
make dev-up
make testacc
```

### Regenerate the API client and docs

```bash
make generate
```

Updates both the generated management API client (from `spec/polaris-management-service.yml`) and the Terraform provider docs under `docs/`.
