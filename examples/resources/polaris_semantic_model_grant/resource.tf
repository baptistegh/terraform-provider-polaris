resource "polaris_semantic_model_grant" "analysts_read_revenue" {
  catalog             = "lakehouse"
  catalog_role        = "analysts"
  namespace           = ["sales"]
  semantic_model_name = "revenue"
  privilege           = "SEMANTIC_MODEL_READ"
}
