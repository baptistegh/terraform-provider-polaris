resource "polaris_catalog_role" "reader" {
  catalog = "prod"
  name    = "reader"
}

resource "polaris_table_grant" "example" {
  catalog      = polaris_catalog_role.reader.catalog
  catalog_role = polaris_catalog_role.reader.name
  namespace    = ["sales"]
  table_name   = "orders"
  privilege    = "TABLE_READ_DATA"
}
