data "polaris_policy" "compaction" {
  catalog   = "lakehouse"
  namespace = ["governance"]
  name      = "nightly-compaction"
}

output "compaction_config" {
  value = jsondecode(data.polaris_policy.compaction.content)
}
