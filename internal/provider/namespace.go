package provider

import "sort"

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
