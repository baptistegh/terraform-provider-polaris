# Vendored Polaris OpenAPI specs

Copies of the upstream [Apache Polaris](https://github.com/apache/polaris/tree/main/spec) specs, taken from the
release tag in `POLARIS_SPEC_TAG` (`Makefile`, currently `apache-polaris-1.9.0`). Do not edit them: refresh them
with `make update-spec` and regenerate with `make generate`.

| File | Upstream path | Served under | Used for |
|---|---|---|---|
| `polaris-management-service.yml` | `spec/polaris-management-service.yml` | `/api/management/v1` | Generated client `pkg/polarismanagement` (`make generate-client`) |
| `bundled-polaris-catalog-service.yaml` | `spec/generated/bundled-polaris-catalog-service.yaml` | `/api/catalog` | Reference only: `doCatalogJSON` with hand-written payloads |

The catalog spec is the upstream bundle of the Iceberg REST catalog API (`/v1/{prefix}/...`: namespaces, tables,
views) and the Polaris-native catalog APIs (`/polaris/v1/{prefix}/...`: policies, generic tables, semantic
models). No client is generated from it: openapi-generator's Go output for it does not compile (duplicate
declarations from flattened `allOf` schemas) and is ~48k lines, almost all for engine-facing table/view
operations. The few payloads the provider needs are written by hand in `internal/provider/`.

Both files are read by the `spec-coverage` skill (`.agents/skills/spec-coverage/coverage.py`).
