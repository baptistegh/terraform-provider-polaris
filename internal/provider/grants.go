package provider

import (
	"context"
	"net/http"
	"net/url"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// listRequiresReplace is a List plan modifier that forces replacement when the value changes.
type listRequiresReplace struct{}

func (listRequiresReplace) Description(_ context.Context) string {
	return "If the value changes, Terraform will destroy and recreate the resource."
}

func (listRequiresReplace) MarkdownDescription(ctx context.Context) string {
	return listRequiresReplace{}.Description(ctx)
}

func (listRequiresReplace) PlanModifyList(_ context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.StateValue.IsNull() || req.PlanValue.Equal(req.StateValue) {
		return
	}
	resp.RequiresReplace = true
}

// listToStringSlice converts a types.List of strings into a []string.
func listToStringSlice(ctx context.Context, l types.List, ds *diag.Diagnostics) []string {
	var result []string
	ds.Append(l.ElementsAs(ctx, &result, false)...)
	return result
}

// grantPayload is the full JSON shape of any grant returned by the list endpoint.
// The generated GrantResource only carries Type, so we unmarshal raw responses with this.
type grantPayload struct {
	Type       string   `json:"type"`
	Privilege  string   `json:"privilege"`
	Namespace  []string `json:"namespace,omitempty"`
	TableName  string   `json:"tableName,omitempty"`
	ViewName   string   `json:"viewName,omitempty"`
	PolicyName string   `json:"policyName,omitempty"`
	// SemanticModelName is set for semantic-model grants.
	SemanticModelName string `json:"semanticModelName,omitempty"`
}

type grantsListResponse struct {
	Grants []grantPayload `json:"grants"`
}

// listGrants fetches all grants for a catalog role and returns the raw payload list.
// The grant endpoints are called through doJSON rather than the generated client:
// grants are polymorphic (allOf + discriminator) and the generated GrantResource model
// only carries the "type" field, which would drop privilege/namespace/table/... fields.

func grantsPath(catalog, catalogRole string) string {
	return "/catalogs/" + url.PathEscape(catalog) + "/catalog-roles/" + url.PathEscape(catalogRole) + "/grants"
}

func listGrants(ctx context.Context, client *polarismanagement.APIClient, catalog, catalogRole string) ([]grantPayload, error) {
	var resp grantsListResponse
	if err := doJSON(ctx, client, http.MethodGet, grantsPath(catalog, catalogRole), nil, http.StatusOK, &resp); err != nil {
		return nil, err
	}
	return resp.Grants, nil
}

func addGrant(ctx context.Context, client *polarismanagement.APIClient, catalog, catalogRole string, grant any) error {
	return doJSON(ctx, client, http.MethodPut, grantsPath(catalog, catalogRole), map[string]any{"grant": grant}, http.StatusCreated, nil)
}

func revokeGrant(ctx context.Context, client *polarismanagement.APIClient, catalog, catalogRole string, grant any) error {
	return doJSON(ctx, client, http.MethodPost, grantsPath(catalog, catalogRole), map[string]any{"grant": grant}, http.StatusCreated, nil)
}

func namespaceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
