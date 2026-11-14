data "polaris_catalogs" "all" {}

# Catalogs on S3, by name
output "s3_catalogs" {
  value = [for c in data.polaris_catalogs.all.catalogs : c.name if c.storage_config.storage_type == "S3"]
}
