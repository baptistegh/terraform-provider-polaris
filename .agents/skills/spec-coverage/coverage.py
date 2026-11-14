#!/usr/bin/env python3
"""Compare the vendored Polaris specs with the types the provider registers.

Usage (from the repository root):
    python3 .agents/skills/spec-coverage/coverage.py [--json]

Standard library only. Each spec in SPECS is parsed line by line, relying on
the upstream layout: paths at 2-space indent, HTTP methods at 4, `operationId`
inside each method, and (management spec) the `GrantResource` discriminator.
"""

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
MANAGEMENT_SPEC = ROOT / "spec" / "polaris-management-service.yml"
# Iceberg REST catalog API + Polaris-native catalog APIs, served under /api/catalog.
CATALOG_SPEC = ROOT / "spec" / "bundled-polaris-catalog-service.yaml"
SPECS = [MANAGEMENT_SPEC, CATALOG_SPEC]
PROVIDER_DIR = ROOT / "internal" / "provider"

R, DS, EPH = "resource", "data-source", "ephemeral"

# operationId -> Terraform types expected to cover it.
# An empty list means "intentionally not exposed"; give the reason in the comment.
# Grant operations are covered by every polaris_<type>_grant derived from the
# GrantResource discriminator, see grant_types().
OPERATIONS = {
    # Catalogs
    "listCatalogs": [(DS, "polaris_catalogs")],
    "createCatalog": [(R, "polaris_catalog")],
    "getCatalog": [(R, "polaris_catalog"), (DS, "polaris_catalog")],
    "updateCatalog": [(R, "polaris_catalog")],
    "deleteCatalog": [(R, "polaris_catalog")],
    # Principals
    "listPrincipals": [(DS, "polaris_principals")],
    "createPrincipal": [(R, "polaris_principal")],
    "getPrincipal": [(R, "polaris_principal"), (DS, "polaris_principal")],
    "updatePrincipal": [(R, "polaris_principal")],
    "deletePrincipal": [(R, "polaris_principal")],
    # A managed resource, not an ephemeral one: both calls invalidate the previous
    # secret, and an ephemeral resource would issue new credentials on every plan.
    "rotateCredentials": [(R, "polaris_principal_credentials")],
    "resetCredentials": [(R, "polaris_principal_credentials")],
    # Principal role assignments (principal <-> principal role)
    "listPrincipalRolesAssigned": [(R, "polaris_principal_role_assignment")],
    "assignPrincipalRole": [(R, "polaris_principal_role_assignment")],
    "revokePrincipalRole": [(R, "polaris_principal_role_assignment")],
    "listAssigneePrincipalsForPrincipalRole": [],  # reverse lookup, read side of the assignment
    # Principal roles
    "listPrincipalRoles": [(DS, "polaris_principal_roles")],
    "createPrincipalRole": [(R, "polaris_principal_role")],
    "getPrincipalRole": [(R, "polaris_principal_role"), (DS, "polaris_principal_role")],
    "updatePrincipalRole": [(R, "polaris_principal_role")],
    "deletePrincipalRole": [(R, "polaris_principal_role")],
    # Catalog role assignments (principal role <-> catalog role)
    "listCatalogRolesForPrincipalRole": [(R, "polaris_catalog_role_assignment")],
    "assignCatalogRoleToPrincipalRole": [(R, "polaris_catalog_role_assignment")],
    "revokeCatalogRoleFromPrincipalRole": [(R, "polaris_catalog_role_assignment")],
    "listAssigneePrincipalRolesForCatalogRole": [],  # reverse lookup, read side of the assignment
    # Catalog roles
    "listCatalogRoles": [(DS, "polaris_catalog_roles")],
    "createCatalogRole": [(R, "polaris_catalog_role")],
    "getCatalogRole": [(R, "polaris_catalog_role"), (DS, "polaris_catalog_role")],
    "updateCatalogRole": [(R, "polaris_catalog_role")],
    "deleteCatalogRole": [(R, "polaris_catalog_role")],
    # Grants: covered by the derived polaris_<type>_grant resources
    "listGrantsForCatalogRole": "grants",
    "addGrantToCatalogRole": "grants",
    "revokeGrantFromCatalogRole": "grants",

    # ---- Catalog API (CATALOG_SPEC) ----
    # Provider and client plumbing, not objects to manage.
    "getConfig": [],  # catalog client configuration
    "getToken": [],  # OAuth token endpoint, used by the provider itself (fetchToken)
    "listAccessibleCatalogs": [],  # the caller's view of catalogs; polaris_catalogs lists them all
    # Namespaces (Iceberg REST)
    "listNamespaces": [(DS, "polaris_namespaces")],
    "createNamespace": [(R, "polaris_namespace")],
    "loadNamespaceMetadata": [(R, "polaris_namespace"), (DS, "polaris_namespace")],
    "namespaceExists": [],  # loadNamespaceMetadata already answers this
    "dropNamespace": [(R, "polaris_namespace")],
    "updateProperties": [(R, "polaris_namespace")],
    # Tables and views: data objects created and changed by query engines (Spark, Trino, ...),
    # not infrastructure. Grants on them exist (polaris_table_grant, polaris_view_grant).
    "listTables": [],
    "createTable": [],
    "registerTable": [],
    "loadTable": [],
    "updateTable": [],
    "dropTable": [],
    "tableExists": [],
    "loadCredentials": [],  # credential vending for engines
    "signRequest": [],  # request signing for engines
    "renameTable": [],
    "reportMetrics": [],  # engine scan/commit metrics
    "commitTransaction": [],
    "listViews": [],
    "createView": [],
    "registerView": [],
    "loadView": [],
    "replaceView": [],
    "dropView": [],
    "viewExists": [],
    "renameView": [],
    "sendNotification": [],  # external catalogs notifying Polaris of table changes
    # Generic tables (non-Iceberg formats): engine-managed like tables.
    "listGenericTables": [],
    "createGenericTable": [],
    "loadGenericTable": [],
    "dropGenericTable": [],
    # Policies (Polaris-native)
    "createPolicy": [(R, "polaris_policy")],
    "listPolicies": [(DS, "polaris_policies")],
    "loadPolicy": [(R, "polaris_policy"), (DS, "polaris_policy")],
    "updatePolicy": [(R, "polaris_policy")],
    "dropPolicy": [(R, "polaris_policy")],
    "attachPolicy": [(R, "polaris_policy_attachment")],
    "detachPolicy": [(R, "polaris_policy_attachment")],
    "getApplicablePolicies": [],  # effective policies after inheritance: a computed view, not an object
    # Semantic models (Polaris-native, needs ENABLE_SEMANTIC_MODELS): administrator-defined
    # documents like policies, so expected as types.
    "createSemanticModel": [(R, "polaris_semantic_model")],
    "listSemanticModels": [(DS, "polaris_semantic_models")],
    "loadSemanticModel": [(R, "polaris_semantic_model"), (DS, "polaris_semantic_model")],
    "updateSemanticModel": [(R, "polaris_semantic_model")],
    "dropSemanticModel": [(R, "polaris_semantic_model")],
}

