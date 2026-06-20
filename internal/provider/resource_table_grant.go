package provider

import (
	"context"
	"fmt"
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
	_ resource.Resource                = &tableGrantResource{}
	_ resource.ResourceWithImportState = &tableGrantResource{}
)

type tableGrantResource struct {
	client *polarismanagement.Client
}

type tableGrantResourceModel struct {
	Catalog     types.String `tfsdk:"catalog"`
	CatalogRole types.String `tfsdk:"catalog_role"`
	Namespace   types.List   `tfsdk:"namespace"`
	TableName   types.String `tfsdk:"table_name"`
	Privilege   types.String `tfsdk:"privilege"`
}

func newTableGrantResource() resource.Resource {
	return &tableGrantResource{}
}

func (r *tableGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_table_grant"
}

func (r *tableGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a table-level privilege to a Polaris catalog role.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"catalog_role": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog role receiving the grant. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"namespace": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Namespace path as an ordered list of components. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.List{listRequiresReplace{}},
			},
			"table_name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the table. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"privilege": schema.StringAttribute{
				Required:    true,
				Description: "The table-level privilege to grant (e.g. TABLE_READ_DATA, TABLE_WRITE_DATA). Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *tableGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tableGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tableGrantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, plan.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := map[string]any{
		"type":      "table",
		"namespace": ns,
		"tableName": plan.TableName.ValueString(),
		"privilege": plan.Privilege.ValueString(),
	}
	if err := addGrant(ctx, r.client, plan.Catalog.ValueString(), plan.CatalogRole.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to add table grant", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *tableGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tableGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	grants, err := listGrants(ctx, r.client, state.Catalog.ValueString(), state.CatalogRole.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to list grants", err.Error())
		return
	}

	for _, g := range grants {
		if g.Type == "table" && g.Privilege == state.Privilege.ValueString() &&
			g.TableName == state.TableName.ValueString() && namespaceEqual(g.Namespace, ns) {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *tableGrantResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {}

func (r *tableGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tableGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := map[string]any{
		"type":      "table",
		"namespace": ns,
		"tableName": state.TableName.ValueString(),
		"privilege": state.Privilege.ValueString(),
	}
	if err := revokeGrant(ctx, r.client, state.Catalog.ValueString(), state.CatalogRole.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to revoke table grant", err.Error())
	}
}

// ImportState expects the ID in the form "{catalog}/{catalog_role}/{privilege}/{ns1}[/{ns2}...]/{table_name}".
func (r *tableGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) < 5 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import ID", `Expected format: "{catalog}/{catalog_role}/{privilege}/{ns1}[/{ns2}...]/{table_name}"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_role"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("privilege"), parts[2])...)
	ns := parts[3 : len(parts)-1]
	nsList, diags := types.ListValueFrom(ctx, types.StringType, ns)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), nsList)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("table_name"), parts[len(parts)-1])...)
}
