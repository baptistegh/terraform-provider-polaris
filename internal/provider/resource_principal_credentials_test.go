package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testCheckCredentials checks that client_id/client_secret of the resource can (or can no
// longer, when valid is false) obtain a token from Polaris.
func testCheckCredentials(res, idAttr, secretAttr string, valid bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[res]
		if !ok {
			return fmt.Errorf("%s not found in state", res)
		}
		id, secret := rs.Primary.Attributes[idAttr], rs.Primary.Attributes[secretAttr]
		_, err := fetchToken(context.Background(), &http.Client{}, os.Getenv("POLARIS_BASE_URL"), id, secret, os.Getenv("POLARIS_REALM"), defaultRealmHeader)
		switch {
		case valid && err != nil:
			return fmt.Errorf("%s: credentials rejected: %v", res, err)
		case !valid && err == nil:
			return fmt.Errorf("%s: old credentials still accepted", res)
		}
		return nil
	}
}

// captureAttr stores an attribute value so a later step can compare against it.
func captureAttr(res, attr string, into *string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttrWith(res, attr, func(v string) error {
		*into = v
		return nil
	})
}

func TestAccResourcePrincipalCredentials_reset(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const res = "polaris_principal_credentials.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal" "test" {
  name = "` + name + `"
}

resource "polaris_principal_credentials" "test" {
  principal         = polaris_principal.test.name
  rotation_triggers = { principal_id = polaris_principal.test.client_id }
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "principal", name),
					resource.TestCheckResourceAttr(res, "method", "RESET"),
					resource.TestCheckResourceAttrPair(res, "client_id", "polaris_principal.test", "client_id"),
					resource.TestCheckResourceAttrSet(res, "client_secret"),
					testCheckCredentials(res, "client_id", "client_secret", true),
					// The secret returned when the principal was created no longer works.
					testCheckCredentials("polaris_principal.test", "client_id", "client_secret", false),
				),
			},
			// Re-applying the same configuration issues nothing new.
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal" "test" {
  name = "` + name + `"
}

resource "polaris_principal_credentials" "test" {
  principal         = polaris_principal.test.name
  rotation_triggers = { principal_id = polaris_principal.test.client_id }
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccResourcePrincipalCredentials_rotationTriggers(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name, _, _ := testCreatePrincipal(t)
	const res = "polaris_principal_credentials.test"
	config := func(trigger string) string {
		return testAccProviderConfig() + `
resource "polaris_principal_credentials" "test" {
  principal         = "` + name + `"
  rotation_triggers = { rotated_on = "` + trigger + `" }
}
`
	}
	var first string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: config("2026-01"),
				Check:  captureAttr(res, "client_secret", &first),
			},
			{
				Config: config("2026-02"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrWith(res, "client_secret", func(v string) error {
						if v == first {
							return fmt.Errorf("client_secret did not change")
						}
						return nil
					}),
					testCheckCredentials(res, "client_id", "client_secret", true),
				),
			},
		},
	})
}

func TestAccResourcePrincipalCredentials_customSecret(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name, _, _ := testCreatePrincipal(t)
	const res = "polaris_principal_credentials.test"
	config := func(secret string) string {
		return testAccProviderConfig() + `
resource "polaris_principal_credentials" "test" {
  principal     = "` + name + `"
  client_secret = "` + secret + `"
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: config("tf-acc-custom-secret-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "client_secret", "tf-acc-custom-secret-1"),
					testCheckCredentials(res, "client_id", "client_secret", true),
				),
			},
			{
				Config: config("tf-acc-custom-secret-2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(res, plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "client_secret", "tf-acc-custom-secret-2"),
					testCheckCredentials(res, "client_id", "client_secret", true),
				),
			},
		},
	})
}

// Polaris only lets a principal rotate its own credentials, so the provider
// authenticates as the principal for ROTATE.
func TestAccResourcePrincipalCredentials_rotateAsPrincipal(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name, clientID, clientSecret := testCreatePrincipal(t)
	const res = "polaris_principal_credentials.self"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: `
provider "polaris" {
  base_url      = "` + os.Getenv("POLARIS_BASE_URL") + `"
  client_id     = "` + clientID + `"
  client_secret = "` + clientSecret + `"
}

resource "polaris_principal_credentials" "self" {
  principal = "` + name + `"
  method    = "ROTATE"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "method", "ROTATE"),
					resource.TestCheckResourceAttr(res, "client_id", clientID),
					testCheckCredentials(res, "client_id", "client_secret", true),
					// Unlike RESET, ROTATE keeps the previous secret valid until the next rotation.
					func(*terraform.State) error {
						_, err := fetchToken(context.Background(), &http.Client{}, os.Getenv("POLARIS_BASE_URL"), clientID, clientSecret, os.Getenv("POLARIS_REALM"), defaultRealmHeader)
						if err != nil {
							return fmt.Errorf("the secret from before the rotation was revoked: %v", err)
						}
						return nil
					},
				),
			},
		},
	})
}

// When the principal is re-created under the same name, the stored credentials belong
// to the old principal: they are dropped from state and issued again.
func TestAccResourcePrincipalCredentials_principalRecreated(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	name := acctest.RandomWithPrefix("tf-acc")
	const res = "polaris_principal_credentials.test"
	config := func(rotationRequired string) string {
		return testAccProviderConfig() + `
resource "polaris_principal" "test" {
  name                         = "` + name + `"
  credential_rotation_required = ` + rotationRequired + `
}

resource "polaris_principal_credentials" "test" {
  principal = polaris_principal.test.name
}
`
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: config("false"),
			},
			// Replacing the principal leaves the credentials stale; the next refresh drops them.
			{
				Config:             config("true"),
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config("true"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(res, "client_id", "polaris_principal.test", "client_id"),
					testCheckCredentials(res, "client_id", "client_secret", true),
				),
			},
		},
	})
}

func TestAccResourcePrincipalCredentials_invalidConfig(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal_credentials" "test" {
  principal     = "anyone"
  method        = "ROTATE"
  client_secret = "chosen"
}
`,
				ExpectError: regexp.MustCompile(`client_secret requires RESET`),
			},
			{
				Config: testAccProviderConfig() + `
resource "polaris_principal_credentials" "test" {
  principal = "anyone"
  method    = "rotate"
}
`,
				ExpectError: regexp.MustCompile(`Invalid method`),
			},
		},
	})
}
