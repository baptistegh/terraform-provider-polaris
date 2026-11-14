data "polaris_catalog" "lakehouse" {
  name = "lakehouse"
}

output "lakehouse_storage_type" {
  value = data.polaris_catalog.lakehouse.storage_config.storage_type
}
