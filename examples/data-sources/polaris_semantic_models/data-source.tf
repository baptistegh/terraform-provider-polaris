data "polaris_semantic_models" "sales" {
  catalog   = "lakehouse"
  namespace = ["sales"]
}

output "sales_semantic_models" {
  value = data.polaris_semantic_models.sales.names
}
