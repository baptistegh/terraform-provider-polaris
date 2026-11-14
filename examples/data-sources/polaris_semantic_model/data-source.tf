data "polaris_semantic_model" "revenue" {
  catalog   = "lakehouse"
  namespace = ["sales"]
  name      = "revenue"
}

output "revenue_metrics" {
  value = [for m in jsondecode(data.polaris_semantic_model.revenue.document).metrics : m.name]
}
