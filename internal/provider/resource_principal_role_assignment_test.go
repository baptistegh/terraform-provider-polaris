package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourcePrincipalRoleAssignment_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal" "test" {
  name = "tf-acc-test-principal-assign"
}

resource "polaris_principal_role" "test" {
  name = "tf-acc-test-role-assign"
}

resource "polaris_principal_role_assignment" "test" {
  principal      = polaris_principal.test.name
  principal_role = polaris_principal_role.test.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_principal_role_assignment.test", "principal", "tf-acc-test-principal-assign"),
					resource.TestCheckResourceAttr("polaris_principal_role_assignment.test", "principal_role", "tf-acc-test-role-assign"),
				),
			},
			// Import
			{
				ResourceName:                        "polaris_principal_role_assignment.test",
				ImportState:                         true,
				ImportStateId:                       "tf-acc-test-principal-assign/tf-acc-test-role-assign",
				ImportStateVerify:                   true,
				ImportStateVerifyIdentifierAttribute: "principal",
			},
		},
	})
}