# Registered types that do not come from a spec operation.
PROVIDER_ONLY = {(DS, "polaris_realm")}  # reads provider config, no API call


def spec_operations(text):
    """Return [(method, path, operationId)] in spec order."""
    ops, path, method = [], None, None
    in_paths = False
    for line in text.splitlines():
        if re.match(r"^paths:\s*$", line):
            in_paths = True
            continue
        if in_paths and re.match(r"^\S", line):
            in_paths = False
        if not in_paths:
            continue
        if m := re.match(r"^  (/\S*):\s*$", line):
            path, method = m.group(1), None
        elif m := re.match(r"^    (get|put|post|delete|patch|head):\s*$", line):
            method = m.group(1).upper()
        elif (m := re.match(r"^      operationId:\s*(\S+)", line)) and method:
            ops.append((method, path, m.group(1)))
    return ops


def grant_types(text):
    """Return the GrantResource discriminator values (catalog, namespace, ...)."""
    m = re.search(r"^    GrantResource:\n(.*?)(?=^    \S)", text, re.M | re.S)
    if not m:
        return []
    mapping = re.search(r"mapping:\n((?:\s{10}\S.*\n)+)", m.group(1))
    if not mapping:
        return []
    return re.findall(r"^\s{10}([\w-]+):", mapping.group(1), re.M)


