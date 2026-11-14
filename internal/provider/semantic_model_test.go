package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// listSemanticModels reads the identifiers of every page.
func TestListSemanticModels_pagination(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.EscapedPath()+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = io.WriteString(w, `{"identifiers":[{"namespace":["sales","eu"],"name":"revenue"}],"next-page-token":"p2"}`)
			return
		}
		_, _ = io.WriteString(w, `{"identifiers":[{"namespace":["sales","eu"],"name":"margin"}],"next-page-token":null}`)
	}))
	t.Cleanup(srv.Close)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	got, err := listSemanticModels(context.Background(), client, "lake", []string{"sales", "eu"})
	if err != nil {
		t.Fatalf("listSemanticModels: %v", err)
	}
	if !reflect.DeepEqual(got, []string{"revenue", "margin"}) {
		t.Errorf("got %q", got)
	}
	want := []string{
		"/api/catalog/polaris/v1/lake/namespaces/sales%1Feu/semantic-models?",
		"/api/catalog/polaris/v1/lake/namespaces/sales%1Feu/semantic-models?pageToken=p2",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("requests = %q\nwant       %q", paths, want)
	}
}

// The update body carries the entity version and the document, as the API expects.
func TestUpdateSemanticModel_requestBody(t *testing.T) {
	srv, reqs := newTestServer(t, http.StatusOK, `{"document":{"version":"0.2.0.dev0","semantic_model":"{}"},"entity-version":"3"}`)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	out, err := updateSemanticModel(context.Background(), client, "lake", []string{"sales"}, "revenue", updateSemanticModelPayload{
		EntityVersion: "2",
		Document:      semanticModelDocument{Version: "0.2.0.dev0", SemanticModel: "{}"},
	})
	if err != nil {
		t.Fatalf("updateSemanticModel: %v", err)
	}
	r := onlyRequest(t, reqs())
	if r.Method != http.MethodPut || r.Path != "/api/catalog/polaris/v1/lake/namespaces/sales/semantic-models/revenue" {
		t.Errorf("request = %s %s", r.Method, r.Path)
	}
	if r.Body != `{"entity-version":"2","document":{"version":"0.2.0.dev0","semantic_model":"{}"}}` {
		t.Errorf("body = %s", r.Body)
	}
	if out.EntityVersion != "3" {
		t.Errorf("entity version = %q", out.EntityVersion)
	}
}
