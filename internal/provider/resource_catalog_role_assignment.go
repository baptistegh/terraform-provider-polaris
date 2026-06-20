package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &catalogRoleAssignmentResource{}
	_ resource.ResourceWithImportState = &catalogRoleAssignmentResource{}
)

type catalogRoleAssignmentResource struct {
	client *polarismanagement.Client
}

type catalogRoleAssignmentResourceModel struct {
	PrincipalRole types.String `tfsdk:"principal_role"`
	Catalog       types.String `tfsdk:"catalog"`
	CatalogRole   types.String `tfsdk:"catalog_role"`
}

func newCatalogRoleAssignmentResource() resource.Resource {
	return &catalogRoleAssignmentResource{}
}

func (r *catalogRoleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog_role_assignment"
}

func (r *catalogRoleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns a catalog role to a Polaris principal role.",
		Attributes: map[string]schema.Attribute{
			"principal_role": schema.StringAttribute{
				Required:    true,
				Description: "The name of the principal role. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"catalog": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog the catalog role belongs to. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"catalog_role": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog role to assign. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *catalogRoleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("expected *ProviderData, got %T", req.ProviderData))
		return
	}
	r.client = pd.Client
}

func (r *catalogRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan catalogRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := polarismanagement.AssignCatalogRoleToPrincipalRoleJSONRequestBody{
		CatalogRole: &polarismanagement.CatalogRole{Name: plan.CatalogRole.ValueString()},
	}
	httpResp, err := r.client.AssignCatalogRoleToPrincipalRole(ctx, plan.PrincipalRole.ValueString(), plan.Catalog.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to assign catalog role", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("POST /principal-roles/{name}/catalog-roles/{catalog} returned HTTP %d.", httpResp.StatusCode))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *catalogRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state catalogRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.ListCatalogRolesForPrincipalRole(ctx, state.PrincipalRole.ValueString(), state.Catalog.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to list catalog roles for principal role", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("GET /principal-roles/{name}/catalog-roles/{catalog} returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var roles polarismanagement.CatalogRoles
	if err := json.NewDecoder(httpResp.Body).Decode(&roles); err != nil {
		resp.Diagnostics.AddError("Failed to decode catalog roles list", err.Error())
		return
	}

	for _, role := range roles.Roles {
		if role.Name == state.CatalogRole.ValueString() {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *catalogRoleAssignmentResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes are RequiresReplace; Update is never called.
}

func (r *catalogRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state catalogRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.RevokeCatalogRoleFromPrincipalRole(ctx, state.PrincipalRole.ValueString(), state.Catalog.ValueString(), state.CatalogRole.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to revoke catalog role", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusNoContent {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("DELETE /principal-roles/{name}/catalog-roles/{catalog}/{role} returned HTTP %d.", httpResp.StatusCode))
		return
	}
}

// ImportState expects the ID in the form "{principal_role}/{catalog}/{catalog_role}".
func (r *catalogRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import ID", `Expected format: "{principal_role}/{catalog}/{catalog_role}"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("principal_role"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_role"), parts[2])...)
}
