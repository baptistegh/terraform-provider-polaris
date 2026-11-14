package provider

import (
	"context"
	"fmt"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &catalogResource{}
	_ resource.ResourceWithImportState = &catalogResource{}
)

type catalogResource struct {
	client *polarismanagement.APIClient
}

type catalogResourceModel struct {
	Name                  types.String `tfsdk:"name"`
	Type                  types.String `tfsdk:"type"`
	DefaultBaseLocation   types.String `tfsdk:"default_base_location"`
	Properties            types.Map    `tfsdk:"properties"`
	StorageConfig         types.Object `tfsdk:"storage_config"`
	ConnectionConfig      types.Object `tfsdk:"connection_config"`
	ServiceIdentityIamArn types.String `tfsdk:"service_identity_iam_arn"`
	CreateTimestamp       types.Int64  `tfsdk:"create_timestamp"`
	LastUpdateTimestamp   types.Int64  `tfsdk:"last_update_timestamp"`
	EntityVersion         types.Int64  `tfsdk:"entity_version"`
}

func newCatalogResource() resource.Resource {
	return &catalogResource{}
}

func (r *catalogResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog"
}

// storageString is an optional storage setting the server may fill in or normalize.
func storageString(description string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Computed: true, Description: description}
}

func storageBool(description string) schema.BoolAttribute {
	return schema.BoolAttribute{Optional: true, Computed: true, Description: description}
}

func storageStrings(description string) schema.ListAttribute {
	return schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, Description: description}
}

func (r *catalogResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Polaris catalog: an INTERNAL catalog managed by Polaris, or an EXTERNAL one, optionally federated to a remote catalog service through `connection_config`.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "The name of the catalog. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"type": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("INTERNAL"),
				Description:   "The catalog type: `INTERNAL` (default) or `EXTERNAL`. Changing this forces a new resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"default_base_location": schema.StringAttribute{
				Required:    true,
				Description: "Default base location for the catalog's tables, e.g. `s3://bucket/prefix`. It must be within one of the storage `allowed_locations`.",
			},
			"properties": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Additional catalog properties. `default-base-location` is managed with `default_base_location` and must not be set here.",
			},
			"storage_config": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Storage configuration. Set the attributes for the chosen `storage_type`; settings the server fills in are read back after apply.",
				Attributes: map[string]schema.Attribute{
					"storage_type": schema.StringAttribute{
						Required:      true,
						Description:   "Storage type: `S3`, `GCS`, `AZURE` or `FILE` (testing only). Polaris cannot change it in place, so changing it forces a new resource.",
						PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
					},
					"allowed_locations": schema.SetAttribute{
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
						Description: "Locations the catalog may use. Polaris always adds `default_base_location`.",
					},
					"storage_name":                 storageString("Name of a server-side storage configuration to use."),
					"role_arn":                     storageString("S3: ARN of the IAM role granting access to the buckets."),
					"external_id":                  storageString("S3: external ID used in the role's trust policy."),
					"user_arn":                     storageString("S3: ARN of the IAM user Polaris uses to assume `role_arn`."),
					"encryption_keys":              storageStrings("S3: KMS key ARNs that can encrypt (and decrypt) data."),
					"decryption_keys":              storageStrings("S3: KMS key ARNs that can only decrypt data."),
					"region":                       storageString("S3: region where the data is stored."),
					"endpoint":                     storageString("S3: endpoint for S3 requests, as seen by clients."),
					"sts_endpoint":                 storageString("S3: endpoint for STS requests made by Polaris."),
					"sts_unavailable":              storageBool("S3: if true, Polaris does not use STS, which disables credential vending."),
					"endpoint_internal":            storageString("S3: endpoint Polaris uses for S3 requests instead of `endpoint`."),
					"path_style_access":            storageBool("S3: use path-style bucket addressing."),
					"kms_unavailable":              storageBool("S3: if true, Polaris does not add KMS key policies."),
					"credential_vending_mechanism": storageString("S3: how Polaris vends subscoped credentials, e.g. `STS`. Empty uses the server default."),
					"tenant_id":                    storageString("AZURE: tenant ID the storage accounts belong to. Required for AZURE."),
					"multi_tenant_app_name":        storageString("AZURE: name of the Azure client application."),
					"consent_url":                  storageString("AZURE: URL of the Azure permissions request page."),
					"hierarchical":                 storageBool("AZURE: scope SAS tokens to the most specific path. Only for storage accounts with hierarchical namespace."),
					"gcs_service_account":          storageString("GCS: Google Cloud Storage service account."),
				},
			},
			"connection_config": schema.SingleNestedAttribute{
				Optional: true,
				Description: "EXTERNAL catalogs only: connection to the remote catalog service to federate. " +
					"Requires the `ENABLE_CATALOG_FEDERATION` feature on the server. " +
					"The management API cannot update a connection, so any change, including a new secret, forces a new resource.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.RequiresReplace()},
				Attributes: map[string]schema.Attribute{
					"connection_type": schema.StringAttribute{
						Required:    true,
						Description: "Remote catalog type: `ICEBERG_REST`, `HADOOP`, `HIVE` or `BIGQUERY`.",
					},
					"uri": schema.StringAttribute{
						Optional:    true,
						Description: "URI of the remote catalog service.",
					},
					"remote_catalog_name": schema.StringAttribute{
						Optional:    true,
						Description: "ICEBERG_REST: name of the catalog in the remote service (often called the warehouse).",
					},
					"warehouse": schema.StringAttribute{
						Optional:    true,
						Description: "HADOOP / HIVE: warehouse location.",
					},
					"properties": schema.MapAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Additional connection properties (for BIGQUERY: `gcp.bigquery.project-id` and `warehouse`).",
					},
					"authentication": schema.SingleNestedAttribute{
						Required:    true,
						Description: "How Polaris authenticates to the remote service.",
						Attributes: map[string]schema.Attribute{
							"type": schema.StringAttribute{
								Required:    true,
								Description: "Authentication type: `OAUTH`, `BEARER`, `SIGV4`, `IMPLICIT` or `GCP`.",
							},
							"token_uri":         schema.StringAttribute{Optional: true, Description: "OAUTH: token endpoint URI."},
							"client_id":         schema.StringAttribute{Optional: true, Description: "OAUTH: client ID."},
							"client_secret":     schema.StringAttribute{Optional: true, Sensitive: true, Description: "OAUTH: client secret. Never returned by the API, so not available after import."},
							"scopes":            schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "OAUTH: scopes requested with the token."},
							"bearer_token":      schema.StringAttribute{Optional: true, Sensitive: true, Description: "BEARER: token. Never returned by the API, so not available after import."},
							"role_arn":          schema.StringAttribute{Optional: true, Description: "SIGV4: ARN of the IAM role Polaris assumes to sign requests."},
							"role_session_name": schema.StringAttribute{Optional: true, Description: "SIGV4: role session name."},
							"external_id":       schema.StringAttribute{Optional: true, Description: "SIGV4: external ID used in the role's trust policy."},
							"signing_region":    schema.StringAttribute{Optional: true, Description: "SIGV4: region used to sign requests."},
							"signing_name":      schema.StringAttribute{Optional: true, Description: "SIGV4: service name used to sign requests (server default `execute-api`)."},
							"session_policy":    schema.StringAttribute{Optional: true, Description: "SIGV4: IAM session policy (JSON) restricting the assumed role."},
						},
					},
				},
			},
			"service_identity_iam_arn": schema.StringAttribute{
				Computed:      true,
				Description:   "For federated catalogs, the ARN of the IAM identity Polaris uses to reach the remote service, if the server reports one.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"create_timestamp": schema.Int64Attribute{
				Computed:      true,
				Description:   "Creation time as a Unix epoch timestamp in milliseconds.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"last_update_timestamp": schema.Int64Attribute{
				Computed:    true,
				Description: "Last update time as a Unix epoch timestamp in milliseconds.",
			},
			"entity_version": schema.Int64Attribute{
				Computed:    true,
				Description: "Version of the catalog object, used for optimistic concurrency control.",
			},
		},
	}
}

