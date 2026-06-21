package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourcePrincipal_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Create
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal" "test" {
  name = "tf-acc-test-principal"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_principal.test", "name", "tf-acc-test-principal"),
					resource.TestCheckResourceAttrSet("polaris_principal.test", "client_id"),
					resource.TestCheckResourceAttrSet("polaris_principal.test", "client_secret"),
					resource.TestCheckResourceAttrSet("polaris_principal.test", "entity_version"),
				),
			},
			// Update properties
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal" "test" {
  name       = "tf-acc-test-principal"
  properties = { "env" = "test" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_principal.test", "name", "tf-acc-test-principal"),
					resource.TestCheckResourceAttr("polaris_principal.test", "properties.env", "test"),
				),
			},
			// Import
			{
				ResourceName:                         "polaris_principal.test",
				ImportState:                          true,
				ImportStateId:                        "tf-acc-test-principal",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
				ImportStateVerifyIgnore:              []string{"client_secret", "credential_rotation_required"},
			},
		},
	})
}

func TestAccResourcePrincipal_credentialRotation(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal" "rotated" {
  name                        = "tf-acc-test-principal-rotate"
  credential_rotation_required = true
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_principal.rotated", "name", "tf-acc-test-principal-rotate"),
					resource.TestCheckResourceAttr("polaris_principal.rotated", "credential_rotation_required", "true"),
					resource.TestCheckResourceAttrSet("polaris_principal.rotated", "client_id"),
					resource.TestCheckResourceAttrSet("polaris_principal.rotated", "client_secret"),
				),
			},
		},
	})
}
