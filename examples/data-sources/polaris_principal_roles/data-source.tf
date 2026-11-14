data "polaris_principal_roles" "all" {}

output "principal_roles" {
  value = data.polaris_principal_roles.all.names
}
