resource "polaris_principal" "alice" {
  name = "alice"
}

resource "polaris_principal_role" "data_engineer" {
  name = "data-engineer"
}

resource "polaris_principal_role_assignment" "example" {
  principal      = polaris_principal.alice.name
  principal_role = polaris_principal_role.data_engineer.name
}
