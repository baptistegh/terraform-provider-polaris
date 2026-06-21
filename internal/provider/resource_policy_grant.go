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
	_ resource.Resource                = &policyGrantResource{}
	_ resource.ResourceWithImportState = &policyGrantResource{}
)

type policyGrantResource struct {
	client *polarismanagement.Client
}

type policyGrantResourceModel struct {
	Catalog     types.String `tfsdk:"catalog"`
	CatalogRole types.String `tfsdk:"catalog_role"`
	Namespace   types.List   `tfsdk:"namespace"`
	PolicyName  types.String `tfsdk:"policy_name"`
	Privilege   types.String `tfsdk:"privilege"`
}

func newPolicyGrantResource() resource.Resource {
	return &policyGrantResource{}
}

func (r *policyGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_grant"
}

func (r *policyGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a policy-level privilege to a Polaris catalog role.",
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
			"policy_name": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the policy. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"privilege": schema.StringAttribute{
				Required:      true,
				Description:   "The policy-level privilege to grant (e.g. POLICY_READ, POLICY_WRITE). Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *policyGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *policyGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyGrantResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, plan.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := map[string]any{
		"type":       "policy",
		"namespace":  ns,
		"policyName": plan.PolicyName.ValueString(),
		"privilege":  plan.Privilege.ValueString(),
	}
	if err := addGrant(ctx, r.client, plan.Catalog.ValueString(), plan.CatalogRole.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to add policy grant", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *policyGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state policyGrantResourceModel
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
		if g.Type == "policy" && g.Privilege == state.Privilege.ValueString() &&
			g.PolicyName == state.PolicyName.ValueString() && namespaceEqual(g.Namespace, ns) {
			resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
			return
		}
	}

	resp.State.RemoveResource(ctx)
}

func (r *policyGrantResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

func (r *policyGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyGrantResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	grant := map[string]any{
		"type":       "policy",
		"namespace":  ns,
		"policyName": state.PolicyName.ValueString(),
		"privilege":  state.Privilege.ValueString(),
	}
	if err := revokeGrant(ctx, r.client, state.Catalog.ValueString(), state.CatalogRole.ValueString(), grant); err != nil {
		resp.Diagnostics.AddError("Failed to revoke policy grant", err.Error())
	}
}

// ImportState expects "{catalog}/{catalog_role}/{privilege}/{ns1}[/{ns2}...]/{policy_name}".
func (r *policyGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) < 5 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		resp.Diagnostics.AddError("Invalid import ID", `Expected format: "{catalog}/{catalog_role}/{privilege}/{ns1}[/{ns2}...]/{policy_name}"`)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog_role"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("privilege"), parts[2])...)
	ns := parts[3 : len(parts)-1]
	nsList, diags := types.ListValueFrom(ctx, types.StringType, ns)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), nsList)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("policy_name"), parts[len(parts)-1])...)
}
