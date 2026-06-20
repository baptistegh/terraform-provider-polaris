package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourcePrincipalRole_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal_role" "test" {
  name = "tf-acc-test-principal-role"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_principal_role.test", "name", "tf-acc-test-principal-role"),
					resource.TestCheckResourceAttrSet("polaris_principal_role.test", "entity_version"),
				),
			},
			// Update properties
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal_role" "test" {
  name       = "tf-acc-test-principal-role"
  properties = { "env" = "test" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_principal_role.test", "properties.env", "test"),
				),
			},
			// Import
			{
				ResourceName:                        "polaris_principal_role.test",
				ImportState:                         true,
				ImportStateId:                       "tf-acc-test-principal-role",
				ImportStateVerify:                   true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
		},
	})
}
