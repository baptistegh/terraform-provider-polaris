package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourcePrincipal_byName(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	clientID := os.Getenv("POLARIS_CLIENT_ID")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "polaris_principal" "test" {
  name = "` + clientID + `"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.polaris_principal.test", "name", clientID),
					resource.TestCheckResourceAttr("data.polaris_principal.test", "client_id", clientID),
					resource.TestCheckResourceAttrSet("data.polaris_principal.test", "entity_version"),
				),
			},
		},
	})
}

func TestAccDatasourcePrincipal_defaultsToProviderClientID(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	clientID := os.Getenv("POLARIS_CLIENT_ID")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				// No name attribute — should resolve to the provider's client_id.
				Config: testAccProviderConfig() + `
data "polaris_principal" "self" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.polaris_principal.self", "name", clientID),
					resource.TestCheckResourceAttrSet("data.polaris_principal.self", "client_id"),
					resource.TestCheckResourceAttrSet("data.polaris_principal.self", "entity_version"),
				),
			},
		},
	})
}