def registered_types():
    """Return {(kind, type_name): source file} for types registered in provider.go."""
    provider = (PROVIDER_DIR / "provider.go").read_text()
    sections = {
        R: r"Resources\(.*?\{(.*?)\n\}",
        DS: r"DataSources\(.*?\{(.*?)\n\}",
        EPH: r"EphemeralResources\(.*?\{(.*?)\n\}",
    }
    sources = {f: f.read_text() for f in PROVIDER_DIR.glob("*.go") if not f.name.endswith("_test.go")}
    found = {}
    for kind, pattern in sections.items():
        body = re.search(pattern, provider, re.S)
        if not body:
            continue
        for ctor in re.findall(r"\b(new\w+)\b", body.group(1)):
            for f, src in sources.items():
                if re.search(rf"^func {ctor}\(", src, re.M):
                    name = re.search(r'ProviderTypeName \+ "(\w+)"', src)
                    if name:
                        found[(kind, "polaris" + name.group(1))] = f.name
                    break
    return found


def main():
    ops = []  # (method, path, operationId, spec file)
    per_spec = {}
    for spec in SPECS:
        spec_ops = spec_operations(spec.read_text())
        if not spec_ops:
            sys.exit(f"could not parse {spec.relative_to(ROOT)}: found no operations")
        per_spec[spec.name] = len(spec_ops)
        ops += [(m, p, o, spec.name) for m, p, o in spec_ops]
    grants = grant_types(MANAGEMENT_SPEC.read_text())
    if not grants:
        sys.exit(f"could not parse {MANAGEMENT_SPEC.relative_to(ROOT)}: found no grant types")

    expected = {}  # (kind, type) -> [operationId]
    unmapped = []
    grant_ops = []
    for method, path, op, spec in ops:
        targets = OPERATIONS.get(op)
        if targets is None:
            unmapped.append({"method": method, "path": path, "operationId": op, "spec": spec})
        elif targets == "grants":
            grant_ops.append(op)
        else:
            for t in targets:
                expected.setdefault(t, []).append(op)
    for g in grants:
        expected[(R, f"polaris_{g.replace('-', '_')}_grant")] = [f"GrantResource type {g!r} (via {', '.join(grant_ops)})"]

    registered = registered_types()
    missing = sorted((k, t, expected[(k, t)]) for (k, t) in expected if (k, t) not in registered)
    covered = sorted((k, t) for (k, t) in expected if (k, t) in registered)
    extra = sorted(k for k in registered if k not in expected and k not in PROVIDER_ONLY)
    stale = sorted(op for op in OPERATIONS if op not in {o for _, _, o, _ in ops})

    result = {
        "missing": [{"kind": k, "type": t, "operations": o} for k, t, o in missing],
        "unmapped_operations": unmapped,
        "registered_not_in_spec": [{"kind": k, "type": t} for k, t in extra],
        "stale_mappings": stale,
        "covered": [{"kind": k, "type": t} for k, t in covered],
    }
    if "--json" in sys.argv:
        print(json.dumps(result, indent=2))
        return

    print("Specs: " + ", ".join(f"{name} ({n} operations)" for name, n in per_spec.items()))
    print(f"Grant types: {', '.join(grants)}")
    print(f"Provider: {len(registered)} registered types, {len(covered)}/{len(expected)} expected types covered\n")
    print("## Missing from the provider\n")
    if missing:
        print("| Kind | Type | Spec operations |\n|---|---|---|")
        for k, t, o in missing:
            print(f"| {k} | `{t}` | {', '.join(o)} |")
    else:
        print("None.")
    if unmapped:
        print("\n## Spec operations with no mapping (new in the spec? add them to OPERATIONS)\n")
        for u in unmapped:
            print(f"- `{u['method']} {u['path']}` ({u['operationId']}, {u['spec']})")
    if extra:
        print("\n## Registered but not expected from the spec\n")
        for k, t in extra:
            print(f"- {k} `{t}`")
    if stale:
        print("\n## Mappings for operations no longer in the spec (remove from OPERATIONS)\n")
        for op in stale:
            print(f"- {op}")


if __name__ == "__main__":
    main()
