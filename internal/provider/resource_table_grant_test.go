package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceTableGrant_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"sales"})
	testCreateTable(t, catalog, []string{"sales"}, "orders")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog = "` + catalog + `"
  name    = "tf-acc-test-crole-tgrant"
}

resource "polaris_table_grant" "test" {
  catalog      = polaris_catalog_role.test.catalog
  catalog_role = polaris_catalog_role.test.name
  namespace    = ["sales"]
  table_name   = "orders"
  privilege    = "TABLE_READ_DATA"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_table_grant.test", "catalog", catalog),
					resource.TestCheckResourceAttr("polaris_table_grant.test", "catalog_role", "tf-acc-test-crole-tgrant"),
					resource.TestCheckResourceAttr("polaris_table_grant.test", "namespace.0", "sales"),
					resource.TestCheckResourceAttr("polaris_table_grant.test", "table_name", "orders"),
					resource.TestCheckResourceAttr("polaris_table_grant.test", "privilege", "TABLE_READ_DATA"),
				),
			},
			// Import
			{
				ResourceName:                         "polaris_table_grant.test",
				ImportState:                          true,
				ImportStateId:                        catalog + "/tf-acc-test-crole-tgrant/TABLE_READ_DATA/sales/orders",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "privilege",
			},
		},
	})
}
