package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &principalRoleResource{}
	_ resource.ResourceWithImportState = &principalRoleResource{}
)

type principalRoleResource struct {
	client *polarismanagement.Client
}

type principalRoleResourceModel struct {
	Name                types.String `tfsdk:"name"`
	Properties          types.Map    `tfsdk:"properties"`
	CreateTimestamp     types.Int64  `tfsdk:"create_timestamp"`
	LastUpdateTimestamp types.Int64  `tfsdk:"last_update_timestamp"`
	EntityVersion       types.Int64  `tfsdk:"entity_version"`
}

func newPrincipalRoleResource() resource.Resource {
	return &principalRoleResource{}
}

func (r *principalRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_principal_role"
}

func (r *principalRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Polaris principal role.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the principal role. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"properties": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Key-value metadata attached to the principal role.",
			},
			"create_timestamp": schema.Int64Attribute{
				Computed:    true,
				Description: "Creation time as a Unix epoch timestamp in milliseconds.",
			},
			"last_update_timestamp": schema.Int64Attribute{
				Computed:    true,
				Description: "Last update time as a Unix epoch timestamp in milliseconds.",
			},
			"entity_version": schema.Int64Attribute{
				Computed:    true,
				Description: "Version of the principal role object, used for optimistic concurrency control.",
			},
		},
	}
}

func (r *principalRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *principalRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan principalRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := polarismanagement.PrincipalRole{Name: plan.Name.ValueString()}
	if !plan.Properties.IsNull() && !plan.Properties.IsUnknown() {
		props := make(map[string]string)
		resp.Diagnostics.Append(plan.Properties.ElementsAs(ctx, &props, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		role.Properties = &props
	}

	httpResp, err := r.client.CreatePrincipalRole(ctx, polarismanagement.CreatePrincipalRoleJSONRequestBody{PrincipalRole: &role})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create principal role", err.Error())
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusCreated {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("POST /principal-roles returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var created polarismanagement.PrincipalRole
	if err := json.NewDecoder(httpResp.Body).Decode(&created); err != nil {
		resp.Diagnostics.AddError("Failed to decode create response", err.Error())
		return
	}

	state, ds := principalRoleFromAPI(ctx, &created, plan.Properties)
	resp.Diagnostics.Append(ds...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}

func (r *principalRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state principalRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.GetPrincipalRole(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get principal role", err.Error())
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("GET /principal-roles returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var role polarismanagement.PrincipalRole
	if err := json.NewDecoder(httpResp.Body).Decode(&role); err != nil {
		resp.Diagnostics.AddError("Failed to decode principal role", err.Error())
		return
	}

	next, ds := principalRoleFromAPI(ctx, &role, state.Properties)
	resp.Diagnostics.Append(ds...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
	}
}

func (r *principalRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state principalRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	props := make(map[string]string)
	if !plan.Properties.IsNull() && !plan.Properties.IsUnknown() {
		resp.Diagnostics.Append(plan.Properties.ElementsAs(ctx, &props, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	httpResp, err := r.client.UpdatePrincipalRole(ctx, state.Name.ValueString(), polarismanagement.UpdatePrincipalRoleJSONRequestBody{
		CurrentEntityVersion: int(state.EntityVersion.ValueInt64()),
		Properties:           props,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update principal role", err.Error())
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("PUT /principal-roles returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var updated polarismanagement.PrincipalRole
	if err := json.NewDecoder(httpResp.Body).Decode(&updated); err != nil {
		resp.Diagnostics.AddError("Failed to decode update response", err.Error())
		return
	}

	next, ds := principalRoleFromAPI(ctx, &updated, plan.Properties)
	resp.Diagnostics.Append(ds...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
	}
}

func (r *principalRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state principalRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.DeletePrincipalRole(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete principal role", err.Error())
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNoContent {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("DELETE /principal-roles returned HTTP %d.", httpResp.StatusCode))
		return
	}
}

func (r *principalRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

func principalRoleFromAPI(ctx context.Context, r *polarismanagement.PrincipalRole, prevProperties types.Map) (principalRoleResourceModel, diag.Diagnostics) {
	var ds diag.Diagnostics
	m := principalRoleResourceModel{Name: types.StringValue(r.Name)}

	if r.Properties != nil {
		props, d := types.MapValueFrom(ctx, types.StringType, *r.Properties)
		ds.Append(d...)
		m.Properties = props
	} else if prevProperties.IsNull() {
		m.Properties = types.MapNull(types.StringType)
	} else {
		m.Properties = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	if r.CreateTimestamp != nil {
		m.CreateTimestamp = types.Int64Value(*r.CreateTimestamp)
	}
	if r.LastUpdateTimestamp != nil {
		m.LastUpdateTimestamp = types.Int64Value(*r.LastUpdateTimestamp)
	}
	if r.EntityVersion != nil {
		m.EntityVersion = types.Int64Value(int64(*r.EntityVersion))
	}

	return m, ds
}
