resource "polaris_catalog_role" "reader" {
  catalog = "prod"
  name    = "reader"
}

resource "polaris_policy_grant" "example" {
  catalog      = polaris_catalog_role.reader.catalog
  catalog_role = polaris_catalog_role.reader.name
  namespace    = ["compliance"]
  policy_name  = "data_retention"
  privilege    = "POLICY_READ"
}
