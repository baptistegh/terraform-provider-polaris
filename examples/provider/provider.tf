# Option A: OAuth2 client credentials (provider fetches the token)
provider "polaris" {
  base_url      = "https://polaris.example.com"
  client_id     = "my-client-id"
  client_secret = "my-client-secret"
}

# Option B: pre-fetched Bearer token
provider "polaris" {
  base_url = "https://polaris.example.com"
  token    = "eyJ..."
}

# Multi-tenant deployment (realm routing required)
provider "polaris" {
  base_url      = "https://polaris.example.com"
  client_id     = "my-client-id"
  client_secret = "my-client-secret"
  realm         = "my-realm"
}
