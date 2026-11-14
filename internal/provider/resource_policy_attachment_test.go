package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testCheckAttached checks against the API whether the policy is directly attached to target.
func testCheckAttached(t *testing.T, catalog string, namespace []string, policy string, target policyTarget, want bool) resource.TestCheckFunc {
	return func(*terraform.State) error {
		policies, err := applicablePolicies(context.Background(), testClient(t), catalog, target)
		if err != nil {
			return err
		}
		if got := isAttached(policies, namespace, policy); got != want {
			return fmt.Errorf("policy %s attached to %+v = %v, want %v", policy, target, got, want)
		}
		return nil
	}
}

const testPolicyAttachmentPolicy = `
resource "polaris_policy" "compaction" {
  catalog   = "%[1]s"
  namespace = ["governance"]
  name      = "compaction"
  type      = "system.data-compaction"
  content   = jsonencode({ version = "2025-02-03", enable = true })
}
`

func TestAccResourcePolicyAttachment_catalogAndNamespace(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"governance"})
	testCreateNamespace(t, catalog, []string{"sales"})
	policyNS := []string{"governance"}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + fmt.Sprintf(testPolicyAttachmentPolicy, catalog) + `
resource "polaris_policy_attachment" "catalog" {
  catalog          = polaris_policy.compaction.catalog
  policy_namespace = polaris_policy.compaction.namespace
  policy_name      = polaris_policy.compaction.name
  target_type      = "catalog"
}

resource "polaris_policy_attachment" "sales" {
  catalog          = polaris_policy.compaction.catalog
  policy_namespace = polaris_policy.compaction.namespace
  policy_name      = polaris_policy.compaction.name
  target_type      = "namespace"
  target_path      = ["sales"]
  parameters       = { "priority" = "high" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("polaris_policy_attachment.catalog", "target_type", "catalog"),
					resource.TestCheckNoResourceAttr("polaris_policy_attachment.catalog", "target_path"),
					resource.TestCheckResourceAttr("polaris_policy_attachment.sales", "target_path.0", "sales"),
					resource.TestCheckResourceAttr("polaris_policy_attachment.sales", "parameters.priority", "high"),
					testCheckAttached(t, catalog, policyNS, "compaction", policyTarget{Type: policyTargetCatalog}, true),
					testCheckAttached(t, catalog, policyNS, "compaction", policyTarget{Type: policyTargetNamespace, Path: []string{"sales"}}, true),
				),
			},
			{
				ResourceName:                         "polaris_policy_attachment.catalog",
				ImportState:                          true,
				ImportStateId:                        catalog + "/governance/compaction/catalog",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "target_type", // both attachments share the policy
			},
			{
				ResourceName:                         "polaris_policy_attachment.sales",
				ImportState:                          true,
				ImportStateId:                        catalog + "/governance/compaction/namespace/sales",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "target_type",          // both attachments share the policy
				ImportStateVerifyIgnore:              []string{"parameters"}, // never returned by the API
			},
			// Removing the namespace attachment detaches it; the catalog one stays.
			{
				Config: testAccProviderConfig() + fmt.Sprintf(testPolicyAttachmentPolicy, catalog) + `
resource "polaris_policy_attachment" "catalog" {
  catalog          = polaris_policy.compaction.catalog
  policy_namespace = polaris_policy.compaction.namespace
  policy_name      = polaris_policy.compaction.name
  target_type      = "catalog"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckAttached(t, catalog, policyNS, "compaction", policyTarget{Type: policyTargetNamespace, Path: []string{"sales"}}, false),
					testCheckAttached(t, catalog, policyNS, "compaction", policyTarget{Type: policyTargetCatalog}, true),
				),
			},
		},
	})
}

func TestAccResourcePolicyAttachment_table(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"governance"})
	testCreateNamespace(t, catalog, []string{"sales"})
	testCreateTable(t, catalog, []string{"sales"}, "orders")
	table := policyTarget{Type: policyTargetTable, Path: []string{"sales", "orders"}}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + fmt.Sprintf(testPolicyAttachmentPolicy, catalog) + `
resource "polaris_policy_attachment" "orders" {
  catalog          = polaris_policy.compaction.catalog
  policy_namespace = polaris_policy.compaction.namespace
  policy_name      = polaris_policy.compaction.name
  target_type      = "table-like"
  target_path      = ["sales", "orders"]
}
`,
				Check: testCheckAttached(t, catalog, []string{"governance"}, "compaction", table, true),
			},
			{
				ResourceName:                         "polaris_policy_attachment.orders",
				ImportState:                          true,
				ImportStateId:                        catalog + "/governance/compaction/table-like/sales.orders",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "policy_name",
			},
			// Detached outside Terraform: the next plan re-creates the attachment.
			{
				PreConfig: func() {
					if err := detachPolicy(context.Background(), testClient(t), catalog, []string{"governance"}, "compaction", table); err != nil {
						t.Fatalf("detach: %v", err)
					}
				},
				Config: testAccProviderConfig() + fmt.Sprintf(testPolicyAttachmentPolicy, catalog) + `
resource "polaris_policy_attachment" "orders" {
  catalog          = polaris_policy.compaction.catalog
  policy_namespace = polaris_policy.compaction.namespace
  policy_name      = polaris_policy.compaction.name
  target_type      = "table-like"
  target_path      = ["sales", "orders"]
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("polaris_policy_attachment.orders", plancheck.ResourceActionCreate)},
				},
				Check: testCheckAttached(t, catalog, []string{"governance"}, "compaction", table, true),
			},
		},
	})
}

func TestAccResourcePolicyAttachment_invalidTarget(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	config := func(target string) string {
		return testAccProviderConfig() + `
resource "polaris_policy_attachment" "test" {
  catalog          = "c"
  policy_namespace = ["ns"]
  policy_name      = "p"
  ` + target + `
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{Config: config(`target_type = "catalog"` + "\n  target_path = [\"ns\"]"), ExpectError: regexp.MustCompile(`must be omitted`)},
			{Config: config(`target_type = "namespace"`), ExpectError: regexp.MustCompile(`namespace levels`)},
			{Config: config(`target_type = "table-like"` + "\n  target_path = [\"orders\"]"), ExpectError: regexp.MustCompile(`table or view name`)},
			{Config: config(`target_type = "table"`), ExpectError: regexp.MustCompile(`target_type must be`)},
		},
	})
}
