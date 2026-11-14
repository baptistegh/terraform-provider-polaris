package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourceSemanticModel_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"sales"})
	testCreateTable(t, catalog, []string{"sales"}, "orders")
	const ds = "data.polaris_semantic_model.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testSemanticModelConfig(catalog, "") + `
data "polaris_semantic_model" "test" {
  catalog   = polaris_semantic_model.test.catalog
  namespace = polaris_semantic_model.test.namespace
  name      = polaris_semantic_model.test.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "spec_version", "0.2.0.dev0"),
					resource.TestCheckResourceAttrPair(ds, "document", "polaris_semantic_model.test", "document"),
					resource.TestCheckResourceAttrPair(ds, "entity_version", "polaris_semantic_model.test", "entity_version"),
				),
			},
			{
				Config: testAccProviderConfig() + `
data "polaris_semantic_model" "missing" {
  catalog   = "` + catalog + `"
  namespace = ["sales"]
  name      = "does-not-exist"
}
`,
				ExpectError: regexp.MustCompile(`Semantic model not found`),
			},
		},
	})
}
