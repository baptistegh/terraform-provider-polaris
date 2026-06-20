package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceCatalogRole_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog = "` + catalog + `"
  name    = "tf-acc-test-catalog-role"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_catalog_role.test", "catalog", catalog),
					resource.TestCheckResourceAttr("polaris_catalog_role.test", "name", "tf-acc-test-catalog-role"),
					resource.TestCheckResourceAttrSet("polaris_catalog_role.test", "entity_version"),
				),
			},
			// Update properties
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog    = "` + catalog + `"
  name       = "tf-acc-test-catalog-role"
  properties = { "env" = "test" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_catalog_role.test", "properties.env", "test"),
				),
			},
			// Import
			{
				ResourceName:                        "polaris_catalog_role.test",
				ImportState:                         true,
				ImportStateId:                       catalog + "/tf-acc-test-catalog-role",
				ImportStateVerify:                   true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}
