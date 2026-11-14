---
name: add-resource
description: Add or extend a Terraform resource or data source in this Polaris provider (resource_<type>.go / datasource_<type>.go, its acceptance test, provider registration, example and docs). Use when implementing a new polaris_* type, e.g. one that already has an example under examples/ but no Go implementation, or when adding attributes to an existing one. For grant resources (polaris_*_grant) follow the grant notes below.
---

# Add a resource or data source

Work through every step. A type is not done until its acceptance test, example and docs exist and the checks at the end pass.

## 1. Decide the names

- Terraform type: `polaris_<type>` (e.g. `polaris_catalog`).
- Files in `internal/provider/`: `resource_<type>.go` + `resource_<type>_test.go`, or `datasource_<type>.go` + `datasource_<type>_test.go` (ephemeral: `ephemeral_<type>.go`). `<type>` has no `polaris_` prefix. This naming is mandatory (AGENTS.md).
- Go identifiers: `<camelType>Resource` / `<camelType>ResourceModel` / `new<CamelType>Resource` (data source: `...DataSource`, `...DataSourceModel`, `new<CamelType>DataSource`).

Check whether `examples/resources/polaris_<type>/resource.tf` or `examples/data-sources/polaris_<type>/data-source.tf` already exists: if it does, the schema must accept exactly that config.

## 2. Find the API calls

Look up the operations in `spec/polaris-management-service.yml` and the generated methods in `pkg/polarismanagement/api_default.go` (`client.DefaultAPI.<Op>(ctx, ...).<Body>(...).Execute()`). Never edit `pkg/polarismanagement/*.go` or `spec/` by hand.

Note the success status of each operation from the spec (create is usually `201`, get/update `200`, delete `204`), and which fields are required vs pointers in the generated model.

## 3. Write the implementation

Copy the structure of the closest existing file:

- Simple named entity with properties: `resource_principal_role.go` (CRUD, `entity_version`, import by name).
- Entity scoped by catalog: `resource_catalog_role.go` (composite import ID).
- Data source: `datasource_principal.go`.
- Grant: `resource_catalog_grant.go` + `grants.go` (see "Polymorphic payloads" below).
- Polymorphic entity with nested objects: `resource_catalog.go` + `catalog.go`.

Rules:

- Assert the interfaces at the top: `var _ resource.Resource = &xResource{}` (plus `resource.ResourceWithImportState` when importable).
- State model is a separate struct with `tfsdk` tags.
- `Configure` returns early on nil `req.ProviderData`, type-asserts to `*ProviderData`, and adds an "Unexpected provider data type" error otherwise. Resources keep `pd.Client`; data sources that need provider config (realm, client_id) keep the whole `*ProviderData`.
- Every API call is checked with `checkResponse(httpResp, err, http.StatusXxx)`: the generated client returns an error for every non-2xx, so never check `err` alone.
- In `Read`, handle `isNotFound(httpResp)` first with `resp.State.RemoveResource(ctx)`.
- Identity attributes (name, catalog, ...) get `RequiresReplace()`. Server-set values (`create_timestamp`, `last_update_timestamp`, `entity_version`) are `Computed`.
- Updates send `CurrentEntityVersion` from state.
- Put API-to-state conversion in one `<type>FromAPI(...)` helper shared by Create/Read/Update. For optional+computed maps (`properties`), keep the null-vs-empty handling from `principalRoleFromAPI` so a config without `properties` doesn't produce a diff.
- Every attribute has a `Description` (it ends up in the docs).

### Polymorphic payloads (grants, catalogs)

The generator flattens `allOf` + discriminator subtypes and silently drops their fields. Check the generated model against the spec before using it: if subtype fields are missing, call the endpoint through `doJSON` with hand-written payloads instead, as grants (`grants.go`, `GrantResource` only carries `type`) and catalogs (`catalog.go`, storage and connection configs) do. Reuse those helpers; detect a 404 with `isNotFoundError(err)`, and cover the payload decoding with an `httptest` unit test.

## 4. Register it

Add the constructor to `Resources()` or `DataSources()` in `internal/provider/provider.go`.

## 5. Write the acceptance test (mandatory)

In the `_test.go` next to the source file:

- Name: `TestAcc<Resource|Datasource><CamelType>_<scenario>`, e.g. `TestAccResourceCatalog_basic`.
- First two lines of every `TestAcc*`: `checkTFAcc(t)` then `testAccPreCheck(t)`.
- `resource.Test(t, resource.TestCase{ProtoV6ProviderFactories: providerFactories, Steps: ...})`.
- Every config starts with `testAccProviderConfig() + ` — never hardcode credentials.
- Need a catalog / namespace / table / view? Call `testCreateCatalog(t)`, `testCreateNamespace`, `testCreateTable`, `testCreateView`: they create random fixtures and clean up via `t.Cleanup`.
- Cover: create, update of each updatable attribute, import (`ImportStateVerify` with `ImportStateVerifyIdentifierAttribute` when there's no `id`), every optional/computed attribute with non-trivial logic, and default-value behaviour (separate `_scenario` functions are fine).
- Logic that doesn't need a server (parsing import IDs, payload building) gets a plain unit test `Test<Name>` without `checkTFAcc`, using `httptest` for HTTP.

## 6. Example and docs

- Add or update `examples/resources/polaris_<type>/resource.tf` (optionally an `import.sh` with `terraform import polaris_<type>.example <id>` matching the import ID format; tfplugindocs adds it to the docs) or `examples/data-sources/polaris_<type>/data-source.tf`.
- Run `make docs` and keep the generated `docs/` changes: CI fails if `make generate` leaves a diff.

## 7. Check

```bash
gofmt -s -w .
go build ./...
make test        # unit tests; TestAcc* skip without TF_ACC
make lint
make dev-up && make testacc   # or a single test: see AGENTS.md
make dev-down
```

Report the results honestly, including any acceptance test you could not run (e.g. Docker unavailable).

Commit with a `feat:` subject (new type) or `fix:` / `feat:` (attribute change); see the `commit-and-pr` skill.
