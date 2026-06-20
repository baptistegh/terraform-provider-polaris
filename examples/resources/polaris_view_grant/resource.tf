resource "polaris_catalog_role" "reader" {
  catalog = "prod"
  name    = "reader"
}

resource "polaris_view_grant" "example" {
  catalog      = polaris_catalog_role.reader.catalog
  catalog_role = polaris_catalog_role.reader.name
  namespace    = ["sales"]
  view_name    = "monthly_summary"
  privilege    = "VIEW_READ_PROPERTIES"
}
