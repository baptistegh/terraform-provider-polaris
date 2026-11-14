package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
)

// recordedRequest is what a test server saw for one request.
type recordedRequest struct {
	Method string
	Path   string
	Header http.Header
	Body   string
}

// newTestServer starts an httptest server that records every request and answers
// with the given status and body. The recorded requests are returned via the func.
func newTestServer(t *testing.T, status int, body string) (*httptest.Server, func() []recordedRequest) {
	t.Helper()
	var (
		mu   sync.Mutex
		reqs []recordedRequest
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqs = append(reqs, recordedRequest{Method: r.Method, Path: r.URL.EscapedPath(), Header: r.Header.Clone(), Body: string(b)})
		mu.Unlock()
		if body != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []recordedRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]recordedRequest(nil), reqs...)
	}
}

func onlyRequest(t *testing.T, reqs []recordedRequest) recordedRequest {
	t.Helper()
	if len(reqs) != 1 {
		t.Fatalf("expected exactly 1 request, got %d", len(reqs))
	}
	return reqs[0]
}

func TestNewManagementClient_setsURLAndHeaders(t *testing.T) {
	srv, reqs := newTestServer(t, http.StatusOK, `{"name":"alice"}`)

	// Trailing slash on the base URL must not produce a double slash.
	client := newManagementClient(srv.URL+"/", srv.Client(), "tok-123", "my-realm", "X-Realm")

	p, httpResp, err := client.DefaultAPI.GetPrincipal(context.Background(), "alice").Execute()
	if err := checkResponse(httpResp, err, http.StatusOK); err != nil {
		t.Fatalf("GetPrincipal: %v", err)
	}
	if p.Name != "alice" {
		t.Errorf("decoded name = %q, want %q", p.Name, "alice")
	}

	r := onlyRequest(t, reqs())
	if r.Method != http.MethodGet || r.Path != "/api/management/v1/principals/alice" {
		t.Errorf("request = %s %s, want GET /api/management/v1/principals/alice", r.Method, r.Path)
	}
	if got := r.Header.Get("Authorization"); got != "Bearer tok-123" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer tok-123")
	}
	if got := r.Header.Get("X-Realm"); got != "my-realm" {
		t.Errorf("realm header = %q, want %q", got, "my-realm")
	}
	if got := r.Header.Get("User-Agent"); got != "terraform-provider-polaris" {
		t.Errorf("User-Agent = %q, want %q", got, "terraform-provider-polaris")
	}
}

func TestNewManagementClient_omitsEmptyRealm(t *testing.T) {
	srv, reqs := newTestServer(t, http.StatusNoContent, "")

	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	httpResp, err := client.DefaultAPI.DeletePrincipal(context.Background(), "bob").Execute()
	if err := checkResponse(httpResp, err, http.StatusNoContent); err != nil {
		t.Fatalf("DeletePrincipal: %v", err)
	}

	r := onlyRequest(t, reqs())
	if _, ok := r.Header[http.CanonicalHeaderKey(defaultRealmHeader)]; ok {
		t.Errorf("realm header %q sent although realm is empty", defaultRealmHeader)
	}
}

func TestCheckResponse(t *testing.T) {
	transportErr := errors.New("connection refused")
	decodeErr := errors.New("bad json")
	okResp := &http.Response{StatusCode: http.StatusOK}

	tests := []struct {
		name    string
		resp    *http.Response
		err     error
		want    int
		wantErr error  // checked with errors.Is when set
		wantMsg string // substring of the error message when set
	}{
		{name: "expected status, no error", resp: okResp, want: http.StatusOK},
		{name: "no response, transport error", err: transportErr, want: http.StatusOK, wantErr: transportErr},
		{name: "no response, no error", want: http.StatusOK, wantMsg: "no response from server"},
		{name: "expected status, decode error", resp: okResp, err: decodeErr, want: http.StatusOK, wantErr: decodeErr},
		{name: "unexpected status without request", resp: &http.Response{StatusCode: http.StatusTeapot}, want: http.StatusOK, wantMsg: "HTTP 418 (expected 200)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkResponse(tt.resp, tt.err, tt.want)
			switch {
			case tt.wantErr == nil && tt.wantMsg == "":
				if got != nil {
					t.Fatalf("got error %v, want nil", got)
				}
			case tt.wantErr != nil:
				if !errors.Is(got, tt.wantErr) {
					t.Fatalf("got error %v, want %v", got, tt.wantErr)
				}
			default:
				if got == nil || !strings.Contains(got.Error(), tt.wantMsg) {
					t.Fatalf("got error %v, want it to contain %q", got, tt.wantMsg)
				}
			}
		})
	}
}

// A real non-2xx response from the generated client: the error must name the
// method, path and status, and carry the server's response body.
func TestCheckResponse_includesAPIErrorBody(t *testing.T) {
	const apiBody = `{"error":{"message":"Principal already exists","code":409}}`
	srv, _ := newTestServer(t, http.StatusConflict, apiBody)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	_, httpResp, err := client.DefaultAPI.CreatePrincipal(context.Background()).
		CreatePrincipalRequest(polarismanagement.CreatePrincipalRequest{Principal: &polarismanagement.Principal{Name: "alice"}}).
		Execute()

	var apiErr *polarismanagement.GenericOpenAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("generated client error = %T (%v), want *GenericOpenAPIError", err, err)
	}

	got := checkResponse(httpResp, err, http.StatusCreated)
	if got == nil {
		t.Fatal("got nil error for HTTP 409")
	}
	for _, want := range []string{"POST /api/management/v1/principals", "HTTP 409 (expected 201)", "Principal already exists"} {
		if !strings.Contains(got.Error(), want) {
			t.Errorf("error %q does not contain %q", got.Error(), want)
		}
	}
}

