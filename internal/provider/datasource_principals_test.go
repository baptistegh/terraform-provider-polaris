package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourcePrincipals_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	clientID := os.Getenv("POLARIS_CLIENT_ID")
	const ds = "data.polaris_principals.all"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal" "test" {
  name       = "tf-acc-test-ds-principals"
  properties = { "team" = "platform" }
}

data "polaris_principals" "all" {
  depends_on = [polaris_principal.test]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckTypeSetElemAttr(ds, "names.*", clientID),
					resource.TestCheckTypeSetElemAttr(ds, "names.*", "tf-acc-test-ds-principals"),
					resource.TestCheckTypeSetElemAttrPair(ds, "principals.*.client_id", "polaris_principal.test", "client_id"),
					resource.TestCheckTypeSetElemNestedAttrs(ds, "principals.*", map[string]string{
						"name":            "tf-acc-test-ds-principals",
						"properties.team": "platform",
					}),
				),
			},
		},
	})
}
