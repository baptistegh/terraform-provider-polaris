package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
)

// newManagementClient builds a client for the Polaris management API (/api/management/v1)
// that authenticates every request with the given Bearer token and realm header.
func newManagementClient(baseURL string, httpClient *http.Client, token, realm, realmHeader string) *polarismanagement.APIClient {
	cfg := polarismanagement.NewConfiguration()
	cfg.Servers = polarismanagement.ServerConfigurations{
		{URL: strings.TrimRight(baseURL, "/") + "/api/management/v1"},
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

// doJSON sends a raw JSON request through the management client's configuration
// (base URL, HTTP client, auth and realm headers). It is used for endpoints whose
// polymorphic payloads the generated models cannot represent (grants).
// path is relative to /api/management/v1 and must already be escaped.
func doJSON(ctx context.Context, client *polarismanagement.APIClient, method, path string, body any, want int, out any) error {
	cfg := client.GetConfig()
	baseURL, err := cfg.ServerURLWithContext(ctx, "")
	if err != nil {
		return fmt.Errorf("resolve server URL: %w", err)
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reqBody)
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
			msg:        fmt.Sprintf("%s %s returned HTTP %d (expected %d): %s", method, req.URL.Path, res.StatusCode, want, strings.TrimSpace(string(respBody))),
		}
	}
	if out != nil {
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
