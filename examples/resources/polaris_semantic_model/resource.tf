resource "polaris_semantic_model" "revenue" {
  catalog      = "lakehouse"
  namespace    = ["sales"]
  name         = "revenue"
  spec_version = "0.2.0.dev0"

  # An Apache Ossie semantic model. Each dataset source must be an existing
  # table or view of the catalog, written <namespace>.<table>.
  document = jsonencode({
    name        = "revenue"
    description = "Revenue analytics"
    datasets = [{
      name        = "orders"
      source      = "sales.orders"
      primary_key = ["order_id"]
    }]
    metrics = [{
      name = "total_revenue"
      expression = {
        dialects = [{ dialect = "ANSI_SQL", expression = "SUM(amount)" }]
      }
    }]
  })
}
