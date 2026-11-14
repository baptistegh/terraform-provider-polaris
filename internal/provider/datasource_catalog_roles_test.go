package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourceCatalogRoles_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	const ds = "data.polaris_catalog_roles.all"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog    = "` + catalog + `"
  name       = "tf-acc-test-ds-croles"
  properties = { "team" = "analytics" }
}

data "polaris_catalog_roles" "all" {
  catalog = polaris_catalog_role.test.catalog
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "names.#", "2"),
					resource.TestCheckTypeSetElemAttr(ds, "names.*", "catalog_admin"),
					resource.TestCheckTypeSetElemAttr(ds, "names.*", "tf-acc-test-ds-croles"),
					resource.TestCheckTypeSetElemNestedAttrs(ds, "roles.*", map[string]string{
						"name":            "tf-acc-test-ds-croles",
						"properties.team": "analytics",
					}),
				),
			},
		},
	})
}
