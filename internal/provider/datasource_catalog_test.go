package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourceCatalog_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const ds, res = "data.polaris_catalog.test", "polaris_catalog.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  default_base_location = "s3://bucket/` + name + `"
  properties            = { "owner" = "data" }

  storage_config = {
    storage_type = "S3"
    role_arn     = "arn:aws:iam::123456789012:role/polaris"
    region       = "eu-west-1"
  }
}

data "polaris_catalog" "test" {
  name = polaris_catalog.test.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "name", name),
					resource.TestCheckResourceAttr(ds, "type", "INTERNAL"),
					resource.TestCheckResourceAttrPair(ds, "default_base_location", res, "default_base_location"),
					resource.TestCheckResourceAttr(ds, "properties.owner", "data"),
					resource.TestCheckResourceAttr(ds, "storage_config.storage_type", "S3"),
					resource.TestCheckResourceAttrPair(ds, "storage_config.role_arn", res, "storage_config.role_arn"),
					resource.TestCheckResourceAttr(ds, "storage_config.region", "eu-west-1"),
					resource.TestCheckTypeSetElemAttr(ds, "storage_config.allowed_locations.*", "s3://bucket/"+name),
					resource.TestCheckResourceAttrPair(ds, "entity_version", res, "entity_version"),
					resource.TestCheckResourceAttrPair(ds, "create_timestamp", res, "create_timestamp"),
					resource.TestCheckNoResourceAttr(ds, "connection_config"),
				),
			},
		},
	})
}

// A catalog without properties reads back an empty map, so lookup() works without a null check.
func TestAccDatasourceCatalog_federatedWithoutProperties(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const ds = "data.polaris_catalog.test"

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
    authentication = {
      type         = "BEARER"
      bearer_token = "token"
    }
  }
}

data "polaris_catalog" "test" {
  name = polaris_catalog.test.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "type", "EXTERNAL"),
					resource.TestCheckResourceAttr(ds, "properties.%", "0"),
					resource.TestCheckResourceAttr(ds, "connection_config.connection_type", "ICEBERG_REST"),
					resource.TestCheckResourceAttr(ds, "connection_config.remote_catalog_name", "remote"),
					resource.TestCheckResourceAttr(ds, "connection_config.authentication.type", "BEARER"),
					// Secrets are never returned by the API.
					resource.TestCheckNoResourceAttr(ds, "connection_config.authentication.bearer_token"),
				),
			},
		},
	})
}

func TestAccDatasourceCatalog_notFound(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "polaris_catalog" "missing" {
  name = "` + acctest.RandomWithPrefix("tf-acc-missing") + `"
}
`,
				ExpectError: regexp.MustCompile(`Catalog not found`),
			},
		},
	})
}
