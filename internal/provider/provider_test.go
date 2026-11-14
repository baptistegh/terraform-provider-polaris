package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// providerFactories is used in every acceptance test.
var providerFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"polaris": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates the required environment variables for acceptance tests.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, env := range []string{"POLARIS_BASE_URL", "POLARIS_CLIENT_ID", "POLARIS_CLIENT_SECRET"} {
		if os.Getenv(env) == "" {
			t.Skipf("acceptance tests require %s to be set", env)
		}
	}
}

// testAccProviderConfig returns the minimal provider block for acceptance tests.
func testAccProviderConfig() string {
	return `
provider "polaris" {
  base_url      = "` + os.Getenv("POLARIS_BASE_URL") + `"
  client_id     = "` + os.Getenv("POLARIS_CLIENT_ID") + `"
  client_secret = "` + os.Getenv("POLARIS_CLIENT_SECRET") + `"
}
`
}

// checkTFAcc skips the test when TF_ACC is not set, matching the standard convention.
func checkTFAcc(t *testing.T) {
	t.Helper()
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
}

// testClient returns a management API client authenticated with the acceptance test
// credentials from the environment. Test helpers use it to create fixtures outside
// Terraform; doCatalogJSON reaches the catalog API through the same client.
func testClient(t *testing.T) *polarismanagement.APIClient {
	t.Helper()

	baseURL, realm := os.Getenv("POLARIS_BASE_URL"), os.Getenv("POLARIS_REALM")
	httpClient := &http.Client{}
	token, err := fetchToken(context.Background(), httpClient, baseURL, os.Getenv("POLARIS_CLIENT_ID"), os.Getenv("POLARIS_CLIENT_SECRET"), realm, defaultRealmHeader)
	if err != nil {
		t.Fatalf("testClient: fetch token: %v", err)
	}
	return newManagementClient(baseURL, httpClient, token, realm, defaultRealmHeader)
}

// testCreateCatalog creates a temporary INTERNAL/FILE catalog with a random name,
// registers t.Cleanup to delete it, and returns the catalog name.
func testCreateCatalog(t *testing.T) string {
	t.Helper()

	client := testClient(t)
	name := acctest.RandomWithPrefix("tf-acc")
	catalog := polarismanagement.Catalog{
		Name: name,
		Type: "INTERNAL",
		Properties: polarismanagement.CatalogProperties{
			DefaultBaseLocation: fmt.Sprintf("file:///tmp/%s", name),
		},
		StorageConfigInfo: polarismanagement.StorageConfigInfo{
			StorageType: "FILE",
		},
	}

	_, httpResp, err := client.DefaultAPI.CreateCatalog(context.Background()).
		CreateCatalogRequest(polarismanagement.CreateCatalogRequest{Catalog: catalog}).
		Execute()
	if err := checkResponse(httpResp, err, http.StatusCreated); err != nil {
		t.Fatalf("testCreateCatalog: %v", err)
	}

	t.Cleanup(func() {
		httpResp, err := client.DefaultAPI.DeleteCatalog(context.Background(), name).Execute()
		if err := checkResponse(httpResp, err, http.StatusNoContent); err != nil {
			t.Logf("testCreateCatalog cleanup: delete %q: %v", name, err)
		}
	})

	return name
}

// testCreateCatalogObject creates an object through the catalog API (POST path with body,
// expecting 200) and registers t.Cleanup to DELETE cleanupPath, expecting 204.
func testCreateCatalogObject(t *testing.T, what, path string, body any, cleanupPath string) {
	t.Helper()

	client := testClient(t)
	if err := doCatalogJSON(context.Background(), client, http.MethodPost, path, body, http.StatusOK, nil); err != nil {
		t.Fatalf("create %s: %v", what, err)
	}
	t.Cleanup(func() {
		if err := doCatalogJSON(context.Background(), client, http.MethodDelete, cleanupPath, nil, http.StatusNoContent, nil); err != nil {
			t.Logf("cleanup: delete %s: %v", what, err)
		}
	})
}

// testCreateNamespace creates a namespace in the given catalog via the Iceberg REST API.
// Polaris requires the namespace to exist before grants can be applied to it.
// Registers t.Cleanup to delete the namespace.
func testCreateNamespace(t *testing.T, catalogName string, namespace []string) {
	t.Helper()
	testCreateCatalogObject(t, fmt.Sprintf("namespace %v", namespace),
		icebergCatalogPath(catalogName, "namespaces"),
		createNamespacePayload{Namespace: namespace},
		icebergCatalogPath(catalogName, "namespaces", namespacePath(namespace)))
}

