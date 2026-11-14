package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestValidatePolicyTarget(t *testing.T) {
	for _, tc := range []struct {
		targetType string
		levels     int
		ok         bool
	}{
		{"catalog", 0, true},
		{"catalog", 1, false},
		{"namespace", 1, true},
		{"namespace", 3, true},
		{"namespace", 0, false},
		{"table-like", 2, true},
		{"table-like", 1, false},
		{"table", 2, false},
	} {
		if err := validatePolicyTarget(tc.targetType, tc.levels); (err == nil) != tc.ok {
			t.Errorf("validatePolicyTarget(%q, %d) = %v, want ok=%v", tc.targetType, tc.levels, err, tc.ok)
		}
	}
}

func TestParsePolicyAttachmentImportID(t *testing.T) {
	cat, ns, name, target, err := parsePolicyAttachmentImportID("lake/gov.eu/compaction/table-like/sales.eu.orders")
	if err != nil || cat != "lake" || !reflect.DeepEqual(ns, []string{"gov", "eu"}) || name != "compaction" ||
		target.Type != "table-like" || !reflect.DeepEqual(target.Path, []string{"sales", "eu", "orders"}) {
		t.Errorf("got %q %q %q %+v %v", cat, ns, name, target, err)
	}
	if _, _, _, target, err := parsePolicyAttachmentImportID("lake/gov/compaction/catalog"); err != nil || target.Path != nil {
		t.Errorf("catalog target: %+v %v", target, err)
	}
	for _, bad := range []string{"lake/gov/compaction", "lake/gov/compaction/namespace", "lake/gov/compaction/catalog/x", "lake//compaction/catalog", "a/b/c/d/e/f"} {
		if _, _, _, _, err := parsePolicyAttachmentImportID(bad); err == nil {
			t.Errorf("parsePolicyAttachmentImportID(%q): want an error", bad)
		}
	}
}

func TestParseNamespacedImportID(t *testing.T) {
	cat, ns, name, err := parseNamespacedImportID("lake/a/b/p")
	if err != nil || cat != "lake" || !reflect.DeepEqual(ns, []string{"a", "b"}) || name != "p" {
		t.Errorf("got %q %q %q %v", cat, ns, name, err)
	}
	for _, bad := range []string{"lake/p", "lake//p", "lake/a/"} {
		if _, _, _, err := parseNamespacedImportID(bad); err == nil {
			t.Errorf("parseNamespacedImportID(%q): want an error", bad)
		}
	}
}

func TestIsAttached(t *testing.T) {
	policies := []applicablePolicy{
		{policyPayload: policyPayload{Name: "inherited"}, Inherited: true, Namespace: []string{"gov"}},
		{policyPayload: policyPayload{Name: "direct"}, Inherited: false, Namespace: []string{"gov"}},
	}
	if !isAttached(policies, []string{"gov"}, "direct") {
		t.Error("direct attachment not found")
	}
	if isAttached(policies, []string{"gov"}, "inherited") {
		t.Error("an inherited policy is not attached to the target")
	}
	if isAttached(policies, []string{"other"}, "direct") {
		t.Error("same name in another namespace matched")
	}
}

// applicablePolicies queries the target with the parameters the API expects and follows pages.
func TestApplicablePolicies_queryAndPagination(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = io.WriteString(w, `{"applicable-policies":[{"name":"a","policy-type":"system.data-compaction","inherited":false,"namespace":["gov"],"content":"{}","version":0}],"next-page-token":"n"}`)
			return
		}
		_, _ = io.WriteString(w, `{"applicable-policies":[{"name":"b","inherited":true,"namespace":["gov"]}],"next-page-token":null}`)
	}))
	t.Cleanup(srv.Close)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	got, err := applicablePolicies(context.Background(), client, "lake", policyTarget{Type: policyTargetTable, Path: []string{"sales", "eu", "orders"}})
	if err != nil {
		t.Fatalf("applicablePolicies: %v", err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[0].Inherited || !got[1].Inherited {
		t.Errorf("got %+v", got)
	}
	want := []string{
		"/api/catalog/polaris/v1/lake/applicable-policies?namespace=sales%1Feu&target-name=orders",
		"/api/catalog/polaris/v1/lake/applicable-policies?namespace=sales%1Feu&pageToken=n&target-name=orders",
	}
	if !reflect.DeepEqual(queries, want) {
		t.Errorf("queries = %q\nwant      %q", queries, want)
	}

	if _, err := applicablePolicies(context.Background(), client, "lake", policyTarget{Type: policyTargetTable, Path: []string{"orders"}}); err == nil {
		t.Error("a table target without namespace: want an error")
	}
}
