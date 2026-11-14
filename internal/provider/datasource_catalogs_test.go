package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourceCatalogs_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	const ds = "data.polaris_catalogs.all"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "polaris_catalogs" "all" {}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckTypeSetElemAttr(ds, "names.*", catalog),
					resource.TestCheckTypeSetElemNestedAttrs(ds, "catalogs.*", map[string]string{
						"name":                        catalog,
						"type":                        "INTERNAL",
						"default_base_location":       "file:///tmp/" + catalog,
						"storage_config.storage_type": "FILE",
					}),
				),
			},
		},
	})
}
