package provider

import (
	"context"
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
	_ resource.Resource                = &principalRoleAssignmentResource{}
	_ resource.ResourceWithImportState = &principalRoleAssignmentResource{}
)

type principalRoleAssignmentResource struct {
	client *polarismanagement.APIClient
}

type principalRoleAssignmentResourceModel struct {
	Principal     types.String `tfsdk:"principal"`
	PrincipalRole types.String `tfsdk:"principal_role"`
}

func newPrincipalRoleAssignmentResource() resource.Resource {
	return &principalRoleAssignmentResource{}
}

func (r *principalRoleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_principal_role_assignment"
}

func (r *principalRoleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns a principal role to a Polaris principal.",
		Attributes: map[string]schema.Attribute{
			"principal": schema.StringAttribute{
				Required:    true,
				Description: "The name of the principal. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"principal_role": schema.StringAttribute{
				Required:    true,
				Description: "The name of the principal role to assign. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *principalRoleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *principalRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan principalRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := polarismanagement.GrantPrincipalRoleRequest{
		PrincipalRole: &polarismanagement.PrincipalRole{Name: plan.PrincipalRole.ValueString()},
	}
	httpResp, err := r.client.DefaultAPI.AssignPrincipalRole(ctx, plan.Principal.ValueString()).GrantPrincipalRoleRequest(body).Execute()
	if err := checkResponse(httpResp, err, http.StatusCreated); err != nil {
		resp.Diagnostics.AddError("Failed to assign principal role", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *principalRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state principalRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	roles, httpResp, err := r.client.DefaultAPI.ListPrincipalRolesAssigned(ctx, state.Principal.ValueString()).Execute()
	if isNotFound(httpResp) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err := checkResponse(httpResp, err, http.StatusOK); err != nil {
		resp.Diagnostics.AddError("Failed to list principal roles", err.Error())
		return
	}

	for _, role := range roles.Roles {
		if role.Name == state.PrincipalRole.ValueString() {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *principalRoleAssignmentResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes are RequiresReplace; Update is never called.
}

func (r *principalRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state principalRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.DefaultAPI.RevokePrincipalRole(ctx, state.Principal.ValueString(), state.PrincipalRole.ValueString()).Execute()
	if err := checkResponse(httpResp, err, http.StatusNoContent); err != nil {
		resp.Diagnostics.AddError("Failed to revoke principal role", err.Error())
		return
	}
}

// ImportState expects the ID in the form "{principal}/{principal_role}".
func (r *principalRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", `Expected format: "{principal}/{principal_role}"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("principal"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("principal_role"), parts[1])...)
}
