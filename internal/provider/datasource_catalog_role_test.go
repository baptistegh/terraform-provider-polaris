package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourceCatalogRole_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	const ds = "data.polaris_catalog_role.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog    = "` + catalog + `"
  name       = "tf-acc-test-ds-crole"
  properties = { "team" = "analytics" }
}

data "polaris_catalog_role" "test" {
  catalog = polaris_catalog_role.test.catalog
  name    = polaris_catalog_role.test.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "catalog", catalog),
					resource.TestCheckResourceAttr(ds, "name", "tf-acc-test-ds-crole"),
					resource.TestCheckResourceAttr(ds, "properties.team", "analytics"),
					resource.TestCheckResourceAttrPair(ds, "entity_version", "polaris_catalog_role.test", "entity_version"),
					resource.TestCheckResourceAttrSet(ds, "create_timestamp"),
				),
			},
		},
	})
}

// The built-in catalog_admin role has no properties: they read back as an empty map.
func TestAccDatasourceCatalogRole_builtinWithoutProperties(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "polaris_catalog_role" "admin" {
  catalog = "` + catalog + `"
  name    = "catalog_admin"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.polaris_catalog_role.admin", "name", "catalog_admin"),
					resource.TestCheckResourceAttr("data.polaris_catalog_role.admin", "properties.%", "0"),
				),
			},
			{
				Config: testAccProviderConfig() + `
data "polaris_catalog_role" "missing" {
  catalog = "` + catalog + `"
  name    = "does-not-exist"
}
`,
				ExpectError: regexp.MustCompile(`Catalog role not found`),
			},
		},
	})
}
