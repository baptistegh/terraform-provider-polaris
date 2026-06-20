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
	_ resource.Resource                = &catalogGrantResource{}
	_ resource.ResourceWithImportState = &catalogGrantResource{}
)

type catalogGrantResource struct {
	client *polarismanagement.Client
}

type catalogGrantResourceModel struct {
	Catalog     types.String `tfsdk:"catalog"`
	CatalogRole types.String `tfsdk:"catalog_role"`
	Privilege   types.String `tfsdk:"privilege"`
}

func newCatalogGrantResource() resource.Resource {
	return &catalogGrantResource{}
}

func (r *catalogGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog_grant"
}

func (r *catalogGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a catalog-level privilege to a Polaris catalog role.",
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
			"privilege": schema.StringAttribute{
				Required:    true,
				Description: "The catalog-level privilege to grant (e.g. CATALOG_MANAGE_CONTENT, CATALOG_READ_PROPERTIES). Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *catalogGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *catalogGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan catalogGrantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := map[string]string{"type": "catalog", "privilege": plan.Privilege.ValueString()}
	if err := addGrant(ctx, r.client, plan.Catalog.ValueString(), plan.CatalogRole.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to add catalog grant", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *catalogGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state catalogGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	grants, err := listGrants(ctx, r.client, state.Catalog.ValueString(), state.CatalogRole.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to list grants", err.Error())
		return
	}

	for _, g := range grants {
		if g.Type == "catalog" && g.Privilege == state.Privilege.ValueString() {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *catalogGrantResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes are RequiresReplace; Update is never called.
}

func (r *catalogGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state catalogGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := map[string]string{"type": "catalog", "privilege": state.Privilege.ValueString()}
	if err := revokeGrant(ctx, r.client, state.Catalog.ValueString(), state.CatalogRole.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to revoke catalog grant", err.Error())
	}
}

// ImportState expects the ID in the form "{catalog}/{catalog_role}/{privilege}".
func (r *catalogGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import ID", `Expected format: "{catalog}/{catalog_role}/{privilege}"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_role"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("privilege"), parts[2])...)
}
