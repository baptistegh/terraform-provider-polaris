package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
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

// testCreateCatalog creates a temporary INTERNAL/FILE catalog with a random name,
// registers t.Cleanup to delete it, and returns the catalog name.
// It builds a management client directly from env vars so it works outside Terraform apply.
func testCreateCatalog(t *testing.T) string {
	t.Helper()

	baseURL := os.Getenv("POLARIS_BASE_URL")
	clientID := os.Getenv("POLARIS_CLIENT_ID")
	clientSecret := os.Getenv("POLARIS_CLIENT_SECRET")
	realm := os.Getenv("POLARIS_REALM")

	ctx := context.Background()
	httpClient := &http.Client{}

	token, err := fetchToken(ctx, httpClient, baseURL, clientID, clientSecret, realm, defaultRealmHeader)
	if err != nil {
		t.Fatalf("testCreateCatalog: fetch token: %v", err)
	}

	client := newManagementClient(baseURL, httpClient, token, realm, defaultRealmHeader)

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

	_, httpResp, err := client.DefaultAPI.CreateCatalog(ctx).
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

// testCreateNamespace creates a namespace in the given catalog via the Iceberg REST API.
// Polaris requires the namespace to exist before grants can be applied to it.
// Registers t.Cleanup to delete the namespace.
func testCreateNamespace(t *testing.T, catalogName string, namespace []string) {
	t.Helper()

	baseURL := os.Getenv("POLARIS_BASE_URL")
	clientID := os.Getenv("POLARIS_CLIENT_ID")
	clientSecret := os.Getenv("POLARIS_CLIENT_SECRET")
	realm := os.Getenv("POLARIS_REALM")

	ctx := context.Background()
	httpClient := &http.Client{}

	token, err := fetchToken(ctx, httpClient, baseURL, clientID, clientSecret, realm, defaultRealmHeader)
	if err != nil {
		t.Fatalf("testCreateNamespace: fetch token: %v", err)
	}

	authHeader := "Bearer " + token

	createURL := fmt.Sprintf("%s/api/catalog/v1/%s/namespaces", strings.TrimRight(baseURL, "/"), catalogName)
	payload, _ := json.Marshal(map[string]any{"namespace": namespace, "properties": map[string]string{}})

	createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", authHeader)
	if realm != "" {
		createReq.Header.Set(defaultRealmHeader, realm)
	}

	resp, err := httpClient.Do(createReq)
	if err != nil {
		t.Fatalf("testCreateNamespace: API call: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		t.Fatalf("testCreateNamespace: expected 200/201, got %d: %v", resp.StatusCode, body)
	}

	t.Cleanup(func() {
		nsPath := strings.Join(namespace, "\x1f")
		deleteURL := fmt.Sprintf("%s/api/catalog/v1/%s/namespaces/%s", strings.TrimRight(baseURL, "/"), catalogName, nsPath)
		deleteReq, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, deleteURL, nil)
		deleteReq.Header.Set("Authorization", authHeader)
		if realm != "" {
			deleteReq.Header.Set(defaultRealmHeader, realm)
		}
		delResp, err := httpClient.Do(deleteReq)
		if err != nil {
			t.Logf("testCreateNamespace cleanup: delete %v: %v", namespace, err)
			return
		}
		_ = delResp.Body.Close()
	})
}

// testCreateTable creates a minimal Iceberg table in the given catalog+namespace.
// Polaris requires the table to exist before a table-level grant can be applied.
// Registers t.Cleanup to drop the table.
func testCreateTable(t *testing.T, catalogName string, namespace []string, tableName string) {
	t.Helper()

	baseURL := os.Getenv("POLARIS_BASE_URL")
	clientID := os.Getenv("POLARIS_CLIENT_ID")
	clientSecret := os.Getenv("POLARIS_CLIENT_SECRET")
	realm := os.Getenv("POLARIS_REALM")

	ctx := context.Background()
	httpClient := &http.Client{}

	token, err := fetchToken(ctx, httpClient, baseURL, clientID, clientSecret, realm, defaultRealmHeader)
	if err != nil {
		t.Fatalf("testCreateTable: fetch token: %v", err)
	}

	authHeader := "Bearer " + token
	nsSegment := strings.Join(namespace, "\x1f")
	baseURL = strings.TrimRight(baseURL, "/")

	createURL := fmt.Sprintf("%s/api/catalog/v1/%s/namespaces/%s/tables", baseURL, catalogName, nsSegment)
	payload, _ := json.Marshal(map[string]any{
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
	})

	createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", authHeader)
	if realm != "" {
		createReq.Header.Set(defaultRealmHeader, realm)
	}

	resp, err := httpClient.Do(createReq)
	if err != nil {
		t.Fatalf("testCreateTable: API call: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		t.Fatalf("testCreateTable: expected 200/201, got %d: %v", resp.StatusCode, body)
	}

	t.Cleanup(func() {
		deleteURL := fmt.Sprintf("%s/api/catalog/v1/%s/namespaces/%s/tables/%s", baseURL, catalogName, nsSegment, tableName)
		deleteReq, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, deleteURL, nil)
		deleteReq.Header.Set("Authorization", authHeader)
		if realm != "" {
			deleteReq.Header.Set(defaultRealmHeader, realm)
		}
		delResp, err := httpClient.Do(deleteReq)
		if err != nil {
			t.Logf("testCreateTable cleanup: delete %s: %v", tableName, err)
			return
		}
		_ = delResp.Body.Close()
	})
}

// testCreateView creates a minimal Iceberg view in the given catalog+namespace.
// Polaris requires the view to exist before a view-level grant can be applied.
// Registers t.Cleanup to drop the view.
func testCreateView(t *testing.T, catalogName string, namespace []string, viewName string) {
	t.Helper()

	baseURL := os.Getenv("POLARIS_BASE_URL")
	clientID := os.Getenv("POLARIS_CLIENT_ID")
	clientSecret := os.Getenv("POLARIS_CLIENT_SECRET")
	realm := os.Getenv("POLARIS_REALM")

	ctx := context.Background()
	httpClient := &http.Client{}

	token, err := fetchToken(ctx, httpClient, baseURL, clientID, clientSecret, realm, defaultRealmHeader)
	if err != nil {
		t.Fatalf("testCreateView: fetch token: %v", err)
	}

	authHeader := "Bearer " + token
	nsSegment := strings.Join(namespace, "\x1f")
	baseURL = strings.TrimRight(baseURL, "/")

	createURL := fmt.Sprintf("%s/api/catalog/v1/%s/namespaces/%s/views", baseURL, catalogName, nsSegment)
	payload, _ := json.Marshal(map[string]any{
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
	})

	createReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(payload))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", authHeader)
	if realm != "" {
		createReq.Header.Set(defaultRealmHeader, realm)
	}

	resp, err := httpClient.Do(createReq)
	if err != nil {
		t.Fatalf("testCreateView: API call: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		t.Fatalf("testCreateView: expected 200/201, got %d: %v", resp.StatusCode, body)
	}

	t.Cleanup(func() {
		deleteURL := fmt.Sprintf("%s/api/catalog/v1/%s/namespaces/%s/views/%s", baseURL, catalogName, nsSegment, viewName)
		deleteReq, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, deleteURL, nil)
		deleteReq.Header.Set("Authorization", authHeader)
		if realm != "" {
			deleteReq.Header.Set(defaultRealmHeader, realm)
		}
		delResp, err := httpClient.Do(deleteReq)
		if err != nil {
			t.Logf("testCreateView cleanup: delete %s: %v", viewName, err)
			return
		}
		_ = delResp.Body.Close()
	})
}

