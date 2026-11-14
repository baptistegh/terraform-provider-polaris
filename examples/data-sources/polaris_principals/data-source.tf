data "polaris_principals" "all" {}

# Client IDs by principal name
output "client_ids" {
  value = { for p in data.polaris_principals.all.principals : p.name => p.client_id }
}
