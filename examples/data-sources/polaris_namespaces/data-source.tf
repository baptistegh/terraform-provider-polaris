data "polaris_namespaces" "under_sales" {
  catalog = "lakehouse"
  parent  = ["sales"]
}

# ["sales.eu", "sales.us", ...]
output "sales_namespaces" {
  value = [for ns in data.polaris_namespaces.under_sales.namespaces : join(".", ns)]
}
