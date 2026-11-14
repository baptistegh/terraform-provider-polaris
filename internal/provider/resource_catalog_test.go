package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccResourceCatalog_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const res = "polaris_catalog.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Create with defaults: type INTERNAL, no properties, allowed_locations
			// filled in by the server from default_base_location.
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  default_base_location = "file:///tmp/` + name + `"

  storage_config = {
    storage_type = "FILE"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "name", name),
					resource.TestCheckResourceAttr(res, "type", "INTERNAL"),
					resource.TestCheckResourceAttr(res, "default_base_location", "file:///tmp/"+name),
					resource.TestCheckNoResourceAttr(res, "properties.%"),
					resource.TestCheckResourceAttr(res, "storage_config.storage_type", "FILE"),
					resource.TestCheckTypeSetElemAttr(res, "storage_config.allowed_locations.*", "file:///tmp/"+name),
					resource.TestCheckNoResourceAttr(res, "connection_config"),
					resource.TestCheckResourceAttrSet(res, "create_timestamp"),
					resource.TestCheckResourceAttr(res, "entity_version", "1"),
				),
			},
			// Update in place: properties, base location and allowed locations.
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  default_base_location = "file:///tmp/` + name + `/v2"
  properties            = { "env" = "test" }

  storage_config = {
    storage_type      = "FILE"
    allowed_locations = ["file:///tmp/` + name + `/v2", "file:///tmp/` + name + `-extra"]
  }
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "default_base_location", "file:///tmp/"+name+"/v2"),
					resource.TestCheckResourceAttr(res, "properties.%", "1"),
					resource.TestCheckResourceAttr(res, "properties.env", "test"),
					resource.TestCheckResourceAttr(res, "storage_config.allowed_locations.#", "2"),
					resource.TestCheckTypeSetElemAttr(res, "storage_config.allowed_locations.*", "file:///tmp/"+name+"-extra"),
					resource.TestCheckResourceAttr(res, "entity_version", "2"),
				),
			},
			// Import
			{
				ResourceName:                         res,
				ImportState:                          true,
				ImportStateId:                        name,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			// Removing all properties stores an empty map, not a diff on every plan.
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  default_base_location = "file:///tmp/` + name + `/v2"
  properties            = {}

  storage_config = {
    storage_type      = "FILE"
    allowed_locations = ["file:///tmp/` + name + `/v2", "file:///tmp/` + name + `-extra"]
  }
}
`,
				Check: resource.TestCheckResourceAttr(res, "properties.%", "0"),
			},
		},
	})
}

func TestAccResourceCatalog_s3(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const res = "polaris_catalog.test"
	config := func(region, pathStyle string) string {
		return testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  default_base_location = "s3://bucket/` + name + `"

  storage_config = {
    storage_type      = "S3"
    role_arn          = "arn:aws:iam::123456789012:role/polaris"
    region            = "` + region + `"
    path_style_access = ` + pathStyle + `
    endpoint          = "http://minio:9000"
  }
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: config("us-east-1", "true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "storage_config.storage_type", "S3"),
					resource.TestCheckResourceAttr(res, "storage_config.role_arn", "arn:aws:iam::123456789012:role/polaris"),
					resource.TestCheckResourceAttr(res, "storage_config.region", "us-east-1"),
					resource.TestCheckResourceAttr(res, "storage_config.path_style_access", "true"),
					resource.TestCheckResourceAttr(res, "storage_config.endpoint", "http://minio:9000"),
					// Server defaults for settings that were not configured.
					resource.TestCheckResourceAttr(res, "storage_config.encryption_keys.#", "0"),
					resource.TestCheckNoResourceAttr(res, "storage_config.tenant_id"),
				),
			},
			{
				Config: config("us-west-2", "false"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "storage_config.region", "us-west-2"),
					resource.TestCheckResourceAttr(res, "storage_config.path_style_access", "false"),
				),
			},
			{
				ResourceName:                         res,
				ImportState:                          true,
				ImportStateId:                        name,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

func TestAccResourceCatalog_azureAndGcs(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog" "azure" {
  name                  = "` + name + `-az"
  default_base_location = "abfss://container@account.dfs.core.windows.net/` + name + `"

  storage_config = {
    storage_type = "AZURE"
    tenant_id    = "00000000-0000-0000-0000-000000000000"
    hierarchical = true
  }
}

resource "polaris_catalog" "gcs" {
  name                  = "` + name + `-gcs"
  default_base_location = "gs://bucket/` + name + `"

  storage_config = {
    storage_type = "GCS"
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_catalog.azure", "storage_config.storage_type", "AZURE"),
					resource.TestCheckResourceAttr("polaris_catalog.azure", "storage_config.tenant_id", "00000000-0000-0000-0000-000000000000"),
					resource.TestCheckResourceAttr("polaris_catalog.azure", "storage_config.hierarchical", "true"),
					resource.TestCheckNoResourceAttr("polaris_catalog.azure", "storage_config.role_arn"),
					resource.TestCheckResourceAttr("polaris_catalog.gcs", "storage_config.storage_type", "GCS"),
					resource.TestCheckTypeSetElemAttr("polaris_catalog.gcs", "storage_config.allowed_locations.*", "gs://bucket/"+name),
				),
			},
		},
	})
}

// Polaris cannot change the storage type in place, so the provider replaces the catalog.
func TestAccResourceCatalog_storageTypeChangeReplaces(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const res = "polaris_catalog.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  default_base_location = "file:///tmp/` + name + `"
  storage_config        = { storage_type = "FILE" }
}
`,
			},
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  default_base_location = "gs://bucket/` + name + `"
  storage_config        = { storage_type = "GCS" }
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr(res, "storage_config.storage_type", "GCS"),
			},
		},
	})
}

