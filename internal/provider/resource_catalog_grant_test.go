package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceCatalogGrant_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog = "` + catalog + `"
  name    = "tf-acc-test-crole-cgrant"
}

resource "polaris_catalog_grant" "test" {
  catalog      = polaris_catalog_role.test.catalog
  catalog_role = polaris_catalog_role.test.name
  privilege    = "CATALOG_READ_PROPERTIES"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_catalog_grant.test", "catalog", catalog),
					resource.TestCheckResourceAttr("polaris_catalog_grant.test", "catalog_role", "tf-acc-test-crole-cgrant"),
					resource.TestCheckResourceAttr("polaris_catalog_grant.test", "privilege", "CATALOG_READ_PROPERTIES"),
				),
			},
			// Import
			{
				ResourceName:                        "polaris_catalog_grant.test",
				ImportState:                         true,
				ImportStateId:                       catalog + "/tf-acc-test-crole-cgrant/CATALOG_READ_PROPERTIES",
				ImportStateVerify:                   true,
				ImportStateVerifyIdentifierAttribute: "privilege",
			},
		},
	})
}
