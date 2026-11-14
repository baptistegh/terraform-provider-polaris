data "polaris_catalog_roles" "lakehouse" {
  catalog = "lakehouse"
}

output "lakehouse_roles" {
  value = data.polaris_catalog_roles.lakehouse.names
}
