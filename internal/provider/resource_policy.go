package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &policyResource{}
	_ resource.ResourceWithImportState = &policyResource{}
)

type policyResource struct {
	client *polarismanagement.APIClient
}

type policyResourceModel struct {
	Catalog     types.String `tfsdk:"catalog"`
	Namespace   types.List   `tfsdk:"namespace"`
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	Description types.String `tfsdk:"description"`
	Content     types.String `tfsdk:"content"`
	Inheritable types.Bool   `tfsdk:"inheritable"`
	Version     types.Int64  `tfsdk:"version"`
}

func newPolicyResource() resource.Resource {
	return &policyResource{}
}

func (r *policyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (r *policyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Polaris policy, defined in a namespace and applied to catalogs, namespaces or tables with `polaris_policy_attachment`. A policy can only be destroyed once it is no longer attached.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the catalog. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"namespace": schema.ListAttribute{
				Required:      true,
				ElementType:   types.StringType,
				Description:   "Namespace the policy is defined in, as an ordered list of levels. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.List{listRequiresReplace{}},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the policy: letters, digits, `-` and `_`. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				Required: true,
				Description: "The policy type: `system.data-compaction`, `system.metadata-compaction`, `system.orphan-file-removal` or `system.snapshot-expiry`. " +
					"Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the policy.",
			},
			"content": schema.StringAttribute{
				Required: true,
				Description: "The policy definition, a JSON document validated against the schema of `type` (use `jsonencode`). " +
					"Polaris stores it as written.",
			},
			"inheritable": schema.BoolAttribute{
				Computed:      true,
				Description:   "Whether the policy applies to the children of the entities it is attached to. Set by Polaris from the policy type.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"version": schema.Int64Attribute{
				Computed:    true,
				Description: "Version of the policy, incremented on every update.",
			},
		},
	}
}

func (r *policyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *policyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, plan.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := createPolicy(ctx, r.client, plan.Catalog.ValueString(), ns, createPolicyPayload{
		Name:        plan.Name.ValueString(),
		Type:        plan.Type.ValueString(),
		Description: optString(plan.Description),
		Content:     plan.Content.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create policy", err.Error())
		return
	}

	state := policyFromAPI(plan, *created)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}

func (r *policyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state policyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := getPolicy(ctx, r.client, state.Catalog.ValueString(), ns, state.Name.ValueString())
	if isNotFoundError(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to get policy", err.Error())
		return
	}

	next := policyFromAPI(state, *got)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
	}
}

func (r *policyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state policyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, plan.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// Omitting the description clears it, like an unset attribute.
	updated, err := updatePolicy(ctx, r.client, plan.Catalog.ValueString(), ns, plan.Name.ValueString(), updatePolicyPayload{
		CurrentPolicyVersion: state.Version.ValueInt64(),
		Description:          optString(plan.Description),
		Content:              plan.Content.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update policy", err.Error())
		return
	}

	next := policyFromAPI(plan, *updated)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
	}
}

func (r *policyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := dropPolicy(ctx, r.client, state.Catalog.ValueString(), ns, state.Name.ValueString()); err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to delete policy", err.Error())
	}
}

// ImportState expects the ID in the form "{catalog}/{ns1}[/{ns2}...]/{name}".
func (r *policyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	catalog, ns, name, err := parseNamespacedImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	nsList, diags := types.ListValueFrom(ctx, types.StringType, ns)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), catalog)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), nsList)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}

// policyFromAPI builds the state from an API policy; catalog and namespace, which the
// API does not return, come from prior (plan or state).
func policyFromAPI(prior policyResourceModel, p policyPayload) policyResourceModel {
	nsList := prior.Namespace
	if nsList.IsNull() || nsList.IsUnknown() {
		nsList = types.ListNull(types.StringType)
	}
	return policyResourceModel{
		Catalog:     prior.Catalog,
		Namespace:   nsList,
		Name:        types.StringValue(p.Name),
		Type:        types.StringValue(p.PolicyType),
		Description: types.StringPointerValue(p.Description),
		Content:     types.StringValue(p.Content),
		Inheritable: types.BoolValue(p.Inheritable),
		Version:     types.Int64Value(p.Version),
	}
}

// parseNamespacedImportID splits "{catalog}/{ns1}[/{ns2}...]/{name}" into the catalog,
// the namespace levels and the object name.
func parseNamespacedImportID(id string) (catalog string, namespace []string, name string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) < 3 {
		return "", nil, "", fmt.Errorf(`expected "{catalog}/{ns1}[/{ns2}...]/{name}", got %q`, id)
	}
	if slices.Contains(parts, "") {
		return "", nil, "", fmt.Errorf(`expected "{catalog}/{ns1}[/{ns2}...]/{name}" without empty segments, got %q`, id)
	}
	return parts[0], parts[1 : len(parts)-1], parts[len(parts)-1], nil
}
