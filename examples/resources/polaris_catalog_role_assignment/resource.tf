resource "polaris_principal_role" "data_engineer" {
  name = "data-engineer"
}

resource "polaris_catalog_role" "reader" {
  catalog = "prod"
  name    = "reader"
}

resource "polaris_catalog_role_assignment" "example" {
  principal_role = polaris_principal_role.data_engineer.name
  catalog        = polaris_catalog_role.reader.catalog
  catalog_role   = polaris_catalog_role.reader.name
}
