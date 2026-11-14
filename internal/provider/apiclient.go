package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
)

const (
	// managementAPIPath is where the management API (generated client, doJSON) is served.
	managementAPIPath = "/api/management/v1"
	// catalogAPIPath is where the catalog API (doCatalogJSON) is served: the Iceberg REST
	// catalog API under /v1/{prefix} and the Polaris-native catalog APIs under /polaris/v1/{prefix}.
	catalogAPIPath = "/api/catalog"
)

// newManagementClient builds a client for the Polaris management API (/api/management/v1)
// that authenticates every request with the given Bearer token and realm header.
func newManagementClient(baseURL string, httpClient *http.Client, token, realm, realmHeader string) *polarismanagement.APIClient {
	cfg := polarismanagement.NewConfiguration()
	cfg.Servers = polarismanagement.ServerConfigurations{
		{URL: strings.TrimRight(baseURL, "/") + managementAPIPath},
	}
	cfg.HTTPClient = httpClient
	cfg.UserAgent = "terraform-provider-polaris"
	cfg.AddDefaultHeader("Authorization", "Bearer "+token)
	if realm != "" {
		cfg.AddDefaultHeader(realmHeader, realm)
	}
	return polarismanagement.NewAPIClient(cfg)
}

// checkResponse validates the result of a generated client call. The generated client
// returns an error for every non-2xx status; this normalizes that into a single error
// stating the method, path, status and response body whenever the status is not want.
// A nil return means the call succeeded with the expected status and decoded cleanly.
func checkResponse(httpResp *http.Response, err error, want int) error {
	if httpResp == nil {
		if err == nil {
			err = errors.New("no response from server")
		}
		return err
	}
	if httpResp.StatusCode != want {
		msg := fmt.Sprintf("HTTP %d (expected %d)", httpResp.StatusCode, want)
		if httpResp.Request != nil {
			msg = fmt.Sprintf("%s %s returned %s", httpResp.Request.Method, httpResp.Request.URL.Path, msg)
		}
		var apiErr *polarismanagement.GenericOpenAPIError
		if errors.As(err, &apiErr) && len(apiErr.Body()) > 0 {
			msg += ": " + strings.TrimSpace(string(apiErr.Body()))
		}
		return errors.New(msg)
	}
	// Expected status, but the generated client may still have failed to decode the body.
	return err
}

func isNotFound(httpResp *http.Response) bool {
	return httpResp != nil && httpResp.StatusCode == http.StatusNotFound
}

// doJSON sends a raw JSON request to the management API through the client's
// configuration (base URL, HTTP client, auth and realm headers). It is used for endpoints
// whose polymorphic payloads the generated models cannot represent (grants, catalogs).
// path is relative to /api/management/v1 and must already be escaped.
func doJSON(ctx context.Context, client *polarismanagement.APIClient, method, path string, body any, want int, out any) error {
	base, err := client.GetConfig().ServerURLWithContext(ctx, "")
	if err != nil {
		return fmt.Errorf("resolve server URL: %w", err)
	}
	return doRequest(ctx, client, method, base+path, body, want, out)
}

// doCatalogJSON is doJSON for the catalog API, which has no generated client (see
// spec/README.md). path is relative to /api/catalog and must already be escaped: build it
// with icebergCatalogPath or polarisCatalogPath.
func doCatalogJSON(ctx context.Context, client *polarismanagement.APIClient, method, path string, body any, want int, out any) error {
	base, err := client.GetConfig().ServerURLWithContext(ctx, "")
	if err != nil {
		return fmt.Errorf("resolve server URL: %w", err)
	}
	root := strings.TrimSuffix(base, managementAPIPath)
	return doRequest(ctx, client, method, root+catalogAPIPath+path, body, want, out)
}

// icebergCatalogPath returns the path of an Iceberg REST catalog API resource of the
// catalog, relative to /api/catalog: /v1/{catalog}/{segments...}. Segments must already be
// escaped (use namespacePath for a namespace).
func icebergCatalogPath(catalog string, segments ...string) string {
	return catalogResourcePath("/v1/", catalog, segments)
}

// polarisCatalogPath is icebergCatalogPath for the Polaris-native catalog APIs
// (policies, generic tables, semantic models): /polaris/v1/{catalog}/{segments...}.
func polarisCatalogPath(catalog string, segments ...string) string {
	return catalogResourcePath("/polaris/v1/", catalog, segments)
}

func catalogResourcePath(prefix, catalog string, segments []string) string {
	p := prefix + url.PathEscape(catalog)
	for _, s := range segments {
		p += "/" + s
	}
	return p
}

// namespacePath encodes a namespace for a URL path segment: its levels joined by the
// unit separator (0x1F), then escaped, as the Iceberg REST spec requires ("a.b" -> "a%1Fb").
func namespacePath(namespace []string) string {
	return url.PathEscape(strings.Join(namespace, "\x1f"))
}

// doRequest sends a JSON request to a full URL with the client's HTTP client, user agent,
// auth and realm headers, and decodes the response into out when the status is want.
func doRequest(ctx context.Context, client *polarismanagement.APIClient, method, fullURL string, body any, want int, out any) error {
	cfg := client.GetConfig()

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, reqBody)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", cfg.UserAgent)
	for k, v := range cfg.DefaultHeader {
		req.Header.Set(k, v)
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if res.StatusCode != want {
		return &unexpectedStatusError{
			StatusCode: res.StatusCode,
			msg:        fmt.Sprintf("%s %s returned HTTP %d (expected %d): %s", method, req.URL.EscapedPath(), res.StatusCode, want, strings.TrimSpace(string(respBody))),
		}
	}
	if out != nil && len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// unexpectedStatusError is returned by doJSON when the server answers with a status
// other than the expected one, so callers can tell a 404 apart from other failures.
type unexpectedStatusError struct {
	StatusCode int
	msg        string
}

func (e *unexpectedStatusError) Error() string { return e.msg }

// isNotFoundError reports whether err is a doJSON error for an HTTP 404 response.
func isNotFoundError(err error) bool {
	var se *unexpectedStatusError
	return errors.As(err, &se) && se.StatusCode == http.StatusNotFound
}
