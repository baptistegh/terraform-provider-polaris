package provider

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// testCheckNamespaces checks that the namespaces attribute of a polaris_namespaces data
// source holds exactly want, each written with its levels joined by ".", in any order.
func testCheckNamespaces(ds string, want ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[ds]
		if !ok {
			return fmt.Errorf("%s not found in state", ds)
		}
		attrs := rs.Primary.Attributes
		n, _ := strconv.Atoi(attrs["namespaces.#"])
		got := make([]string, 0, n)
		for i := 0; i < n; i++ {
			levels, _ := strconv.Atoi(attrs[fmt.Sprintf("namespaces.%d.#", i)])
			parts := make([]string, levels)
			for j := range parts {
				parts[j] = attrs[fmt.Sprintf("namespaces.%d.%d", i, j)]
			}
			got = append(got, strings.Join(parts, "."))
		}
		sort.Strings(got)
		sort.Strings(want)
		if len(got) != len(want) || (len(got) > 0 && !reflect.DeepEqual(got, want)) {
			return fmt.Errorf("%s: namespaces = %q, want %q", ds, got, want)
		}
		return nil
	}
}

func TestAccDatasourceNamespaces_basic(t *testing.T) {
	checkTFAcc(t)
	testAccPreCheck(t)

	catalog := testCreateCatalog(t)
	testCreateNamespace(t, catalog, []string{"sales"})
	testCreateNamespace(t, catalog, []string{"sales", "eu"})
	testCreateNamespace(t, catalog, []string{"sales", "us"})
	testCreateNamespace(t, catalog, []string{"marketing"})

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccProviderConfig() + `
data "polaris_namespaces" "top" {
  catalog = "` + catalog + `"
}

data "polaris_namespaces" "sales" {
  catalog = "` + catalog + `"
  parent  = ["sales"]
}

data "polaris_namespaces" "leaf" {
  catalog = "` + catalog + `"
  parent  = ["sales", "eu"]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					testCheckNamespaces("data.polaris_namespaces.top", "marketing", "sales"),
					// Children are returned with their full path.
					testCheckNamespaces("data.polaris_namespaces.sales", "sales.eu", "sales.us"),
					resource.TestCheckResourceAttr("data.polaris_namespaces.leaf", "namespaces.#", "0"),
				),
			},
		},
	})
}
