package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSplitNamespaceProperties(t *testing.T) {
	loc, rest := splitNamespaceProperties(map[string]string{"location": "file:///tmp/c/a/", "owner": "data"})
	if loc == nil || *loc != "file:///tmp/c/a/" {
		t.Errorf("location = %v, want file:///tmp/c/a/", loc)
	}
	if !reflect.DeepEqual(rest, map[string]string{"owner": "data"}) {
		t.Errorf("rest = %v, want only owner", rest)
	}

	loc, rest = splitNamespaceProperties(nil)
	if loc != nil || len(rest) != 0 {
		t.Errorf("no properties: location = %v, rest = %v", loc, rest)
	}
}

func TestNamespacePropertiesUpdate(t *testing.T) {
	ptr := func(s string) *string { return &s }
	// What Polaris returns for a namespace created with {"owner": "data", "tier": "gold"}.
	current := map[string]string{"location": "file:///tmp/c/a/", "owner": "data", "tier": "gold"}

	for _, tc := range []struct {
		name     string
		desired  map[string]string
		location *string
		want     updateNamespacePropertiesPayload
		wantOK   bool
	}{
		{
			name:    "unchanged properties, location not configured: nothing to do",
			desired: map[string]string{"owner": "data", "tier": "gold"},
		},
		{
			// The bug this guards against: location is not in the configured properties,
			// but must not be removed.
			name:    "removing a property keeps the server location",
			desired: map[string]string{"owner": "data"},
			want:    updateNamespacePropertiesPayload{Removals: []string{"tier"}},
			wantOK:  true,
		},
		{
			name:    "all properties removed, location kept",
			desired: map[string]string{},
			want:    updateNamespacePropertiesPayload{Removals: []string{"owner", "tier"}},
			wantOK:  true,
		},
		{
			name:    "changed and added properties",
			desired: map[string]string{"owner": "platform", "tier": "gold", "env": "prod"},
			want:    updateNamespacePropertiesPayload{Updates: map[string]string{"owner": "platform", "env": "prod"}},
			wantOK:  true,
		},
		{
			name:     "explicit location change",
			desired:  map[string]string{"owner": "data", "tier": "gold"},
			location: ptr("file:///tmp/elsewhere/a/"),
			want:     updateNamespacePropertiesPayload{Updates: map[string]string{"location": "file:///tmp/elsewhere/a/"}},
			wantOK:   true,
		},
		{
			name:     "explicit location equal to the server value: nothing to do",
			desired:  map[string]string{"owner": "data", "tier": "gold"},
			location: ptr("file:///tmp/c/a/"),
		},
		{
			name:    "location inside desired properties is ignored",
			desired: map[string]string{"owner": "data", "tier": "gold", "location": "file:///x/"},
		},
	} {
		got, ok := namespacePropertiesUpdate(current, tc.desired, tc.location)
		if ok != tc.wantOK || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s:\n got  %+v (ok=%v)\n want %+v (ok=%v)", tc.name, got, ok, tc.want, tc.wantOK)
		}
	}
}

// The payloads decode the responses Polaris 1.9.0 returns (captured from a local server).
func TestNamespacePayloads_decodePolarisResponses(t *testing.T) {
	var ns namespacePayload
	if err := json.Unmarshal([]byte(`{"namespace":["sales","eu"],"properties":{"location":"file:///tmp/c/sales/eu/","k":"v"}}`), &ns); err != nil {
		t.Fatal(err)
	}
	loc, rest := splitNamespaceProperties(ns.Properties)
	if !reflect.DeepEqual(ns.Namespace, []string{"sales", "eu"}) || loc == nil || *loc != "file:///tmp/c/sales/eu/" || rest["k"] != "v" {
		t.Errorf("decoded %+v", ns)
	}

	var list listNamespacesPayload
	if err := json.Unmarshal([]byte(`{"namespaces":[["a"],["b"]],"next-page-token":null}`), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Namespaces) != 2 || list.NextPageToken != nil {
		t.Errorf("decoded %+v", list)
	}
}

// The update request is sent as the Iceberg REST spec expects: empty fields are omitted.
func TestUpdateNamespaceProperties_requestBody(t *testing.T) {
	srv, reqs := newTestServer(t, http.StatusOK, `{"removed":["tier"],"updated":[],"missing":[]}`)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	req, _ := namespacePropertiesUpdate(map[string]string{"location": "l", "tier": "gold"}, map[string]string{}, nil)
	if err := updateNamespaceProperties(context.Background(), client, "c1", []string{"a"}, req); err != nil {
		t.Fatalf("updateNamespaceProperties: %v", err)
	}
	r := onlyRequest(t, reqs())
	if r.Body != `{"removals":["tier"]}` {
		t.Errorf("body = %s, want {\"removals\":[\"tier\"]}", r.Body)
	}
	if r.Path != "/api/catalog/v1/c1/namespaces/a/properties" {
		t.Errorf("path = %s", r.Path)
	}
}

// Polaris 1.9.0 answers 409 to a request with both removals and updates: they go separately.
func TestUpdateNamespaceProperties_splitsUpdatesAndRemovals(t *testing.T) {
	srv, reqs := newTestServer(t, http.StatusOK, `{"removed":[],"updated":[],"missing":[]}`)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	req := updateNamespacePropertiesPayload{Removals: []string{"tier"}, Updates: map[string]string{"owner": "platform"}}
	if err := updateNamespaceProperties(context.Background(), client, "c1", []string{"a"}, req); err != nil {
		t.Fatalf("updateNamespaceProperties: %v", err)
	}
	got := reqs()
	if len(got) != 2 || got[0].Body != `{"updates":{"owner":"platform"}}` || got[1].Body != `{"removals":["tier"]}` {
		t.Errorf("requests = %+v, want updates then removals", got)
	}
}

func TestParseNamespaceImportID(t *testing.T) {
	cat, ns, err := parseNamespaceImportID("lakehouse/sales/eu")
	if err != nil || cat != "lakehouse" || !reflect.DeepEqual(ns, []string{"sales", "eu"}) {
		t.Errorf("got %q %q %v", cat, ns, err)
	}
	for _, bad := range []string{"", "lakehouse", "lakehouse/", "/sales", "lakehouse//eu"} {
		if _, _, err := parseNamespaceImportID(bad); err == nil {
			t.Errorf("parseNamespaceImportID(%q): want an error", bad)
		}
	}
}

func TestSameLocation(t *testing.T) {
	if !sameLocation("s3://b/ns", "s3://b/ns/") || !sameLocation("s3://b/ns/", "s3://b/ns/") {
		t.Error("trailing slash should not matter")
	}
	if sameLocation("s3://b/ns", "s3://b/ns2/") {
		t.Error("different locations reported equal")
	}
}

// listNamespaces follows next-page-token and sends parent with the unit separator.
func TestListNamespaces_pagination(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = io.WriteString(w, `{"namespaces":[["sales","eu"]],"next-page-token":"p2"}`)
			return
		}
		_, _ = io.WriteString(w, `{"namespaces":[["sales","us"]],"next-page-token":null}`)
	}))
	t.Cleanup(srv.Close)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	got, err := listNamespaces(context.Background(), client, "c1", []string{"sales"})
	if err != nil {
		t.Fatalf("listNamespaces: %v", err)
	}
	if !reflect.DeepEqual(got, [][]string{{"sales", "eu"}, {"sales", "us"}}) {
		t.Errorf("got %v", got)
	}
	if len(queries) != 2 || queries[0] != "parent=sales" || queries[1] != "pageToken=p2&parent=sales" {
		t.Errorf("queries = %q", queries)
	}
}