func TestAccResourceCatalog_external(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const res = "polaris_catalog.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  type                  = "EXTERNAL"
  default_base_location = "file:///tmp/` + name + `"
  storage_config        = { storage_type = "FILE" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "type", "EXTERNAL"),
					resource.TestCheckNoResourceAttr(res, "connection_config"),
				),
			},
			{
				ResourceName:                         res,
				ImportState:                          true,
				ImportStateId:                        name,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}

// Federated catalogs need ENABLE_CATALOG_FEDERATION (set in docker-compose.yml).
// Polaris does not contact the remote service when the catalog is created.
func TestAccResourceCatalog_federatedOAuth(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const res = "polaris_catalog.test"
	config := func(secret string) string {
		return testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  type                  = "EXTERNAL"
  default_base_location = "file:///tmp/` + name + `"
  storage_config        = { storage_type = "FILE" }

  connection_config = {
    connection_type     = "ICEBERG_REST"
    uri                 = "https://remote.example.com/api/catalog"
    remote_catalog_name = "remote"

    authentication = {
      type          = "OAUTH"
      token_uri     = "https://remote.example.com/api/catalog/v1/oauth/tokens"
      client_id     = "client"
      client_secret = "` + secret + `"
      scopes        = ["PRINCIPAL_ROLE:ALL"]
    }
  }
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: config("s3cr3t"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "connection_config.connection_type", "ICEBERG_REST"),
					resource.TestCheckResourceAttr(res, "connection_config.uri", "https://remote.example.com/api/catalog"),
					resource.TestCheckResourceAttr(res, "connection_config.remote_catalog_name", "remote"),
					resource.TestCheckResourceAttr(res, "connection_config.authentication.type", "OAUTH"),
					resource.TestCheckResourceAttr(res, "connection_config.authentication.client_id", "client"),
					resource.TestCheckResourceAttr(res, "connection_config.authentication.scopes.0", "PRINCIPAL_ROLE:ALL"),
					// The API never returns the secret: it is kept from the configuration.
					resource.TestCheckResourceAttr(res, "connection_config.authentication.client_secret", "s3cr3t"),
					resource.TestCheckNoResourceAttr(res, "connection_config.properties.%"),
				),
			},
			// A new secret cannot be sent to an existing catalog: the catalog is replaced.
			{
				Config: config("rotated"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr(res, "connection_config.authentication.client_secret", "rotated"),
			},
			{
				ResourceName:                         res,
				ImportState:                          true,
				ImportStateId:                        name,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"connection_config.authentication.client_secret"},
			},
		},
	})
}

func TestAccResourceCatalog_federatedBearer(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const res = "polaris_catalog.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  type                  = "EXTERNAL"
  default_base_location = "file:///tmp/` + name + `"
  storage_config        = { storage_type = "FILE" }

  connection_config = {
    connection_type     = "ICEBERG_REST"
    uri                 = "https://remote.example.com/api/catalog"
    remote_catalog_name = "remote"
    properties          = { "rest.client.timeout" = "30" }

    authentication = {
      type         = "BEARER"
      bearer_token = "token"
    }
  }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "connection_config.authentication.type", "BEARER"),
					resource.TestCheckResourceAttr(res, "connection_config.authentication.bearer_token", "token"),
					resource.TestCheckResourceAttr(res, "connection_config.properties.rest.client.timeout", "30"),
				),
			},
			// Updating the catalog properties must not touch (or replace) the connection.
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  type                  = "EXTERNAL"
  default_base_location = "file:///tmp/` + name + `"
  properties            = { "owner" = "data" }
  storage_config        = { storage_type = "FILE" }

  connection_config = {
    connection_type     = "ICEBERG_REST"
    uri                 = "https://remote.example.com/api/catalog"
    remote_catalog_name = "remote"
    properties          = { "rest.client.timeout" = "30" }

    authentication = {
      type         = "BEARER"
      bearer_token = "token"
    }
  }
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "properties.owner", "data"),
					resource.TestCheckResourceAttr(res, "connection_config.authentication.bearer_token", "token"),
				),
			},
		},
	})
}
