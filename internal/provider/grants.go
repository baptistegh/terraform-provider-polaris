package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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
}

type grantsListResponse struct {
	Grants []grantPayload `json:"grants"`
}

// listGrants fetches all grants for a catalog role and returns the raw payload list.
func listGrants(ctx context.Context, client *polarismanagement.Client, catalog, catalogRole string) ([]grantPayload, error) {
	httpResp, err := client.ListGrantsForCatalogRole(ctx, catalog, catalogRole)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /grants returned HTTP %d", httpResp.StatusCode)
	}

	var resp grantsListResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode grants list: %w", err)
	}
	return resp.Grants, nil
}

// addGrant sends a PUT request to add a grant to a catalog role.
func addGrant(ctx context.Context, client *polarismanagement.Client, catalog, catalogRole string, grant any) error {
	body, err := json.Marshal(map[string]any{"grant": grant})
	if err != nil {
		return fmt.Errorf("marshal grant: %w", err)
	}
	httpResp, err := client.AddGrantToCatalogRoleWithBody(ctx, catalog, catalogRole, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		var errBody map[string]any
		_ = json.NewDecoder(httpResp.Body).Decode(&errBody)
		return fmt.Errorf("PUT /grants returned HTTP %d: %v", httpResp.StatusCode, errBody)
	}
	return nil
}

// revokeGrant sends a POST request to revoke a grant from a catalog role.
// The revoke endpoint returns 201 on success (per the Polaris management spec).
func revokeGrant(ctx context.Context, client *polarismanagement.Client, catalog, catalogRole string, grant any) error {
	body, err := json.Marshal(map[string]any{"grant": grant})
	if err != nil {
		return fmt.Errorf("marshal grant: %w", err)
	}
	httpResp, err := client.RevokeGrantFromCatalogRoleWithBody(ctx, catalog, catalogRole, nil, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		var errBody map[string]any
		_ = json.NewDecoder(httpResp.Body).Decode(&errBody)
		return fmt.Errorf("POST /grants (revoke) returned HTTP %d: %v", httpResp.StatusCode, errBody)
	}
	return nil
}

// namespaceEqual compares two namespace slices for equality.
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
