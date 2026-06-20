package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourceRealm(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "polaris_realm" "current" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					// realm is computed — just verify the attribute is present and a string
					resource.TestCheckResourceAttrSet("data.polaris_realm.current", "realm"),
				),
			},
		},
	})
}
