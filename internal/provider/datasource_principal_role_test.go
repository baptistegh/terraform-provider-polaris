package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourcePrincipalRole_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	const ds = "data.polaris_principal_role.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal_role" "test" {
  name       = "tf-acc-test-ds-prole"
  properties = { "team" = "data-engineering" }
}

data "polaris_principal_role" "test" {
  name = polaris_principal_role.test.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "name", "tf-acc-test-ds-prole"),
					resource.TestCheckResourceAttr(ds, "properties.team", "data-engineering"),
					resource.TestCheckResourceAttr(ds, "federated", "false"),
					resource.TestCheckResourceAttrPair(ds, "entity_version", "polaris_principal_role.test", "entity_version"),
				),
			},
		},
	})
}

func TestAccDatasourcePrincipalRole_notFound(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "polaris_principal_role" "missing" {
  name = "tf-acc-test-does-not-exist"
}
`,
				ExpectError: regexp.MustCompile(`Principal role not found`),
			},
		},
	})
}
