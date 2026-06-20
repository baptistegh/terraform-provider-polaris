resource "polaris_principal_role" "example" {
  name = "data-engineer"

  properties = {
    "team" = "data-engineering"
  }
}
