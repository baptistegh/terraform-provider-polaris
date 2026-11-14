data "polaris_namespace" "sales" {
  catalog   = "lakehouse"
  namespace = ["sales"]
}

output "sales_location" {
  value = data.polaris_namespace.sales.location
}
