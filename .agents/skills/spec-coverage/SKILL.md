---
name: spec-coverage
description: List the Terraform resources, data sources and ephemeral resources that the Polaris management spec (spec/polaris-management-service.yml) supports but the provider does not register yet. Use when asked what is missing, unimplemented or left to do in the provider, after updating the spec to a new Polaris version, or before picking the next type to implement with the add-resource skill.
---

# Spec coverage

## Run

From the repository root:

```bash
python3 .agents/skills/spec-coverage/coverage.py          # Markdown report
python3 .agents/skills/spec-coverage/coverage.py --json   # machine-readable
```

Standard library only, nothing to install.

## How it works

1. **Spec side:** it reads every `operationId` under `paths:` in `spec/polaris-management-service.yml`, plus the `GrantResource` discriminator values (`catalog`, `namespace`, `table`, ...).
2. **Expected types:** the `OPERATIONS` table in `coverage.py` maps each operationId to the Terraform types that should cover it:
   - CRUD on an entity → a resource `polaris_<entity>`; its `get*` → also a data source `polaris_<entity>`; its `list*` → a plural data source `polaris_<entities>`.
   - `assign*` / `revoke*` / `list*Assigned` → the `*_assignment` resource.
   - `rotateCredentials` / `resetCredentials` → the ephemeral `polaris_principal_credentials`.
   - Each grant discriminator value `<t>` → a resource `polaris_<t>_grant` (hyphens become underscores).
   - `[]` means intentionally not exposed (reverse lookups), with the reason in a comment.
3. **Provider side:** it reads the constructors in `Resources()`, `DataSources()` and `EphemeralResources()` in `internal/provider/provider.go`, then takes the type name from each constructor's `Metadata` (`req.ProviderTypeName + "_<type>"`). Only *registered* types count: a file that exists but isn't registered is reported as missing.

## Report sections

- **Missing from the provider:** the list the user asked for. Present it as is, grouped by kind if helpful. Point out which ones already have an example under `examples/` (`ls examples/resources examples/data-sources`), because those are planned and their example fixes the schema.
- **Spec operations with no mapping:** operations the table doesn't know, usually new in a spec update. Decide which type should cover each one using the rules above, add it to `OPERATIONS` (or `[]` with a reason), and run the script again. Never leave this section non-empty without telling the user.
- **Registered but not expected from the spec:** a type the provider has that the spec doesn't explain. Either the mapping is wrong, or it belongs in `PROVIDER_ONLY` (like `polaris_realm`, which only reads provider config).
- **Mappings for operations no longer in the spec:** stale `OPERATIONS` entries after a spec update; remove them and check whether a registered type now relies on a removed endpoint.

If the script exits with "could not parse", the upstream spec layout changed: fix the regexes in `spec_operations` / `grant_types` rather than working around it.

## Next step

To implement one of the missing types, use the `add-resource` skill.
