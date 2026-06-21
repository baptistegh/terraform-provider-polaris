package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &principalResource{}
	_ resource.ResourceWithImportState = &principalResource{}
)

type principalResource struct {
	client *polarismanagement.Client
}

type principalResourceModel struct {
	Name                       types.String `tfsdk:"name"`
	Properties                 types.Map    `tfsdk:"properties"`
	CredentialRotationRequired types.Bool   `tfsdk:"credential_rotation_required"`
	ClientID                   types.String `tfsdk:"client_id"`
	ClientSecret               types.String `tfsdk:"client_secret"`
	CreateTimestamp            types.Int64  `tfsdk:"create_timestamp"`
	LastUpdateTimestamp        types.Int64  `tfsdk:"last_update_timestamp"`
	EntityVersion              types.Int64  `tfsdk:"entity_version"`
}

func newPrincipalResource() resource.Resource {
	return &principalResource{}
}

func (r *principalResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_principal"
}

func (r *principalResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Polaris principal.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the principal. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"properties": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Key-value metadata attached to the principal.",
			},
			"credential_rotation_required": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "If true, the initial credentials can only be used to call rotateCredentials. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"client_id": schema.StringAttribute{
				Computed:    true,
				Description: "The OAuth client ID assigned to this principal by Polaris.",
			},
			"client_secret": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The OAuth client secret returned at creation time. Not available after import.",
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
				Description: "Version of the principal object, used for optimistic concurrency control.",
			},
		},
	}
}

func (r *principalResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("expected *ProviderData, got %T", req.ProviderData),
		)
		return
	}
	r.client = pd.Client
}

func (r *principalResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan principalResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	principal := polarismanagement.Principal{Name: plan.Name.ValueString()}
	if !plan.Properties.IsNull() && !plan.Properties.IsUnknown() {
		props := make(map[string]string)
		resp.Diagnostics.Append(plan.Properties.ElementsAs(ctx, &props, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		principal.Properties = &props
	}

	body := polarismanagement.CreatePrincipalJSONRequestBody{Principal: &principal}
	if !plan.CredentialRotationRequired.IsNull() && !plan.CredentialRotationRequired.IsUnknown() {
		v := plan.CredentialRotationRequired.ValueBool()
		body.CredentialRotationRequired = &v
	}

	httpResp, err := r.client.CreatePrincipal(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create principal", err.Error())
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusCreated {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("POST /principals returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var created polarismanagement.PrincipalWithCredentials
	if err := json.NewDecoder(httpResp.Body).Decode(&created); err != nil {
		resp.Diagnostics.AddError("Failed to decode create response", err.Error())
		return
	}

	state, ds := principalFromAPI(ctx, &created.Principal, plan.Properties)
	resp.Diagnostics.Append(ds...)
	if resp.Diagnostics.HasError() {
		return
	}

	// client_secret and client_id come from the credentials envelope, not the principal object.
	state.ClientID = types.StringPointerValue(created.Credentials.ClientId)
	state.ClientSecret = types.StringPointerValue(created.Credentials.ClientSecret)
	// credential_rotation_required is not returned by GET — default to false if not explicitly set.
	if !plan.CredentialRotationRequired.IsNull() && !plan.CredentialRotationRequired.IsUnknown() {
		state.CredentialRotationRequired = plan.CredentialRotationRequired
	} else {
		state.CredentialRotationRequired = types.BoolValue(false)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *principalResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state principalResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.GetPrincipal(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to get principal", err.Error())
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("GET /principals returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var principal polarismanagement.Principal
	if err := json.NewDecoder(httpResp.Body).Decode(&principal); err != nil {
		resp.Diagnostics.AddError("Failed to decode principal", err.Error())
		return
	}

	next, ds := principalFromAPI(ctx, &principal, state.Properties)
	resp.Diagnostics.Append(ds...)
	if resp.Diagnostics.HasError() {
		return
	}

	// GET never returns credentials — preserve them from prior state.
	next.ClientSecret = state.ClientSecret
	next.CredentialRotationRequired = state.CredentialRotationRequired

	resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
}

func (r *principalResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state principalResourceModel
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

	body := polarismanagement.UpdatePrincipalJSONRequestBody{
		CurrentEntityVersion: int(state.EntityVersion.ValueInt64()),
		Properties:           props,
	}

	httpResp, err := r.client.UpdatePrincipal(ctx, state.Name.ValueString(), body)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update principal", err.Error())
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("PUT /principals returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var updated polarismanagement.Principal
	if err := json.NewDecoder(httpResp.Body).Decode(&updated); err != nil {
		resp.Diagnostics.AddError("Failed to decode update response", err.Error())
		return
	}

	next, ds := principalFromAPI(ctx, &updated, plan.Properties)
	resp.Diagnostics.Append(ds...)
	if resp.Diagnostics.HasError() {
		return
	}

	next.ClientSecret = state.ClientSecret
	next.CredentialRotationRequired = state.CredentialRotationRequired

	resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
}

func (r *principalResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state principalResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	httpResp, err := r.client.DeletePrincipal(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete principal", err.Error())
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode != http.StatusNoContent {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("DELETE /principals returned HTTP %d.", httpResp.StatusCode))
		return
	}
}

func (r *principalResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
	// client_secret is not recoverable after import.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("client_secret"), types.StringValue(""))...)
}

// principalFromAPI maps a polarismanagement.Principal to principalResourceModel.
// prevProperties is passed so we can preserve null vs empty-map distinction when the API returns nil.
func principalFromAPI(ctx context.Context, p *polarismanagement.Principal, prevProperties types.Map) (principalResourceModel, diag.Diagnostics) {
	var ds diag.Diagnostics

	m := principalResourceModel{
		Name:     types.StringValue(p.Name),
		ClientID: types.StringPointerValue(p.ClientId),
	}

	if p.Properties != nil {
		props, d := types.MapValueFrom(ctx, types.StringType, *p.Properties)
		ds.Append(d...)
		m.Properties = props
	} else if prevProperties.IsNull() {
		m.Properties = types.MapNull(types.StringType)
	} else {
		m.Properties = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	if p.CreateTimestamp != nil {
		m.CreateTimestamp = types.Int64Value(*p.CreateTimestamp)
	}
	if p.LastUpdateTimestamp != nil {
		m.LastUpdateTimestamp = types.Int64Value(*p.LastUpdateTimestamp)
	}
	if p.EntityVersion != nil {
		m.EntityVersion = types.Int64Value(int64(*p.EntityVersion))
	}

	return m, ds
}
