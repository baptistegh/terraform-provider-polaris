package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourcePolicies_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"governance"})
	testCreateNamespace(t, catalog, []string{"empty"})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_policy" "compaction" {
  catalog   = "` + catalog + `"
  namespace = ["governance"]
  name      = "compaction"
  type      = "system.data-compaction"
  content   = jsonencode({ version = "2025-02-03", enable = true })
}

resource "polaris_policy" "expiry" {
  catalog   = "` + catalog + `"
  namespace = ["governance"]
  name      = "expiry"
  type      = "system.snapshot-expiry"
  content   = jsonencode({ version = "2025-02-03", enable = true })
}

data "polaris_policies" "all" {
  catalog    = "` + catalog + `"
  namespace  = ["governance"]
  depends_on = [polaris_policy.compaction, polaris_policy.expiry]
}

data "polaris_policies" "compaction" {
  catalog     = "` + catalog + `"
  namespace   = ["governance"]
  policy_type = "system.data-compaction"
  depends_on  = [polaris_policy.compaction, polaris_policy.expiry]
}

data "polaris_policies" "empty" {
  catalog   = "` + catalog + `"
  namespace = ["empty"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.polaris_policies.all", "names.#", "2"),
					resource.TestCheckTypeSetElemAttr("data.polaris_policies.all", "names.*", "compaction"),
					resource.TestCheckTypeSetElemAttr("data.polaris_policies.all", "names.*", "expiry"),
					resource.TestCheckResourceAttr("data.polaris_policies.compaction", "names.#", "1"),
					resource.TestCheckResourceAttr("data.polaris_policies.compaction", "names.0", "compaction"),
					resource.TestCheckResourceAttr("data.polaris_policies.empty", "names.#", "0"),
				),
			},
		},
	})
}
