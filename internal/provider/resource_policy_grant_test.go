package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourcePolicyGrant_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	// Polaris requires the policy to exist before a grant can be applied to it.
	// Policies are created via a separate Polaris governance API not covered by this provider.
	// This test is skipped until a testCreatePolicy helper is available.
	t.Skip("polaris_policy_grant requires a pre-existing policy; skipped until testCreatePolicy helper is implemented")

	catalog := testCreateCatalog(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_catalog_role" "test" {
  catalog = "` + catalog + `"
  name    = "tf-acc-test-crole-pgrant"
}

resource "polaris_policy_grant" "test" {
  catalog      = polaris_catalog_role.test.catalog
  catalog_role = polaris_catalog_role.test.name
  namespace    = ["compliance"]
  policy_name  = "data_retention"
  privilege    = "POLICY_READ"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_policy_grant.test", "catalog", catalog),
					resource.TestCheckResourceAttr("polaris_policy_grant.test", "catalog_role", "tf-acc-test-crole-pgrant"),
					resource.TestCheckResourceAttr("polaris_policy_grant.test", "namespace.0", "compliance"),
					resource.TestCheckResourceAttr("polaris_policy_grant.test", "policy_name", "data_retention"),
					resource.TestCheckResourceAttr("polaris_policy_grant.test", "privilege", "POLICY_READ"),
				),
			},
			// Import
			{
				ResourceName:                        "polaris_policy_grant.test",
				ImportState:                         true,
				ImportStateId:                       catalog + "/tf-acc-test-crole-pgrant/POLICY_READ/compliance/data_retention",
				ImportStateVerify:                   true,
				ImportStateVerifyIdentifierAttribute: "privilege",
			},
		},
	})
}
