package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = &principalCredentialsResource{}
	_ resource.ResourceWithValidateConfig = &principalCredentialsResource{}
)

const (
	credentialsMethodReset  = "RESET"
	credentialsMethodRotate = "ROTATE"
)

type principalCredentialsResource struct {
	client *polarismanagement.APIClient
}

type principalCredentialsResourceModel struct {
	Principal        types.String `tfsdk:"principal"`
	Method           types.String `tfsdk:"method"`
	RotationTriggers types.Map    `tfsdk:"rotation_triggers"`
	ClientID         types.String `tfsdk:"client_id"`
	ClientSecret     types.String `tfsdk:"client_secret"`
}

// principalWithCredentials is the response of the rotate and reset endpoints.
type principalWithCredentials struct {
	Principal struct {
		Name     string  `json:"name"`
		ClientID *string `json:"clientId,omitempty"`
	} `json:"principal"`
	Credentials struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	} `json:"credentials"`
}

func newPrincipalCredentialsResource() resource.Resource {
	return &principalCredentialsResource{}
}

func (r *principalCredentialsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_principal_credentials"
}

func (r *principalCredentialsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Issues new credentials for a Polaris principal. New credentials are issued when the resource is created, " +
			"and again only when `principal`, `method`, `rotation_triggers` or `client_secret` change. " +
			"The secret is stored in state as a sensitive value. " +
			"Destroying the resource only removes it from state: the credentials stay valid. " +
			"With `RESET`, every previous secret stops working, including `polaris_principal.client_secret`.",
		Attributes: map[string]schema.Attribute{
			"principal": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the principal. Changing this issues new credentials.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"method": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(credentialsMethodReset),
				Description: "How the credentials are issued. `RESET` (default) calls resetCredentials: it needs admin rights on the principal " +
					"and revokes every previous secret. `ROTATE` calls rotateCredentials: Polaris only allows it for the principal itself, so the provider " +
					"must authenticate as that principal, and the previous secret keeps working until the next rotation, for zero-downtime rotation. " +
					"Changing this issues new credentials.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"rotation_triggers": schema.MapAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				Description:   "Arbitrary values that issue new credentials when they change, e.g. a date to rotate on a schedule, or the principal's `client_id` so new credentials follow a re-created principal.",
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
			"client_id": schema.StringAttribute{
				Computed:      true,
				Description:   "The OAuth client ID of the principal.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"client_secret": schema.StringAttribute{
				Optional:  true,
				Computed:  true,
				Sensitive: true,
				Description: "The new OAuth client secret. Set it to choose the secret yourself (`RESET` only; the server may reject secrets that do not meet its rules), " +
					"or leave it unset to let Polaris generate one. Changing it issues new credentials.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *principalCredentialsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config principalCredentialsResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config.Method.IsUnknown() || config.ClientSecret.IsUnknown() {
		return
	}
	method := config.Method.ValueString()
	if !config.Method.IsNull() && method != credentialsMethodReset && method != credentialsMethodRotate {
		resp.Diagnostics.AddAttributeError(path.Root("method"), "Invalid method", fmt.Sprintf("method must be %q or %q, got %q.", credentialsMethodReset, credentialsMethodRotate, method))
	}
	if method == credentialsMethodRotate && !config.ClientSecret.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("client_secret"), "client_secret requires RESET", "rotateCredentials always generates the secret; set method = \"RESET\" to choose it.")
	}
}

func (r *principalCredentialsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *principalCredentialsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan principalCredentialsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	principalPath := "/principals/" + url.PathEscape(plan.Principal.ValueString())
	var out principalWithCredentials
	var err error
	if plan.Method.ValueString() == credentialsMethodRotate {
		err = doJSON(ctx, r.client, http.MethodPost, principalPath+"/rotate", nil, http.StatusOK, &out)
	} else {
		body := map[string]string{}
		if !plan.ClientSecret.IsNull() && !plan.ClientSecret.IsUnknown() {
			body["clientSecret"] = plan.ClientSecret.ValueString()
		}
		err = doJSON(ctx, r.client, http.MethodPost, principalPath+"/reset", body, http.StatusOK, &out)
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to issue principal credentials", err.Error())
		return
	}

	plan.ClientID = types.StringValue(out.Credentials.ClientID)
	plan.ClientSecret = types.StringValue(out.Credentials.ClientSecret)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *principalCredentialsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state principalCredentialsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	principal, httpResp, err := r.client.DefaultAPI.GetPrincipal(ctx, state.Principal.ValueString()).Execute()
	if isNotFound(httpResp) {
		resp.State.RemoveResource(ctx)
		return
	}
	// A principal rotating its own credentials (ROTATE) is not allowed to read itself:
	// keep the state as is when the check is forbidden.
	if httpResp != nil && httpResp.StatusCode == http.StatusForbidden {
		return
	}
	if err := checkResponse(httpResp, err, http.StatusOK); err != nil {
		resp.Diagnostics.AddError("Failed to get principal", err.Error())
		return
	}
	// A principal re-created under the same name has a new client ID: these credentials are gone.
	if principal.ClientId != nil && *principal.ClientId != state.ClientID.ValueString() {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *principalCredentialsResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// Every configurable attribute requires replacement; Update is never called.
}

func (r *principalCredentialsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
	// Polaris cannot revoke credentials without issuing new ones, so destroying only
	// removes the resource from state.
}
