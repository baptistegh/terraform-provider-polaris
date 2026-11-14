package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
)

// Namespaces are Iceberg REST catalog API objects (doCatalogJSON, icebergCatalogPath,
// namespacePath). These payloads and helpers are shared by the namespace fixtures in the
// tests and the polaris_namespace resource and data sources (#23).

// namespaceLocationProperty is the namespace property holding its storage location.
// Polaris sets it on create (to <catalog base location>/<namespace path>/) and only
// accepts a different value when ALLOW_NAMESPACE_CUSTOM_LOCATION is enabled. It can be
// removed through updateProperties, which would leave the namespace without a location,
// so it is kept out of the user-managed properties: see splitNamespaceProperties and
// namespacePropertiesUpdate.
const namespaceLocationProperty = "location"

type createNamespacePayload struct {
	Namespace  []string          `json:"namespace"`
	Properties map[string]string `json:"properties,omitempty"`
}

// namespacePayload is the response of createNamespace and loadNamespaceMetadata.
type namespacePayload struct {
	Namespace  []string          `json:"namespace"`
	Properties map[string]string `json:"properties,omitempty"`
}

type updateNamespacePropertiesPayload struct {
	Removals []string          `json:"removals,omitempty"`
	Updates  map[string]string `json:"updates,omitempty"`
}

type listNamespacesPayload struct {
	Namespaces    [][]string `json:"namespaces"`
	NextPageToken *string    `json:"next-page-token,omitempty"`
}

// splitNamespaceProperties separates the server-managed location from the other
// namespace properties. location is nil when the namespace has none.
func splitNamespaceProperties(props map[string]string) (location *string, rest map[string]string) {
	rest = make(map[string]string, len(props))
	for k, v := range props {
		if k == namespaceLocationProperty {
			location = &v // per-iteration variable since Go 1.22
			continue
		}
		rest[k] = v
	}
	return location, rest
}

// namespacePropertiesUpdate computes the updateProperties request that turns the
// namespace's current properties (as returned by the API, location included) into the
// desired user-managed properties. location is the desired location, or nil to keep
// whatever the server has: it is never removed, since that would leave the namespace
// without a storage location. ok is false when nothing has to change.
func namespacePropertiesUpdate(current, desired map[string]string, location *string) (req updateNamespacePropertiesPayload, ok bool) {
	updates := map[string]string{}
	for k, v := range desired {
		if k == namespaceLocationProperty {
			continue // managed through location
		}
		if cur, found := current[k]; !found || cur != v {
			updates[k] = v
		}
	}
	if location != nil && current[namespaceLocationProperty] != *location {
		updates[namespaceLocationProperty] = *location
	}

	var removals []string
	for k := range current {
		if k == namespaceLocationProperty {
			continue
		}
		if _, keep := desired[k]; !keep {
			removals = append(removals, k)
		}
	}
	sort.Strings(removals)

	if len(updates) == 0 && len(removals) == 0 {
		return updateNamespacePropertiesPayload{}, false
	}
	req.Removals = removals
	if len(updates) > 0 {
		req.Updates = updates
	}
	return req, true
}

func createNamespace(ctx context.Context, client *polarismanagement.APIClient, catalog string, p createNamespacePayload) (*namespacePayload, error) {
	var out namespacePayload
	if err := doCatalogJSON(ctx, client, http.MethodPost, icebergCatalogPath(catalog, "namespaces"), p, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// getNamespace returns the namespace; use isNotFoundError on the error to detect a missing one.
func getNamespace(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string) (*namespacePayload, error) {
	var out namespacePayload
	if err := doCatalogJSON(ctx, client, http.MethodGet, icebergCatalogPath(catalog, "namespaces", namespacePath(namespace)), nil, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// updateNamespaceProperties applies req. Polaris 1.9.0 rejects a request carrying both
// removals and updates with HTTP 409 (CommitConflictException, "Concurrent modification
// of namespace"), while each alone succeeds, so they are sent as two requests: updates
// first, then removals.
func updateNamespaceProperties(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, req updateNamespacePropertiesPayload) error {
	path := icebergCatalogPath(catalog, "namespaces", namespacePath(namespace), "properties")
	if len(req.Updates) > 0 {
		if err := doCatalogJSON(ctx, client, http.MethodPost, path, updateNamespacePropertiesPayload{Updates: req.Updates}, http.StatusOK, nil); err != nil {
			return err
		}
	}
	if len(req.Removals) > 0 {
		return doCatalogJSON(ctx, client, http.MethodPost, path, updateNamespacePropertiesPayload{Removals: req.Removals}, http.StatusOK, nil)
	}
	return nil
}

func dropNamespace(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string) error {
	return doCatalogJSON(ctx, client, http.MethodDelete, icebergCatalogPath(catalog, "namespaces", namespacePath(namespace)), nil, http.StatusNoContent, nil)
}

// listNamespaces returns the children of parent (the top-level namespaces when parent is
// empty), following next-page-token until the last page.
func listNamespaces(ctx context.Context, client *polarismanagement.APIClient, catalog string, parent []string) ([][]string, error) {
	var all [][]string
	err := paginate(func(token string) (*string, error) {
		q := url.Values{}
		if len(parent) > 0 {
			q.Set("parent", strings.Join(parent, "\x1f"))
		}
		if token != "" {
			q.Set("pageToken", token)
		}
		var page listNamespacesPayload
		if err := doCatalogJSON(ctx, client, http.MethodGet, withQuery(icebergCatalogPath(catalog, "namespaces"), q), nil, http.StatusOK, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Namespaces...)
		return page.NextPageToken, nil
	})
	return all, err
}

// sameLocation reports whether two locations are equal, ignoring the trailing slash
// Polaris adds to every namespace location.
func sameLocation(a, b string) bool {
	return strings.TrimSuffix(a, "/") == strings.TrimSuffix(b, "/")
}

// parseNamespaceImportID splits "{catalog}/{ns1}[/{ns2}...]" into the catalog and the
// namespace levels.
func parseNamespaceImportID(id string) (catalog string, namespace []string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) < 2 {
		return "", nil, fmt.Errorf(`expected "{catalog}/{ns1}[/{ns2}...]", got %q`, id)
	}
	for _, p := range parts {
		if p == "" {
			return "", nil, fmt.Errorf(`expected "{catalog}/{ns1}[/{ns2}...]" without empty segments, got %q`, id)
		}
	}
	return parts[0], parts[1:], nil
}
