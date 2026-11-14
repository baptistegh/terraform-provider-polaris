package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// testSemanticModelConfig returns a polaris_semantic_model "test" in sales over the
// sales.orders table, with the given extra document fields.
func testSemanticModelConfig(catalog, extra string) string {
	return testAccProviderConfig() + `
resource "polaris_semantic_model" "test" {
  catalog      = "` + catalog + `"
  namespace    = ["sales"]
  name         = "revenue"
  spec_version = "0.2.0.dev0"

  document = jsonencode({
    name     = "revenue"
    datasets = [{ name = "orders", source = "sales.orders" }]
    ` + extra + `
  })
}
`
}

// Semantic models need ENABLE_SEMANTIC_MODELS (set in docker-compose.yml).
func TestAccResourceSemanticModel_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"sales"})
	testCreateTable(t, catalog, []string{"sales"}, "orders")
	const res = "polaris_semantic_model.test"
	var firstVersion string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testSemanticModelConfig(catalog, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "name", "revenue"),
					resource.TestCheckResourceAttr(res, "namespace.0", "sales"),
					resource.TestCheckResourceAttr(res, "spec_version", "0.2.0.dev0"),
					resource.TestCheckResourceAttr(res, "document", `{"datasets":[{"name":"orders","source":"sales.orders"}],"name":"revenue"}`),
					resource.TestCheckResourceAttrSet(res, "entity_version"),
					captureAttr(res, "entity_version", &firstVersion),
				),
			},
			// The document changes in place; the entity version moves on.
			{
				Config: testSemanticModelConfig(catalog, `description = "Revenue by order"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "document", `{"datasets":[{"name":"orders","source":"sales.orders"}],"description":"Revenue by order","name":"revenue"}`),
					resource.TestCheckResourceAttrWith(res, "entity_version", func(v string) error {
						if v == firstVersion {
							return fmt.Errorf("entity_version did not change after the update")
						}
						return nil
					}),
				),
			},
			{
				ResourceName:                         res,
				ImportState:                          true,
				ImportStateId:                        catalog + "/sales/revenue",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			{
				Config: testSemanticModelConfig(catalog, `description = "Revenue by order"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Deleted outside Terraform: the next plan re-creates it.
			{
				PreConfig: func() {
					if err := dropSemanticModel(context.Background(), testClient(t), catalog, []string{"sales"}, "revenue"); err != nil {
						t.Fatalf("drop: %v", err)
					}
				},
				Config: testSemanticModelConfig(catalog, `description = "Revenue by order"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionCreate)},
				},
			},
		},
	})
}

// Polaris checks that every dataset source is an existing table or view of the catalog.
func TestAccResourceSemanticModel_unknownSource(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"sales"})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:      testSemanticModelConfig(catalog, ""), // sales.orders does not exist
				ExpectError: regexp.MustCompile(`does\s+not\s+resolve\s+to\s+a\s+table\s+or\s+view`),
			},
		},
	})
}
