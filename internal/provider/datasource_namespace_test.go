package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourceNamespace_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	const ds = "data.polaris_namespace.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_namespace" "test" {
  catalog    = "` + catalog + `"
  namespace  = ["sales"]
  properties = { "owner" = "data" }
}

data "polaris_namespace" "test" {
  catalog   = polaris_namespace.test.catalog
  namespace = polaris_namespace.test.namespace
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "namespace.0", "sales"),
					resource.TestCheckResourceAttr(ds, "properties.%", "1"),
					resource.TestCheckResourceAttr(ds, "properties.owner", "data"),
					resource.TestCheckResourceAttrPair(ds, "location", "polaris_namespace.test", "location"),
				),
			},
		},
	})
}

// A namespace without properties reads back an empty map, so lookup() works without a null check.
func TestAccDatasourceNamespace_withoutProperties(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"bare"})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "polaris_namespace" "bare" {
  catalog   = "` + catalog + `"
  namespace = ["bare"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.polaris_namespace.bare", "properties.%", "0"),
					resource.TestCheckResourceAttr("data.polaris_namespace.bare", "location", "file:///tmp/"+catalog+"/bare/"),
				),
			},
			{
				Config: testAccProviderConfig() + `
data "polaris_namespace" "missing" {
  catalog   = "` + catalog + `"
  namespace = ["does", "not", "exist"]
}
`,
				ExpectError: regexp.MustCompile(`Namespace not found`),
			},
		},
	})
}
