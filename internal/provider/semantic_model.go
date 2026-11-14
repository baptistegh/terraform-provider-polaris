package provider

import (
	"context"
	"net/http"
	"net/url"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
)

// Semantic models are Polaris-native catalog API objects under /polaris/v1/{catalog}
// (doCatalogJSON, polarisCatalogPath), available when ENABLE_SEMANTIC_MODELS is on.
// Shared by polaris_semantic_model, its data sources and the test fixtures.

// semanticModelDocument is an Apache Ossie document: the spec version and the semantic
// model, serialized as a JSON object string.
type semanticModelDocument struct {
	Version       string `json:"version"`
	SemanticModel string `json:"semantic_model"`
}

type createSemanticModelPayload struct {
	Name     string                `json:"name"`
	Document semanticModelDocument `json:"document"`
}

type updateSemanticModelPayload struct {
	EntityVersion string                `json:"entity-version"`
	Document      semanticModelDocument `json:"document"`
}

// semanticModelPayload is the response of create, load and update. EntityVersion is an
// opaque string used for optimistic concurrency.
type semanticModelPayload struct {
	Document      semanticModelDocument `json:"document"`
	EntityVersion string                `json:"entity-version"`
}

type listSemanticModelsPayload struct {
	Identifiers   []policyIdentifier `json:"identifiers"`
	NextPageToken *string            `json:"next-page-token,omitempty"`
}

func semanticModelsPath(catalog string, namespace []string, segments ...string) string {
	return polarisCatalogPath(catalog, append([]string{"namespaces", namespacePath(namespace), "semantic-models"}, segments...)...)
}

func semanticModelPath(catalog string, namespace []string, name string) string {
	return semanticModelsPath(catalog, namespace, url.PathEscape(name))
}

func createSemanticModel(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, p createSemanticModelPayload) (*semanticModelPayload, error) {
	var out semanticModelPayload
	if err := doCatalogJSON(ctx, client, http.MethodPost, semanticModelsPath(catalog, namespace), p, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// getSemanticModel returns the semantic model; use isNotFoundError on the error to detect
// a missing one.
func getSemanticModel(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, name string) (*semanticModelPayload, error) {
	var out semanticModelPayload
	if err := doCatalogJSON(ctx, client, http.MethodGet, semanticModelPath(catalog, namespace, name), nil, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func updateSemanticModel(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, name string, p updateSemanticModelPayload) (*semanticModelPayload, error) {
	var out semanticModelPayload
	if err := doCatalogJSON(ctx, client, http.MethodPut, semanticModelPath(catalog, namespace, name), p, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func dropSemanticModel(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string, name string) error {
	return doCatalogJSON(ctx, client, http.MethodDelete, semanticModelPath(catalog, namespace, name), nil, http.StatusNoContent, nil)
}

// listSemanticModels returns the names of the semantic models of namespace, following
// next-page-token until the last page.
func listSemanticModels(ctx context.Context, client *polarismanagement.APIClient, catalog string, namespace []string) ([]string, error) {
	var names []string
	err := paginate(func(token string) (*string, error) {
		q := url.Values{}
		if token != "" {
			q.Set("pageToken", token)
		}
		var page listSemanticModelsPayload
		if err := doCatalogJSON(ctx, client, http.MethodGet, withQuery(semanticModelsPath(catalog, namespace), q), nil, http.StatusOK, &page); err != nil {
			return nil, err
		}
		for _, id := range page.Identifiers {
			names = append(names, id.Name)
		}
		return page.NextPageToken, nil
	})
	return names, err
}
