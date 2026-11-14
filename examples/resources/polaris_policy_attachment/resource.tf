# Apply the policy to every table of the catalog...
resource "polaris_policy_attachment" "catalog" {
  catalog          = polaris_policy.compaction.catalog
  policy_namespace = polaris_policy.compaction.namespace
  policy_name      = polaris_policy.compaction.name
  target_type      = "catalog"
}

# ...or to a namespace...
resource "polaris_policy_attachment" "sales" {
  catalog          = polaris_policy.compaction.catalog
  policy_namespace = polaris_policy.compaction.namespace
  policy_name      = polaris_policy.compaction.name
  target_type      = "namespace"
  target_path      = ["sales"]
}

# ...or to a single table or view.
resource "polaris_policy_attachment" "orders" {
  catalog          = polaris_policy.compaction.catalog
  policy_namespace = polaris_policy.compaction.namespace
  policy_name      = polaris_policy.compaction.name
  target_type      = "table-like"
  target_path      = ["sales", "orders"]
}
