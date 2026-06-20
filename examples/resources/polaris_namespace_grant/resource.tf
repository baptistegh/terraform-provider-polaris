resource "polaris_catalog_role" "reader" {
  catalog = "prod"
  name    = "reader"
}

resource "polaris_namespace_grant" "example" {
  catalog      = polaris_catalog_role.reader.catalog
  catalog_role = polaris_catalog_role.reader.name
  namespace    = ["sales"]
  privilege    = "NAMESPACE_LIST"
}
