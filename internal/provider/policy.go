package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
)

// Policies are Polaris-native catalog API objects under /polaris/v1/{catalog}
// (doCatalogJSON, polarisCatalogPath). Shared by polaris_policy, polaris_policy_attachment,
// their data sources and the test fixtures.

// Policy attachment target types, as the API names them.
const (
	policyTargetCatalog   = "catalog"
	policyTargetNamespace = "namespace"
	policyTargetTable     = "table-like"
)

type createPolicyPayload struct {
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Description *string `json:"description,omitempty"`
	Content     string  `json:"content"`
}

type updatePolicyPayload struct {
	CurrentPolicyVersion int64   `json:"current-policy-version"`
	Description          *string `json:"description,omitempty"`
	Content              string  `json:"content"`
}

type policyPayload struct {
	PolicyType  string  `json:"policy-type"`
	Inheritable bool    `json:"inheritable"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Content     string  `json:"content"`
	Version     int64   `json:"version"`
}

type loadPolicyPayload struct {
	Policy policyPayload `json:"policy"`
}

type policyIdentifier struct {
	Namespace []string `json:"namespace"`
	Name      string   `json:"name"`
}

type listPoliciesPayload struct {
	Identifiers   []policyIdentifier `json:"identifiers"`
	NextPageToken *string            `json:"next-page-token,omitempty"`
}

type policyTarget struct {
	Type string   `json:"type"`
	Path []string `json:"path,omitempty"`
}

type attachPolicyPayload struct {
	Target     policyTarget      `json:"target"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

type detachPolicyPayload struct {
	Target policyTarget `json:"target"`
}

// applicablePolicy is a policy returned by getApplicablePolicies: Inherited is false when
// the policy is attached to the queried target itself.
type applicablePolicy struct {
	policyPayload
	Inherited bool     `json:"inherited"`
	Namespace []string `json:"namespace"`
}

type applicablePoliciesPayload struct {
	ApplicablePolicies []applicablePolicy `json:"applicable-policies"`
	NextPageToken      *string            `json:"next-page-token,omitempty"`
}

func policyPath(catalog string, namespace []string, name string, segments ...string) string {
	return polarisCatalogPath(catalog, append([]string{"namespaces", namespacePath(namespace), "policies", url.PathEscape(name)}, segments...)...)
}

func createPolicy(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, p createPolicyPayload) (*policyPayload, error) {
	var out loadPolicyPayload
	path := polarisCatalogPath(catalog, "namespaces", namespacePath(namespace), "policies")
	if err := doCatalogJSON(ctx, client, http.MethodPost, path, p, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out.Policy, nil
}

// getPolicy returns the policy; use isNotFoundError on the error to detect a missing one.
func getPolicy(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, name string) (*policyPayload, error) {
	var out loadPolicyPayload
	if err := doCatalogJSON(ctx, client, http.MethodGet, policyPath(catalog, namespace, name), nil, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out.Policy, nil
}

func updatePolicy(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, name string, p updatePolicyPayload) (*policyPayload, error) {
	var out loadPolicyPayload
	if err := doCatalogJSON(ctx, client, http.MethodPut, policyPath(catalog, namespace, name), p, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out.Policy, nil
}

func dropPolicy(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, name string) error {
	return doCatalogJSON(ctx, client, http.MethodDelete, policyPath(catalog, namespace, name), nil, http.StatusNoContent, nil)
}

// listPolicies returns the names of the policies defined in namespace, optionally only
// those of policyType, following next-page-token until the last page.
func listPolicies(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, policyType string) ([]string, error) {
	base := polarisCatalogPath(catalog, "namespaces", namespacePath(namespace), "policies")
	var names []string
	err := paginate(func(token string) (*string, error) {
		q := url.Values{}
		if policyType != "" {
			q.Set("policyType", policyType)
		}
		if token != "" {
			q.Set("pageToken", token)
		}
		var page listPoliciesPayload
		if err := doCatalogJSON(ctx, client, http.MethodGet, withQuery(base, q), nil, http.StatusOK, &page); err != nil {
			return nil, err
		}
		for _, id := range page.Identifiers {
			names = append(names, id.Name)
		}
		return page.NextPageToken, nil
	})
	return names, err
}

func attachPolicy(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, name string, p attachPolicyPayload) error {
	return doCatalogJSON(ctx, client, http.MethodPut, policyPath(catalog, namespace, name, "mappings"), p, http.StatusNoContent, nil)
}

// detachPolicy removes the attachment; use isNotFoundError on the error to detect one
// that does not exist.
func detachPolicy(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, name string, target policyTarget) error {
	return doCatalogJSON(ctx, client, http.MethodPost, policyPath(catalog, namespace, name, "mappings"), detachPolicyPayload{Target: target}, http.StatusNoContent, nil)
}

// applicablePolicies returns the policies that apply to target: attached to it directly
// (Inherited false) or inherited from its parents.
func applicablePolicies(ctx context.Context, client *polarismanagement.APIClient, catalog string, target policyTarget) ([]applicablePolicy, error) {
	if target.Type == policyTargetTable && len(target.Path) < 2 {
		return nil, fmt.Errorf("a %s target needs a namespace and a name, got %q", policyTargetTable, target.Path)
	}
	base := polarisCatalogPath(catalog, "applicable-policies")
	var all []applicablePolicy
	err := paginate(func(token string) (*string, error) {
		q := url.Values{}
		switch target.Type {
		case policyTargetNamespace:
			q.Set("namespace", strings.Join(target.Path, "\x1f"))
		case policyTargetTable:
			q.Set("namespace", strings.Join(target.Path[:len(target.Path)-1], "\x1f"))
			q.Set("target-name", target.Path[len(target.Path)-1])
		}
		if token != "" {
			q.Set("pageToken", token)
		}
		var out applicablePoliciesPayload
		if err := doCatalogJSON(ctx, client, http.MethodGet, withQuery(base, q), nil, http.StatusOK, &out); err != nil {
			return nil, err
		}
		all = append(all, out.ApplicablePolicies...)
		return out.NextPageToken, nil
	})
	return all, err
}

// isAttached reports whether the policy is directly attached to the target. Polaris allows
// one policy of each type per target, and getApplicablePolicies reports a policy attached
// to the target itself with inherited = false.
func isAttached(policies []applicablePolicy, namespace []string, name string) bool {
	for _, p := range policies {
		if !p.Inherited && p.Name == name && namespaceEqual(p.Namespace, namespace) {
			return true
		}
	}
	return false
}

// paginate calls fetch with an empty token, then with each next-page-token it returns,
// until the token is null or empty.
func paginate(fetch func(token string) (*string, error)) error {
	token := ""
	for {
		next, err := fetch(token)
		if err != nil {
			return err
		}
		if next == nil || *next == "" {
			return nil
		}
		token = *next
	}
}

func withQuery(path string, q url.Values) string {
	if len(q) == 0 {
		return path
	}
	return path + "?" + q.Encode()
}
