package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccResourceNamespace_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	const res = "polaris_namespace.test"
	defaultLocation := "file:///tmp/" + catalog + "/sales/"
	config := func(props string) string {
		return testAccProviderConfig() + `
resource "polaris_namespace" "test" {
  catalog   = "` + catalog + `"
  namespace = ["sales"]
  ` + props + `
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Create: Polaris sets the location, which is kept out of properties.
			{
				Config: config(`properties = { "owner" = "data", "tier" = "gold" }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "catalog", catalog),
					resource.TestCheckResourceAttr(res, "namespace.#", "1"),
					resource.TestCheckResourceAttr(res, "namespace.0", "sales"),
					resource.TestCheckResourceAttr(res, "properties.%", "2"),
					resource.TestCheckResourceAttr(res, "properties.owner", "data"),
					resource.TestCheckNoResourceAttr(res, "properties.location"),
					resource.TestCheckResourceAttr(res, "location", defaultLocation),
				),
			},
			// Change one property, add one, remove one: an in-place update that keeps the location.
			{
				Config: config(`properties = { "owner" = "platform", "env" = "prod" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "properties.%", "2"),
					resource.TestCheckResourceAttr(res, "properties.owner", "platform"),
					resource.TestCheckResourceAttr(res, "properties.env", "prod"),
					resource.TestCheckNoResourceAttr(res, "properties.tier"),
					resource.TestCheckResourceAttr(res, "location", defaultLocation),
				),
			},
			// Import
			{
				ResourceName:                         res,
				ImportState:                          true,
				ImportStateId:                        catalog + "/sales",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "catalog",
			},
			// Removing every property must not remove the server-managed location.
			{
				Config: config(``),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(res, "properties.%"),
					resource.TestCheckResourceAttr(res, "location", defaultLocation),
				),
			},
			// And the read-back is stable: no diff on the next plan.
			{
				Config: config(``),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccResourceNamespace_nested(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	const child = "polaris_namespace.eu"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_namespace" "sales" {
  catalog   = "` + catalog + `"
  namespace = ["sales"]
}

resource "polaris_namespace" "eu" {
  catalog    = polaris_namespace.sales.catalog
  namespace  = concat(polaris_namespace.sales.namespace, ["eu"])
  properties = { "region" = "eu-west-1" }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(child, "namespace.#", "2"),
					resource.TestCheckResourceAttr(child, "namespace.1", "eu"),
					resource.TestCheckResourceAttr(child, "properties.region", "eu-west-1"),
					resource.TestCheckResourceAttr(child, "location", "file:///tmp/"+catalog+"/sales/eu/"),
				),
			},
			{
				ResourceName:                         child,
				ImportState:                          true,
				ImportStateId:                        catalog + "/sales/eu",
				ImportStateVerify:                    true,
				// Both namespaces share the catalog: location tells them apart.
				ImportStateVerifyIdentifierAttribute: "location",
			},
		},
	})
}

// Custom locations need ALLOW_NAMESPACE_CUSTOM_LOCATION (set in docker-compose.yml).
func TestAccResourceNamespace_customLocation(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const res = "polaris_namespace.test"
	config := func(location string) string {
		return testAccProviderConfig() + `
resource "polaris_catalog" "test" {
  name                  = "` + name + `"
  default_base_location = "file:///tmp/` + name + `"

  storage_config = {
    storage_type      = "FILE"
    allowed_locations = ["file:///tmp/` + name + `", "file:///tmp/` + name + `-custom"]
  }
}

resource "polaris_namespace" "test" {
  catalog   = polaris_catalog.test.name
  namespace = ["raw"]
  location  = "` + location + `"
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Written without the trailing slash Polaris adds: kept as configured, no diff.
			{
				Config: config("file:///tmp/" + name + "-custom/raw"),
				Check:  resource.TestCheckResourceAttr(res, "location", "file:///tmp/"+name+"-custom/raw"),
			},
			{
				Config: config("file:///tmp/" + name + "-custom/raw"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Moving the location is an in-place update.
			{
				Config: config("file:///tmp/" + name + "-custom/raw-v2/"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttr(res, "location", "file:///tmp/"+name+"-custom/raw-v2/"),
			},
		},
	})
}

func TestAccResourceNamespace_invalidImportID(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_namespace" "test" {
  catalog   = "c"
  namespace = ["ns"]
}
`,
				ResourceName:  "polaris_namespace.test",
				ImportState:   true,
				ImportStateId: "only-a-catalog",
				ExpectError:   regexp.MustCompile(`Invalid import ID`),
			},
		},
	})
}
