package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	_ resource.Resource                = &catalogRoleResource{}
	_ resource.ResourceWithImportState = &catalogRoleResource{}
)

type catalogRoleResource struct {
	client *polarismanagement.Client
}

type catalogRoleResourceModel struct {
	Catalog             types.String `tfsdk:"catalog"`
	Name                types.String `tfsdk:"name"`
	Properties          types.Map    `tfsdk:"properties"`
	CreateTimestamp     types.Int64  `tfsdk:"create_timestamp"`
	LastUpdateTimestamp types.Int64  `tfsdk:"last_update_timestamp"`
	EntityVersion       types.Int64  `tfsdk:"entity_version"`
}

func newCatalogRoleResource() resource.Resource {
	return &catalogRoleResource{}
}

func (r *catalogRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog_role"
}

func (r *catalogRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Polaris catalog role.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog this role belongs to. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog role. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"properties": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Key-value metadata attached to the catalog role.",
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
				Description: "Version of the catalog role object, used for optimistic concurrency control.",
			},
		},
	}
}

func (r *catalogRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *catalogRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan catalogRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role := polarismanagement.CatalogRole{Name: plan.Name.ValueString()}
	if !plan.Properties.IsNull() && !plan.Properties.IsUnknown() {
		props := make(map[string]string)
		resp.Diagnostics.Append(plan.Properties.ElementsAs(ctx, &props, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		role.Properties = &props
	}

	httpResp, err := r.client.CreateCatalogRole(ctx, plan.Catalog.ValueString(), polarismanagement.CreateCatalogRoleJSONRequestBody{CatalogRole: &role})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create catalog role", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusCreated {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("POST /catalog-roles returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var created polarismanagement.CatalogRole
	if err := json.NewDecoder(httpResp.Body).Decode(&created); err != nil {
		resp.Diagnostics.AddError("Failed to decode create response", err.Error())
		return
	}

	state, ds := catalogRoleFromAPI(ctx, &created, plan.Catalog.ValueString(), plan.Properties)
	resp.Diagnostics.Append(ds...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}

func (r *catalogRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state catalogRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.GetCatalogRole(ctx, state.Catalog.ValueString(), state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get catalog role", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("GET /catalog-roles returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var role polarismanagement.CatalogRole
	if err := json.NewDecoder(httpResp.Body).Decode(&role); err != nil {
		resp.Diagnostics.AddError("Failed to decode catalog role", err.Error())
		return
	}

	next, ds := catalogRoleFromAPI(ctx, &role, state.Catalog.ValueString(), state.Properties)
	resp.Diagnostics.Append(ds...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
	}
}

func (r *catalogRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state catalogRoleResourceModel
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

	httpResp, err := r.client.UpdateCatalogRole(ctx, state.Catalog.ValueString(), state.Name.ValueString(), polarismanagement.UpdateCatalogRoleJSONRequestBody{
		CurrentEntityVersion: int(state.EntityVersion.ValueInt64()),
		Properties:           props,
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update catalog role", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("PUT /catalog-roles returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var updated polarismanagement.CatalogRole
	if err := json.NewDecoder(httpResp.Body).Decode(&updated); err != nil {
		resp.Diagnostics.AddError("Failed to decode update response", err.Error())
		return
	}

	next, ds := catalogRoleFromAPI(ctx, &updated, state.Catalog.ValueString(), plan.Properties)
	resp.Diagnostics.Append(ds...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
	}
}

func (r *catalogRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state catalogRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.DeleteCatalogRole(ctx, state.Catalog.ValueString(), state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete catalog role", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusNoContent {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("DELETE /catalog-roles returned HTTP %d.", httpResp.StatusCode))
		return
	}
}

// ImportState expects the ID in the form "{catalog}/{name}".
func (r *catalogRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", `Expected format: "{catalog}/{name}"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), parts[1])...)
}

func catalogRoleFromAPI(ctx context.Context, r *polarismanagement.CatalogRole, catalog string, prevProperties types.Map) (catalogRoleResourceModel, diag.Diagnostics) {
	var ds diag.Diagnostics
	m := catalogRoleResourceModel{
		Catalog: types.StringValue(catalog),
		Name:    types.StringValue(r.Name),
	}

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
