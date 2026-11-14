package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceSemanticModelGrant_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"sales"})
	testCreateSemanticModel(t, catalog, []string{"sales"}, "revenue")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog = "` + catalog + `"
  name    = "tf-acc-test-crole-smgrant"
}

resource "polaris_semantic_model_grant" "test" {
  catalog             = polaris_catalog_role.test.catalog
  catalog_role        = polaris_catalog_role.test.name
  namespace           = ["sales"]
  semantic_model_name = "revenue"
  privilege           = "SEMANTIC_MODEL_READ"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_semantic_model_grant.test", "catalog", catalog),
					resource.TestCheckResourceAttr("polaris_semantic_model_grant.test", "catalog_role", "tf-acc-test-crole-smgrant"),
					resource.TestCheckResourceAttr("polaris_semantic_model_grant.test", "namespace.0", "sales"),
					resource.TestCheckResourceAttr("polaris_semantic_model_grant.test", "semantic_model_name", "revenue"),
					resource.TestCheckResourceAttr("polaris_semantic_model_grant.test", "privilege", "SEMANTIC_MODEL_READ"),
				),
			},
			// Changing the privilege replaces the grant.
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog = "` + catalog + `"
  name    = "tf-acc-test-crole-smgrant"
}

resource "polaris_semantic_model_grant" "test" {
  catalog             = polaris_catalog_role.test.catalog
  catalog_role        = polaris_catalog_role.test.name
  namespace           = ["sales"]
  semantic_model_name = "revenue"
  privilege           = "SEMANTIC_MODEL_WRITE"
}
`,
				Check: resource.TestCheckResourceAttr("polaris_semantic_model_grant.test", "privilege", "SEMANTIC_MODEL_WRITE"),
			},
			// Import
			{
				ResourceName:                         "polaris_semantic_model_grant.test",
				ImportState:                          true,
				ImportStateId:                        catalog + "/tf-acc-test-crole-smgrant/SEMANTIC_MODEL_WRITE/sales/revenue",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "privilege",
			},
		},
	})
}
