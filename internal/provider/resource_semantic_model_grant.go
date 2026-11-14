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
	_ resource.Resource                = &semanticModelGrantResource{}
	_ resource.ResourceWithImportState = &semanticModelGrantResource{}
)

type semanticModelGrantResource struct {
	client *polarismanagement.APIClient
}

type semanticModelGrantResourceModel struct {
	Catalog           types.String `tfsdk:"catalog"`
	CatalogRole       types.String `tfsdk:"catalog_role"`
	Namespace         types.List   `tfsdk:"namespace"`
	SemanticModelName types.String `tfsdk:"semantic_model_name"`
	Privilege         types.String `tfsdk:"privilege"`
}

func newSemanticModelGrantResource() resource.Resource {
	return &semanticModelGrantResource{}
}

func (r *semanticModelGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_semantic_model_grant"
}

func (r *semanticModelGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a semantic-model-level privilege to a Polaris catalog role. Requires the `ENABLE_SEMANTIC_MODELS` feature on the server, and the semantic model must exist.",
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
			"semantic_model_name": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the semantic model. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"privilege": schema.StringAttribute{
				Required:      true,
				Description:   "The semantic-model-level privilege to grant (e.g. SEMANTIC_MODEL_READ, SEMANTIC_MODEL_WRITE). Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *semanticModelGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *semanticModelGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan semanticModelGrantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, plan.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := map[string]any{
		"type":              "semantic-model",
		"namespace":         ns,
		"semanticModelName": plan.SemanticModelName.ValueString(),
		"privilege":         plan.Privilege.ValueString(),
	}
	if err := addGrant(ctx, r.client, plan.Catalog.ValueString(), plan.CatalogRole.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to add semantic model grant", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *semanticModelGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state semanticModelGrantResourceModel
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
		if g.Type == "semantic-model" && g.Privilege == state.Privilege.ValueString() &&
			g.SemanticModelName == state.SemanticModelName.ValueString() && namespaceEqual(g.Namespace, ns) {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *semanticModelGrantResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

func (r *semanticModelGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state semanticModelGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := map[string]any{
		"type":              "semantic-model",
		"namespace":         ns,
		"semanticModelName": state.SemanticModelName.ValueString(),
		"privilege":         state.Privilege.ValueString(),
	}
	if err := revokeGrant(ctx, r.client, state.Catalog.ValueString(), state.CatalogRole.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to revoke semantic model grant", err.Error())
	}
}

// ImportState expects the ID in the form "{catalog}/{catalog_role}/{privilege}/{ns1}[/{ns2}...]/{semantic_model_name}".
func (r *semanticModelGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) < 5 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import ID", `Expected format: "{catalog}/{catalog_role}/{privilege}/{ns1}[/{ns2}...]/{semantic_model_name}"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_role"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("privilege"), parts[2])...)
	ns := parts[3 : len(parts)-1]
	nsList, diags := types.ListValueFrom(ctx, types.StringType, ns)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), nsList)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("semantic_model_name"), parts[len(parts)-1])...)
}
