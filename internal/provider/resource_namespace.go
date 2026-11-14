package provider

import (
	"context"
	"fmt"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &namespaceResource{}
	_ resource.ResourceWithImportState = &namespaceResource{}
)

type namespaceResource struct {
	client *polarismanagement.APIClient
}

type namespaceResourceModel struct {
	Catalog    types.String `tfsdk:"catalog"`
	Namespace  types.List   `tfsdk:"namespace"`
	Properties types.Map    `tfsdk:"properties"`
	Location   types.String `tfsdk:"location"`
}

func newNamespaceResource() resource.Resource {
	return &namespaceResource{}
}

func (r *namespaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (r *namespaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a namespace in a Polaris catalog. Nested namespaces need their parent to exist: reference the parent resource. A namespace can only be destroyed once it is empty.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the catalog. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"namespace": schema.ListAttribute{
				Required:      true,
				ElementType:   types.StringType,
				Description:   "Namespace path as an ordered list of levels, e.g. `[\"sales\", \"eu\"]`. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.List{listRequiresReplace{}},
			},
			"properties": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Namespace properties. The storage location is managed with `location` and must not be set here.",
			},
			"location": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Storage location of the namespace. Polaris sets it to `<catalog default_base_location>/<namespace path>/` when unset. " +
					"Setting another value requires the `ALLOW_NAMESPACE_CUSTOM_LOCATION` feature on the server. Removing it from the configuration keeps the current location.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *namespaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *namespaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan namespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, plan.Namespace, &resp.Diagnostics)
	props := optStringMap(ctx, plan.Properties, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	delete(props, namespaceLocationProperty)
	if loc := optString(plan.Location); loc != nil {
		if props == nil {
			props = map[string]string{}
		}
		props[namespaceLocationProperty] = *loc
	}

	created, err := createNamespace(ctx, r.client, plan.Catalog.ValueString(), createNamespacePayload{Namespace: ns, Properties: props})
	if err != nil {
		resp.Diagnostics.AddError("Failed to create namespace", err.Error())
		return
	}

	state := namespaceFromAPI(ctx, plan.Catalog.ValueString(), created, plan, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}

func (r *namespaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state namespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := getNamespace(ctx, r.client, state.Catalog.ValueString(), ns)
	if isNotFoundError(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace", err.Error())
		return
	}

	next := namespaceFromAPI(ctx, state.Catalog.ValueString(), got, state, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
	}
}

func (r *namespaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan namespaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	catalog := plan.Catalog.ValueString()
	ns := listToStringSlice(ctx, plan.Namespace, &resp.Diagnostics)
	desired := optStringMap(ctx, plan.Properties, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Diff against what the server has now, not the prior state: updateProperties
	// applies removals and updates to the current properties.
	current, err := getNamespace(ctx, r.client, catalog, ns)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace", err.Error())
		return
	}
	location := optString(plan.Location)
	if location != nil && sameLocation(*location, current.Properties[namespaceLocationProperty]) {
		location = nil
	}
	if update, ok := namespacePropertiesUpdate(current.Properties, desired, location); ok {
		if err := updateNamespaceProperties(ctx, r.client, catalog, ns, update); err != nil {
			resp.Diagnostics.AddError("Failed to update namespace properties", err.Error())
			return
		}
		if current, err = getNamespace(ctx, r.client, catalog, ns); err != nil {
			resp.Diagnostics.AddError("Failed to read namespace after update", err.Error())
			return
		}
	}

	next := namespaceFromAPI(ctx, catalog, current, plan, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
	}
}

func (r *namespaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state namespaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := dropNamespace(ctx, r.client, state.Catalog.ValueString(), ns); err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to delete namespace", err.Error())
	}
}

// ImportState expects the ID in the form "{catalog}/{ns1}[/{ns2}...]".
func (r *namespaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	catalog, ns, err := parseNamespaceImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	nsList, diags := types.ListValueFrom(ctx, types.StringType, ns)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("catalog"), catalog)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), nsList)...)
}

// namespaceFromAPI builds the state from an API namespace. prior (plan or state) decides
// whether "no properties" is stored as null or as an empty map, and keeps a configured
// location as written when the server only added its trailing slash.
func namespaceFromAPI(ctx context.Context, catalog string, n *namespacePayload, prior namespaceResourceModel, ds *diag.Diagnostics) namespaceResourceModel {
	location, rest := splitNamespaceProperties(n.Properties)

	props := types.MapNull(types.StringType)
	if len(rest) > 0 || (!prior.Properties.IsNull() && !prior.Properties.IsUnknown()) {
		m, d := types.MapValueFrom(ctx, types.StringType, rest)
		ds.Append(d...)
		props = m
	}

	loc := types.StringPointerValue(location)
	if location != nil && !prior.Location.IsNull() && !prior.Location.IsUnknown() && sameLocation(prior.Location.ValueString(), *location) {
		loc = prior.Location
	}

	nsList, d := types.ListValueFrom(ctx, types.StringType, n.Namespace)
	ds.Append(d...)
	return namespaceResourceModel{
		Catalog:    types.StringValue(catalog),
		Namespace:  nsList,
		Properties: props,
		Location:   loc,
	}
}