// testCreateTable creates a minimal Iceberg table in the given catalog+namespace.
// Polaris requires the table to exist before a table-level grant can be applied.
// Registers t.Cleanup to drop the table.
func testCreateTable(t *testing.T, catalogName string, namespace []string, tableName string) {
	t.Helper()
	ns := namespacePath(namespace)
	testCreateCatalogObject(t, "table "+tableName,
		icebergCatalogPath(catalogName, "namespaces", ns, "tables"),
		map[string]any{
			"name": tableName,
			"schema": map[string]any{
				"type":      "struct",
				"schema-id": 0,
				"fields":    []map[string]any{{"id": 1, "name": "id", "required": false, "type": "long"}},
			},
			"partition-spec": map[string]any{"spec-id": 0, "fields": []any{}},
			"write-order":    map[string]any{"order-id": 0, "fields": []any{}},
			"stage-create":   false,
			"properties":     map[string]string{},
		},
		icebergCatalogPath(catalogName, "namespaces", ns, "tables", url.PathEscape(tableName)))
}

// testCreateView creates a minimal Iceberg view in the given catalog+namespace.
// Polaris requires the view to exist before a view-level grant can be applied.
// Registers t.Cleanup to drop the view.
func testCreateView(t *testing.T, catalogName string, namespace []string, viewName string) {
	t.Helper()
	ns := namespacePath(namespace)
	testCreateCatalogObject(t, "view "+viewName,
		icebergCatalogPath(catalogName, "namespaces", ns, "views"),
		map[string]any{
			"name":              viewName,
			"default-namespace": namespace,
			"schema":            map[string]any{"type": "struct", "schema-id": 0, "fields": []any{}},
			"view-version": map[string]any{
				"version-id":        1,
				"schema-id":         -1,
				"timestamp-ms":      0,
				"default-namespace": namespace,
				"summary":           map[string]string{"operation": "replace"},
				"representations": []map[string]string{
					{"type": "sql", "sql": "SELECT 1 AS dummy", "dialect": "spark"},
				},
			},
			"properties": map[string]string{},
		},
		icebergCatalogPath(catalogName, "namespaces", ns, "views", url.PathEscape(viewName)))
}

// testCreateSemanticModel creates an empty semantic model in the given catalog+namespace
// through the Polaris semantic model API (requires ENABLE_SEMANTIC_MODELS on the server).
// Polaris requires the semantic model to exist before a semantic-model grant can be applied.
// Registers t.Cleanup to drop it.
func testCreateSemanticModel(t *testing.T, catalogName string, namespace []string, modelName string) {
	t.Helper()
	ns := namespacePath(namespace)
	// An Apache Ossie document with no datasets is the smallest valid semantic model.
	testCreateCatalogObject(t, "semantic model "+modelName,
		polarisCatalogPath(catalogName, "namespaces", ns, "semantic-models"),
		map[string]any{
			"name":     modelName,
			"document": map[string]string{"version": "0.2.0.dev0", "semantic_model": `{"datasets":[]}`},
		},
		polarisCatalogPath(catalogName, "namespaces", ns, "semantic-models", url.PathEscape(modelName)))
}

// testPolicyContent is a valid system.data-compaction policy definition.
const testPolicyContent = `{"version":"2025-02-03","enable":true}`

// testCreatePolicy creates a system.data-compaction policy in the given catalog+namespace
// through the Polaris policy API. Polaris requires the policy to exist before a policy
// grant can be applied to it. Registers t.Cleanup to drop it.
func testCreatePolicy(t *testing.T, catalogName string, namespace []string, policyName string) {
	t.Helper()
	testCreateCatalogObject(t, "policy "+policyName,
		polarisCatalogPath(catalogName, "namespaces", namespacePath(namespace), "policies"),
		createPolicyPayload{Name: policyName, Type: "system.data-compaction", Content: testPolicyContent},
		policyPath(catalogName, namespace, policyName))
}

// testCreatePrincipal creates a principal with a random name through the API, registers
// t.Cleanup to delete it, and returns its name, client ID and client secret.
func testCreatePrincipal(t *testing.T) (name, clientID, clientSecret string) {
	t.Helper()

	client := testClient(t)
	name = acctest.RandomWithPrefix("tf-acc")
	created, httpResp, err := client.DefaultAPI.CreatePrincipal(context.Background()).
		CreatePrincipalRequest(polarismanagement.CreatePrincipalRequest{Principal: &polarismanagement.Principal{Name: name}}).
		Execute()
	if err := checkResponse(httpResp, err, http.StatusCreated); err != nil {
		t.Fatalf("testCreatePrincipal: %v", err)
	}

	t.Cleanup(func() {
		httpResp, err := client.DefaultAPI.DeletePrincipal(context.Background(), name).Execute()
		if err := checkResponse(httpResp, err, http.StatusNoContent); err != nil {
			t.Logf("testCreatePrincipal cleanup: delete %q: %v", name, err)
		}
	})

	return name, created.Credentials.GetClientId(), created.Credentials.GetClientSecret()
}

// Ensure the resource package is test-compatible.
var _ = resource.Test
