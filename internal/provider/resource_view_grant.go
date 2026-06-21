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
	_ resource.Resource                = &viewGrantResource{}
	_ resource.ResourceWithImportState = &viewGrantResource{}
)

type viewGrantResource struct {
	client *polarismanagement.Client
}

type viewGrantResourceModel struct {
	Catalog     types.String `tfsdk:"catalog"`
	CatalogRole types.String `tfsdk:"catalog_role"`
	Namespace   types.List   `tfsdk:"namespace"`
	ViewName    types.String `tfsdk:"view_name"`
	Privilege   types.String `tfsdk:"privilege"`
}

func newViewGrantResource() resource.Resource {
	return &viewGrantResource{}
}

func (r *viewGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_view_grant"
}

func (r *viewGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a view-level privilege to a Polaris catalog role.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the catalog. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"catalog_role": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the catalog role receiving the grant. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"namespace": schema.ListAttribute{
				Required:      true,
				ElementType:   types.StringType,
				Description:   "Namespace path as an ordered list of components. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.List{listRequiresReplace{}},
			},
			"view_name": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the view. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"privilege": schema.StringAttribute{
				Required:      true,
				Description:   "The view-level privilege to grant (e.g. VIEW_READ_PROPERTIES, VIEW_FULL_METADATA). Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *viewGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *viewGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan viewGrantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, plan.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := map[string]any{
		"type":      "view",
		"namespace": ns,
		"viewName":  plan.ViewName.ValueString(),
		"privilege": plan.Privilege.ValueString(),
	}
	if err := addGrant(ctx, r.client, plan.Catalog.ValueString(), plan.CatalogRole.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to add view grant", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *viewGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state viewGrantResourceModel
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
		if g.Type == "view" && g.Privilege == state.Privilege.ValueString() &&
			g.ViewName == state.ViewName.ValueString() && namespaceEqual(g.Namespace, ns) {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *viewGrantResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

func (r *viewGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state viewGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := map[string]any{
		"type":      "view",
		"namespace": ns,
		"viewName":  state.ViewName.ValueString(),
		"privilege": state.Privilege.ValueString(),
	}
	if err := revokeGrant(ctx, r.client, state.Catalog.ValueString(), state.CatalogRole.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to revoke view grant", err.Error())
	}
}

// ImportState expects "{catalog}/{catalog_role}/{privilege}/{ns1}[/{ns2}...]/{view_name}".
func (r *viewGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) < 5 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import ID", `Expected format: "{catalog}/{catalog_role}/{privilege}/{ns1}[/{ns2}...]/{view_name}"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_role"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("privilege"), parts[2])...)
	ns := parts[3 : len(parts)-1]
	nsList, diags := types.ListValueFrom(ctx, types.StringType, ns)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), nsList)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("view_name"), parts[len(parts)-1])...)
}