// testCreateSemanticModel creates an empty semantic model in the given catalog+namespace
// through the Polaris semantic model API (requires ENABLE_SEMANTIC_MODELS on the server).
// Polaris requires the semantic model to exist before a semantic-model grant can be applied.
// Registers t.Cleanup to drop it.
func testCreateSemanticModel(t *testing.T, catalogName string, namespace []string, modelName string) {
	t.Helper()

	baseURL := strings.TrimRight(os.Getenv("POLARIS_BASE_URL"), "/")
	clientID := os.Getenv("POLARIS_CLIENT_ID")
	clientSecret := os.Getenv("POLARIS_CLIENT_SECRET")
	realm := os.Getenv("POLARIS_REALM")

	ctx := context.Background()
	httpClient := &http.Client{}

	token, err := fetchToken(ctx, httpClient, baseURL, clientID, clientSecret, realm, defaultRealmHeader)
	if err != nil {
		t.Fatalf("testCreateSemanticModel: fetch token: %v", err)
	}

	do := func(ctx context.Context, method, url string, body []byte) (int, string, error) {
		req, _ := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		if realm != "" {
			req.Header.Set(defaultRealmHeader, realm)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			return 0, "", err
		}
		defer func() { _ = resp.Body.Close() }()
		var b bytes.Buffer
		_, _ = b.ReadFrom(resp.Body)
		return resp.StatusCode, b.String(), nil
	}

	collectionURL := fmt.Sprintf("%s/api/catalog/polaris/v1/%s/namespaces/%s/semantic-models", baseURL, catalogName, strings.Join(namespace, "\x1f"))
	// An Apache Ossie document with no datasets is the smallest valid semantic model.
	payload, _ := json.Marshal(map[string]any{
		"name":     modelName,
		"document": map[string]string{"version": "0.2.0.dev0", "semantic_model": `{"datasets":[]}`},
	})

	status, body, err := do(ctx, http.MethodPost, collectionURL, payload)
	if err != nil {
		t.Fatalf("testCreateSemanticModel: API call: %v", err)
	}
	if status != http.StatusOK && status != http.StatusCreated {
		t.Fatalf("testCreateSemanticModel: expected 200/201, got %d: %s (is ENABLE_SEMANTIC_MODELS on?)", status, body)
	}

	t.Cleanup(func() {
		if _, _, err := do(context.Background(), http.MethodDelete, collectionURL+"/"+modelName, nil); err != nil {
			t.Logf("testCreateSemanticModel cleanup: delete %s: %v", modelName, err)
		}
	})
}

// Ensure the resource package is test-compatible.
var _ = resource.Test