func (r *catalogResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *catalogResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan catalogResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := catalogPayload{
		Type:                 plan.Type.ValueString(),
		Name:                 plan.Name.ValueString(),
		Properties:           catalogProperties(ctx, plan.DefaultBaseLocation, plan.Properties, &resp.Diagnostics),
		StorageConfigInfo:    storageToPayload(ctx, plan.StorageConfig, &resp.Diagnostics),
		ConnectionConfigInfo: connectionToPayload(ctx, plan.ConnectionConfig, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := createCatalog(ctx, r.client, payload); err != nil {
		resp.Diagnostics.AddError("Failed to create catalog", err.Error())
		return
	}
	// The create response reports lastUpdateTimestamp as 0; read the stored catalog back.
	created, err := getCatalog(ctx, r.client, payload.Name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read catalog after create", err.Error())
		return
	}

	state := catalogFromAPI(ctx, created, plan, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}

func (r *catalogResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state catalogResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	catalog, err := getCatalog(ctx, r.client, state.Name.ValueString())
	if isNotFoundError(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to get catalog", err.Error())
		return
	}

	next := catalogFromAPI(ctx, catalog, state, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
	}
}

func (r *catalogResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state catalogResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API replaces the properties and the storage configuration as a whole,
	// so send everything that is configured.
	update := updateCatalogPayload{
		CurrentEntityVersion: state.EntityVersion.ValueInt64(),
		Properties:           catalogProperties(ctx, plan.DefaultBaseLocation, plan.Properties, &resp.Diagnostics),
		StorageConfigInfo:    storageToPayload(ctx, plan.StorageConfig, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	updated, err := updateCatalog(ctx, r.client, state.Name.ValueString(), update)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update catalog", err.Error())
		return
	}

	next := catalogFromAPI(ctx, updated, plan, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, next)...)
	}
}

func (r *catalogResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state catalogResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := deleteCatalog(ctx, r.client, state.Name.ValueString()); err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to delete catalog", err.Error())
	}
}

func (r *catalogResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...)
}

// catalogFromAPI builds the state from an API catalog. prior (plan or state) supplies
// the values the API does not return: connection secrets, and whether empty maps and
// lists were configured.
func catalogFromAPI(ctx context.Context, c *catalogPayload, prior catalogResourceModel, ds *diag.Diagnostics) catalogResourceModel {
	base, props := splitCatalogProperties(ctx, c.Properties, prior.Properties, ds)
	return catalogResourceModel{
		Name:                  types.StringValue(c.Name),
		Type:                  types.StringValue(c.Type),
		DefaultBaseLocation:   base,
		Properties:            props,
		StorageConfig:         storageFromPayload(ctx, c.StorageConfigInfo, ds),
		ConnectionConfig:      connectionFromPayload(ctx, c.ConnectionConfigInfo, prior.ConnectionConfig, ds),
		ServiceIdentityIamArn: serviceIdentityFromPayload(c.ConnectionConfigInfo),
		CreateTimestamp:       int64Value(c.CreateTimestamp),
		LastUpdateTimestamp:   int64Value(c.LastUpdateTimestamp),
		EntityVersion:         int64Value(c.EntityVersion),
	}
}
