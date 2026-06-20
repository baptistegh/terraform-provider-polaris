package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceCatalogRoleAssignment_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal_role" "test" {
  name = "tf-acc-test-role-cassign"
}

resource "polaris_catalog_role" "test" {
  catalog = "` + catalog + `"
  name    = "tf-acc-test-crole-cassign"
}

resource "polaris_catalog_role_assignment" "test" {
  principal_role = polaris_principal_role.test.name
  catalog        = polaris_catalog_role.test.catalog
  catalog_role   = polaris_catalog_role.test.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_catalog_role_assignment.test", "principal_role", "tf-acc-test-role-cassign"),
					resource.TestCheckResourceAttr("polaris_catalog_role_assignment.test", "catalog", catalog),
					resource.TestCheckResourceAttr("polaris_catalog_role_assignment.test", "catalog_role", "tf-acc-test-crole-cassign"),
				),
			},
			// Import
			{
				ResourceName:                        "polaris_catalog_role_assignment.test",
				ImportState:                         true,
				ImportStateId:                       "tf-acc-test-role-cassign/" + catalog + "/tf-acc-test-crole-cassign",
				ImportStateVerify:                   true,
				ImportStateVerifyIdentifierAttribute: "principal_role",
			},
		},
	})
}
