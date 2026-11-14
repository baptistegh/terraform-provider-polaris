package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourcePrincipalRoles_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	const ds = "data.polaris_principal_roles.all"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal_role" "test" {
  name       = "tf-acc-test-ds-proles"
  properties = { "team" = "platform" }
}

data "polaris_principal_roles" "all" {
  depends_on = [polaris_principal_role.test]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckTypeSetElemAttr(ds, "names.*", "service_admin"),
					resource.TestCheckTypeSetElemAttr(ds, "names.*", "tf-acc-test-ds-proles"),
					resource.TestCheckTypeSetElemNestedAttrs(ds, "roles.*", map[string]string{
						"name":            "tf-acc-test-ds-proles",
						"federated":       "false",
						"properties.team": "platform",
					}),
				),
			},
		},
	})
}
