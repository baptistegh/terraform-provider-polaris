package provider

import (
	"context"
	"fmt"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &semanticModelResource{}
	_ resource.ResourceWithImportState = &semanticModelResource{}
)

type semanticModelResource struct {
	client *polarismanagement.APIClient
}

type semanticModelResourceModel struct {
	Catalog       types.String `tfsdk:"catalog"`
	Namespace     types.List   `tfsdk:"namespace"`
	Name          types.String `tfsdk:"name"`
	SpecVersion   types.String `tfsdk:"spec_version"`
	Document      types.String `tfsdk:"document"`
	EntityVersion types.String `tfsdk:"entity_version"`
}

func newSemanticModelResource() resource.Resource {
	return &semanticModelResource{}
}

func (r *semanticModelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_semantic_model"
}

func (r *semanticModelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Polaris semantic model: an [Apache Ossie](https://github.com/apache/ossie) document stored in a namespace. " +
			"Requires the `ENABLE_SEMANTIC_MODELS` feature on the server.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the catalog. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"namespace": schema.ListAttribute{
				Required:      true,
				ElementType:   types.StringType,
				Description:   "Namespace the semantic model is stored in, as an ordered list of levels. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.List{listRequiresReplace{}},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the semantic model: letters, digits, `-` and `_`. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"spec_version": schema.StringAttribute{
				Required:    true,
				Description: "The Apache Ossie spec version the document follows, e.g. `0.2.0.dev0`.",
			},
			"document": schema.StringAttribute{
				Required: true,
				Description: "The Ossie semantic model, a JSON object (use `jsonencode`). It must have a `datasets` array, and each dataset " +
					"`source` must name an existing table or view of the catalog as `<namespace>.<table>`. Polaris stores it as written.",
			},
			"entity_version": schema.StringAttribute{
				Computed:    true,
				Description: "Opaque version of the semantic model, changed by every update and used for optimistic concurrency.",
			},
		},
	}
}

func (r *semanticModelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *semanticModelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan semanticModelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, plan.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	created, err := createSemanticModel(ctx, r.client, plan.Catalog.ValueString(), ns, createSemanticModelPayload{
		Name:     plan.Name.ValueString(),
		Document: semanticModelDocument{Version: plan.SpecVersion.ValueString(), SemanticModel: plan.Document.ValueString()},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create semantic model", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, semanticModelFromAPI(plan, *created))...)
}

func (r *semanticModelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state semanticModelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := getSemanticModel(ctx, r.client, state.Catalog.ValueString(), ns, state.Name.ValueString())
	if isNotFoundError(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to get semantic model", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, semanticModelFromAPI(state, *got))...)
}

func (r *semanticModelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state semanticModelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, plan.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	updated, err := updateSemanticModel(ctx, r.client, plan.Catalog.ValueString(), ns, plan.Name.ValueString(), updateSemanticModelPayload{
		EntityVersion: state.EntityVersion.ValueString(),
		Document:      semanticModelDocument{Version: plan.SpecVersion.ValueString(), SemanticModel: plan.Document.ValueString()},
	})
	if err != nil {
		resp.Diagnostics.AddError("Failed to update semantic model", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, semanticModelFromAPI(plan, *updated))...)
}

func (r *semanticModelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state semanticModelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := dropSemanticModel(ctx, r.client, state.Catalog.ValueString(), ns, state.Name.ValueString()); err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to delete semantic model", err.Error())
	}
}

// ImportState expects the ID in the form "{catalog}/{ns1}[/{ns2}...]/{name}".
func (r *semanticModelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// semanticModelFromAPI builds the state from an API semantic model; catalog, namespace and
// name, which the API does not return, come from prior (plan or state).
func semanticModelFromAPI(prior semanticModelResourceModel, m semanticModelPayload) semanticModelResourceModel {
	return semanticModelResourceModel{
		Catalog:       prior.Catalog,
		Namespace:     prior.Namespace,
		Name:          prior.Name,
		SpecVersion:   types.StringValue(m.Document.Version),
		Document:      types.StringValue(m.Document.SemanticModel),
		EntityVersion: types.StringValue(m.EntityVersion),
	}
}
