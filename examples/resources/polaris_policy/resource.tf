resource "polaris_namespace" "governance" {
  catalog   = "lakehouse"
  namespace = ["governance"]
}

resource "polaris_policy" "compaction" {
  catalog     = polaris_namespace.governance.catalog
  namespace   = polaris_namespace.governance.namespace
  name        = "nightly-compaction"
  type        = "system.data-compaction"
  description = "Compact small files every night"

  content = jsonencode({
    version = "2025-02-03"
    enable  = true
    config = {
      "target_file_size_bytes" = 134217728
    }
  })
}
