resource "polaris_principal" "spark" {
  name = "spark-job"
}

# Issue new credentials every month, and whenever the principal is re-created.
resource "time_rotating" "monthly" {
  rotation_months = 1
}

resource "polaris_principal_credentials" "spark" {
  principal = polaris_principal.spark.name

  rotation_triggers = {
    principal_id = polaris_principal.spark.client_id
    rotated_on   = time_rotating.monthly.id
  }
}

output "spark_client_secret" {
  value     = polaris_principal_credentials.spark.client_secret
  sensitive = true
}
