data "polaris_policies" "compaction" {
  catalog     = "lakehouse"
  namespace   = ["governance"]
  policy_type = "system.data-compaction"
}

output "compaction_policies" {
  value = data.polaris_policies.compaction.names
}
