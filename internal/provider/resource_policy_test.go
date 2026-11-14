package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccResourcePolicy_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"governance"})
	const res = "polaris_policy.test"
	config := func(enable, description string) string {
		return testAccProviderConfig() + `
resource "polaris_policy" "test" {
  catalog   = "` + catalog + `"
  namespace = ["governance"]
  name      = "compaction"
  type      = "system.data-compaction"
  content   = jsonencode({ version = "2025-02-03", enable = ` + enable + ` })
  ` + description + `
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: config("true", `description = "Nightly compaction"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "name", "compaction"),
					resource.TestCheckResourceAttr(res, "type", "system.data-compaction"),
					resource.TestCheckResourceAttr(res, "description", "Nightly compaction"),
					resource.TestCheckResourceAttr(res, "content", `{"enable":true,"version":"2025-02-03"}`),
					resource.TestCheckResourceAttr(res, "inheritable", "true"),
					resource.TestCheckResourceAttr(res, "version", "0"),
				),
			},
			// Content and description change in place; the version increments.
			{
				Config: config("false", `description = "Disabled for now"`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "content", `{"enable":false,"version":"2025-02-03"}`),
					resource.TestCheckResourceAttr(res, "description", "Disabled for now"),
					resource.TestCheckResourceAttr(res, "version", "1"),
				),
			},
			// Import
			{
				ResourceName:                         res,
				ImportState:                          true,
				ImportStateId:                        catalog + "/governance/compaction",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "name",
			},
			// Removing the description clears it.
			{
				Config: config("false", ``),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(res, "description"),
					resource.TestCheckResourceAttr(res, "version", "2"),
				),
			},
			{
				Config: config("false", ``),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccResourcePolicy_typeChangeReplaces(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"governance"})
	const res = "polaris_policy.test"
	config := func(policyType string) string {
		return testAccProviderConfig() + `
resource "polaris_policy" "test" {
  catalog   = "` + catalog + `"
  namespace = ["governance"]
  name      = "housekeeping"
  type      = "` + policyType + `"
  content   = jsonencode({ version = "2025-02-03", enable = true })
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{Config: config("system.snapshot-expiry")},
			{
				Config: config("system.orphan-file-removal"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr(res, "type", "system.orphan-file-removal"),
			},
		},
	})
}

// Polaris validates the content against the schema of the policy type.
func TestAccResourcePolicy_invalidContent(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"governance"})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_policy" "test" {
  catalog   = "` + catalog + `"
  namespace = ["governance"]
  name      = "broken"
  type      = "system.data-compaction"
  content   = jsonencode({ not_a_field = 1 })
}
`,
				ExpectError: regexp.MustCompile(`Invalid\s+policy`),
			},
		},
	})
}
