resource "polaris_principal" "example" {
  name = "alice"

  properties = {
    "team" = "data-engineering"
  }
}
