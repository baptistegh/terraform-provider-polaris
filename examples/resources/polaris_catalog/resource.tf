# Internal catalog on S3
resource "polaris_catalog" "lakehouse" {
  name                  = "lakehouse"
  default_base_location = "s3://my-bucket/lakehouse"

  properties = {
    "owner" = "data-platform"
  }

  storage_config = {
    storage_type      = "S3"
    role_arn          = "arn:aws:iam::123456789012:role/polaris-lakehouse"
    region            = "eu-west-1"
    allowed_locations = ["s3://my-bucket/lakehouse", "s3://my-bucket/shared"]
  }
}

# External catalog federated to a remote Iceberg REST catalog
resource "polaris_catalog" "remote" {
  name                  = "remote"
  type                  = "EXTERNAL"
  default_base_location = "s3://my-bucket/remote"

  storage_config = {
    storage_type = "S3"
    role_arn     = "arn:aws:iam::123456789012:role/polaris-remote"
  }

  connection_config = {
    connection_type     = "ICEBERG_REST"
    uri                 = "https://remote.example.com/api/catalog"
    remote_catalog_name = "analytics"

    authentication = {
      type          = "OAUTH"
      token_uri     = "https://remote.example.com/api/catalog/v1/oauth/tokens"
      client_id     = var.remote_client_id
      client_secret = var.remote_client_secret
      scopes        = ["PRINCIPAL_ROLE:ALL"]
    }
  }
}
