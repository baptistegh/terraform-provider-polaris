resource "polaris_namespace" "sales" {
  catalog   = "lakehouse"
  namespace = ["sales"]

  properties = {
    "owner" = "sales-analytics"
  }
}

# Nested namespace: reference the parent so it is created first.
resource "polaris_namespace" "sales_eu" {
  catalog   = polaris_namespace.sales.catalog
  namespace = concat(polaris_namespace.sales.namespace, ["eu"])
}
