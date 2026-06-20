resource "polaris_catalog_role" "example" {
  catalog = "prod"
  name    = "reader"

  properties = {
    "env" = "production"
  }
}
