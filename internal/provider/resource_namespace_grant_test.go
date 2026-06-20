package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceNamespaceGrant_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"sales"})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog = "` + catalog + `"
  name    = "tf-acc-test-crole-nsgrant"
}

resource "polaris_namespace_grant" "test" {
  catalog      = polaris_catalog_role.test.catalog
  catalog_role = polaris_catalog_role.test.name
  namespace    = ["sales"]
  privilege    = "NAMESPACE_LIST"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_namespace_grant.test", "catalog", catalog),
					resource.TestCheckResourceAttr("polaris_namespace_grant.test", "catalog_role", "tf-acc-test-crole-nsgrant"),
					resource.TestCheckResourceAttr("polaris_namespace_grant.test", "namespace.0", "sales"),
					resource.TestCheckResourceAttr("polaris_namespace_grant.test", "privilege", "NAMESPACE_LIST"),
				),
			},
			// Import
			{
				ResourceName:                        "polaris_namespace_grant.test",
				ImportState:                         true,
				ImportStateId:                       catalog + "/tf-acc-test-crole-nsgrant/NAMESPACE_LIST/sales",
				ImportStateVerify:                   true,
				ImportStateVerifyIdentifierAttribute: "privilege",
			},
		},
	})
}