// A successful status other than the expected one is still an error, even though
// the generated client itself reports no error for it.
func TestCheckResponse_unexpectedSuccessStatus(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusOK, "")
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	httpResp, err := client.DefaultAPI.DeletePrincipal(context.Background(), "alice").Execute()
	if err != nil {
		t.Fatalf("generated client returned error for HTTP 200: %v", err)
	}
	got := checkResponse(httpResp, err, http.StatusNoContent)
	if got == nil || !strings.Contains(got.Error(), "DELETE /api/management/v1/principals/alice returned HTTP 200 (expected 204)") {
		t.Fatalf("got error %v, want an unexpected-status error", got)
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		resp *http.Response
		want bool
	}{
		{name: "nil response", resp: nil, want: false},
		{name: "404", resp: &http.Response{StatusCode: http.StatusNotFound}, want: true},
		{name: "200", resp: &http.Response{StatusCode: http.StatusOK}, want: false},
		{name: "403", resp: &http.Response{StatusCode: http.StatusForbidden}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNotFound(tt.resp); got != tt.want {
				t.Errorf("isNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDoJSON_sendsBodyAndDecodesResponse(t *testing.T) {
	srv, reqs := newTestServer(t, http.StatusCreated, `{"grants":[{"type":"catalog","privilege":"CATALOG_MANAGE_CONTENT"}]}`)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "POLARIS", defaultRealmHeader)

	var out grantsListResponse
	in := map[string]any{"grant": map[string]string{"type": "catalog", "privilege": "CATALOG_MANAGE_CONTENT"}}
	if err := doJSON(context.Background(), client, http.MethodPut, "/catalogs/c1/catalog-roles/r1/grants", in, http.StatusCreated, &out); err != nil {
		t.Fatalf("doJSON: %v", err)
	}

	if len(out.Grants) != 1 || out.Grants[0].Privilege != "CATALOG_MANAGE_CONTENT" {
		t.Errorf("decoded = %+v, want one CATALOG_MANAGE_CONTENT grant", out.Grants)
	}

	r := onlyRequest(t, reqs())
	if r.Method != http.MethodPut || r.Path != "/api/management/v1/catalogs/c1/catalog-roles/r1/grants" {
		t.Errorf("request = %s %s", r.Method, r.Path)
	}
	for header, want := range map[string]string{
		"Authorization":    "Bearer tok",
		defaultRealmHeader: "POLARIS",
		"Content-Type":     "application/json",
		"Accept":           "application/json",
		"User-Agent":       "terraform-provider-polaris",
	} {
		if got := r.Header.Get(header); got != want {
			t.Errorf("header %s = %q, want %q", header, got, want)
		}
	}
	var sent map[string]map[string]string
	if err := json.Unmarshal([]byte(r.Body), &sent); err != nil {
		t.Fatalf("request body %q is not JSON: %v", r.Body, err)
	}
	if sent["grant"]["privilege"] != "CATALOG_MANAGE_CONTENT" {
		t.Errorf("request body = %s", r.Body)
	}
}

func TestDoJSON_noBody(t *testing.T) {
	srv, reqs := newTestServer(t, http.StatusOK, `{"grants":[]}`)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	// nil out: the response body is ignored.
	if err := doJSON(context.Background(), client, http.MethodGet, "/catalogs/c1/catalog-roles/r1/grants", nil, http.StatusOK, nil); err != nil {
		t.Fatalf("doJSON: %v", err)
	}

	r := onlyRequest(t, reqs())
	if r.Body != "" {
		t.Errorf("request body = %q, want empty", r.Body)
	}
	if got := r.Header.Get("Content-Type"); got != "" {
		t.Errorf("Content-Type = %q, want unset for a request without body", got)
	}
}

func TestDoJSON_unexpectedStatus(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusForbidden, `{"error":{"message":"not authorized"}}`)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	err := doJSON(context.Background(), client, http.MethodPost, "/catalogs/c1/catalog-roles/r1/grants", map[string]any{}, http.StatusCreated, nil)
	if err == nil {
		t.Fatal("got nil error for HTTP 403")
	}
	for _, want := range []string{"POST /api/management/v1/catalogs/c1/catalog-roles/r1/grants", "HTTP 403 (expected 201)", "not authorized"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

func TestDoJSON_invalidResponseJSON(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusOK, `not json`)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	var out grantsListResponse
	err := doJSON(context.Background(), client, http.MethodGet, "/x", nil, http.StatusOK, &out)
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("got error %v, want a decode error", err)
	}
}

func TestDoJSON_unmarshalableBody(t *testing.T) {
	srv, reqs := newTestServer(t, http.StatusOK, "")
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	err := doJSON(context.Background(), client, http.MethodPut, "/x", map[string]any{"bad": make(chan int)}, http.StatusOK, nil)
	if err == nil || !strings.Contains(err.Error(), "marshal request") {
		t.Fatalf("got error %v, want a marshal error", err)
	}
	if n := len(reqs()); n != 0 {
		t.Errorf("%d request(s) sent for an unmarshalable body, want 0", n)
	}
}

func TestDoJSON_transportError(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusOK, "")
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)
	srv.Close() // nothing listens any more

	if err := doJSON(context.Background(), client, http.MethodGet, "/x", nil, http.StatusOK, nil); err == nil {
		t.Fatal("got nil error with the server closed")
	}
}
