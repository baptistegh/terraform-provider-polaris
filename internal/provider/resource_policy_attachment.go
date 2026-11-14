package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &policyAttachmentResource{}
	_ resource.ResourceWithImportState    = &policyAttachmentResource{}
	_ resource.ResourceWithValidateConfig = &policyAttachmentResource{}
)

type policyAttachmentResource struct {
	client *polarismanagement.APIClient
}

type policyAttachmentResourceModel struct {
	Catalog         types.String `tfsdk:"catalog"`
	PolicyNamespace types.List   `tfsdk:"policy_namespace"`
	PolicyName      types.String `tfsdk:"policy_name"`
	TargetType      types.String `tfsdk:"target_type"`
	TargetPath      types.List   `tfsdk:"target_path"`
	Parameters      types.Map    `tfsdk:"parameters"`
}

func newPolicyAttachmentResource() resource.Resource {
	return &policyAttachmentResource{}
}

func (r *policyAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_attachment"
}

func (r *policyAttachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Attaches a Polaris policy to its catalog, a namespace, or a table or view. " +
			"Polaris allows one policy of each type per target. Every attribute forces a new resource when changed.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the catalog holding the policy and the target.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"policy_namespace": schema.ListAttribute{
				Required:      true,
				ElementType:   types.StringType,
				Description:   "Namespace the policy is defined in, as an ordered list of levels.",
				PlanModifiers: []planmodifier.List{listRequiresReplace{}},
			},
			"policy_name": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the policy.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"target_type": schema.StringAttribute{
				Required:      true,
				Description:   "What the policy is attached to: `catalog`, `namespace`, or `table-like` (a table or view).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"target_path": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Path of the target: omit it for `catalog`, the namespace levels for `namespace`, " +
					"the namespace levels followed by the table or view name for `table-like`.",
				PlanModifiers: []planmodifier.List{listRequiresReplace{}},
			},
			"parameters": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Attachment parameters. Polaris does not return them, so they are not checked for drift and not imported.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *policyAttachmentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config policyAttachmentResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.TargetType.IsUnknown() || config.TargetPath.IsUnknown() {
		return
	}
	if err := validatePolicyTarget(config.TargetType.ValueString(), len(config.TargetPath.Elements())); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("target_path"), "Invalid policy attachment target", err.Error())
	}
}

// validatePolicyTarget checks the number of target_path levels for a target type.
func validatePolicyTarget(targetType string, levels int) error {
	switch targetType {
	case policyTargetCatalog:
		if levels != 0 {
			return fmt.Errorf("target_path must be omitted for a %q target", targetType)
		}
	case policyTargetNamespace:
		if levels < 1 {
			return fmt.Errorf("target_path must hold the namespace levels for a %q target", targetType)
		}
	case policyTargetTable:
		if levels < 2 {
			return fmt.Errorf("target_path must hold the namespace levels and the table or view name for a %q target", targetType)
		}
	default:
		return fmt.Errorf("target_type must be %q, %q or %q, got %q", policyTargetCatalog, policyTargetNamespace, policyTargetTable, targetType)
	}
	return nil
}

func (r *policyAttachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// parts returns the policy namespace and the target of the model.
func (m policyAttachmentResourceModel) parts(ctx context.Context, r *diag.Diagnostics) ([]string, policyTarget) {
	ns := listToStringSlice(ctx, m.PolicyNamespace, r)
	target := policyTarget{Type: m.TargetType.ValueString()}
	if !m.TargetPath.IsNull() && !m.TargetPath.IsUnknown() {
		target.Path = listToStringSlice(ctx, m.TargetPath, r)
	}
	return ns, target
}

func (r *policyAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan policyAttachmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns, target := plan.parts(ctx, &resp.Diagnostics)
	params := optStringMap(ctx, plan.Parameters, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := attachPolicy(ctx, r.client, plan.Catalog.ValueString(), ns, plan.PolicyName.ValueString(), attachPolicyPayload{Target: target, Parameters: params}); err != nil {
		resp.Diagnostics.AddError("Failed to attach policy", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *policyAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state policyAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns, target := state.parts(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	policies, err := applicablePolicies(ctx, r.client, state.Catalog.ValueString(), target)
	if isNotFoundError(err) { // the target (or catalog) is gone, and the attachment with it
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the policies applicable to the target", err.Error())
		return
	}
	if !isAttached(policies, ns, state.PolicyName.ValueString()) {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *policyAttachmentResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// Every attribute requires replacement; Update is never called.
}

func (r *policyAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state policyAttachmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns, target := state.parts(ctx, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := detachPolicy(ctx, r.client, state.Catalog.ValueString(), ns, state.PolicyName.ValueString(), target); err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to detach policy", err.Error())
	}
}

// ImportState expects "{catalog}/{policy namespace}/{policy name}/{target type}[/{target path}]",
// with namespace and target path levels joined by "." (so levels containing "." cannot be
// imported), e.g. "lakehouse/governance/compact/table-like/sales.orders".
func (r *policyAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	catalog, ns, name, target, err := parsePolicyAttachmentImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	nsList, diags := types.ListValueFrom(ctx, types.StringType, ns)
	resp.Diagnostics.Append(diags...)
	targetPath := types.ListNull(types.StringType)
	if len(target.Path) > 0 {
		targetPath, diags = types.ListValueFrom(ctx, types.StringType, target.Path)
		resp.Diagnostics.Append(diags...)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), catalog)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("policy_namespace"), nsList)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("policy_name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("target_type"), target.Type)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("target_path"), targetPath)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("parameters"), types.MapNull(types.StringType))...)
}

func parsePolicyAttachmentImportID(id string) (catalog string, namespace []string, name string, target policyTarget, err error) {
	const format = `"{catalog}/{policy namespace}/{policy name}/{target type}[/{target path}]", levels joined by "."`
	parts := strings.Split(id, "/")
	if len(parts) != 4 && len(parts) != 5 {
		return "", nil, "", policyTarget{}, fmt.Errorf("expected %s, got %q", format, id)
	}
	if slices.Contains(parts, "") {
		return "", nil, "", policyTarget{}, fmt.Errorf("expected %s without empty segments, got %q", format, id)
	}
	target = policyTarget{Type: parts[3]}
	if len(parts) == 5 {
		target.Path = strings.Split(parts[4], ".")
	}
	if err := validatePolicyTarget(target.Type, len(target.Path)); err != nil {
		return "", nil, "", policyTarget{}, err
	}
	return parts[0], strings.Split(parts[1], "."), parts[2], target, nil
}
