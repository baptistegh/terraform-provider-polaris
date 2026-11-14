package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDatasourcePolicy_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"governance"})
	const ds = "data.polaris_policy.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_policy" "test" {
  catalog     = "` + catalog + `"
  namespace   = ["governance"]
  name        = "compaction"
  type        = "system.data-compaction"
  description = "Nightly compaction"
  content     = jsonencode({ version = "2025-02-03", enable = true })
}

data "polaris_policy" "test" {
  catalog   = polaris_policy.test.catalog
  namespace = polaris_policy.test.namespace
  name      = polaris_policy.test.name
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ds, "type", "system.data-compaction"),
					resource.TestCheckResourceAttr(ds, "description", "Nightly compaction"),
					resource.TestCheckResourceAttrPair(ds, "content", "polaris_policy.test", "content"),
					resource.TestCheckResourceAttr(ds, "inheritable", "true"),
					resource.TestCheckResourceAttr(ds, "version", "0"),
				),
			},
			{
				Config: testAccProviderConfig() + `
data "polaris_policy" "missing" {
  catalog   = "` + catalog + `"
  namespace = ["governance"]
  name      = "does-not-exist"
}
`,
				ExpectError: regexp.MustCompile(`Policy not found`),
			},
		},
	})
}
