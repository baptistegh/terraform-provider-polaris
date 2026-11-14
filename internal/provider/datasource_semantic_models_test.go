package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourceSemanticModels_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"sales"})
	testCreateNamespace(t, catalog, []string{"empty"})
	testCreateSemanticModel(t, catalog, []string{"sales"}, "revenue")
	testCreateSemanticModel(t, catalog, []string{"sales"}, "margin")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "polaris_semantic_models" "sales" {
  catalog   = "` + catalog + `"
  namespace = ["sales"]
}

data "polaris_semantic_models" "empty" {
  catalog   = "` + catalog + `"
  namespace = ["empty"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.polaris_semantic_models.sales", "names.#", "2"),
					resource.TestCheckTypeSetElemAttr("data.polaris_semantic_models.sales", "names.*", "revenue"),
					resource.TestCheckTypeSetElemAttr("data.polaris_semantic_models.sales", "names.*", "margin"),
					resource.TestCheckResourceAttr("data.polaris_semantic_models.empty", "names.#", "0"),
				),
			},
		},
	})
}
