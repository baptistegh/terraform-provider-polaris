package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceViewGrant_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"reporting"})
	testCreateView(t, catalog, []string{"reporting"}, "monthly_summary")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog = "` + catalog + `"
  name    = "tf-acc-test-crole-vgrant"
}

resource "polaris_view_grant" "test" {
  catalog      = polaris_catalog_role.test.catalog
  catalog_role = polaris_catalog_role.test.name
  namespace    = ["reporting"]
  view_name    = "monthly_summary"
  privilege    = "VIEW_READ_PROPERTIES"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_view_grant.test", "catalog", catalog),
					resource.TestCheckResourceAttr("polaris_view_grant.test", "catalog_role", "tf-acc-test-crole-vgrant"),
					resource.TestCheckResourceAttr("polaris_view_grant.test", "namespace.0", "reporting"),
					resource.TestCheckResourceAttr("polaris_view_grant.test", "view_name", "monthly_summary"),
					resource.TestCheckResourceAttr("polaris_view_grant.test", "privilege", "VIEW_READ_PROPERTIES"),
				),
			},
			// Import
			{
				ResourceName:                        "polaris_view_grant.test",
				ImportState:                         true,
				ImportStateId:                       catalog + "/tf-acc-test-crole-vgrant/VIEW_READ_PROPERTIES/reporting/monthly_summary",
				ImportStateVerify:                   true,
				ImportStateVerifyIdentifierAttribute: "privilege",
			},
		},
	})
}
