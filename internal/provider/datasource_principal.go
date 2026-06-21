package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &principalDataSource{}

type principalDataSource struct {
	pd *ProviderData
}

type principalDataSourceModel struct {
	Name                types.String `tfsdk:"name"`
	ClientID            types.String `tfsdk:"client_id"`
	Properties          types.Map    `tfsdk:"properties"`
	CreateTimestamp     types.Int64  `tfsdk:"create_timestamp"`
	LastUpdateTimestamp types.Int64  `tfsdk:"last_update_timestamp"`
	EntityVersion       types.Int64  `tfsdk:"entity_version"`
}

func newPrincipalDataSource() datasource.DataSource {
	return &principalDataSource{}
}

func (d *principalDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_principal"
}

func (d *principalDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a Polaris principal by name.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The name of the principal. Defaults to the client_id configured in the provider.",
			},
			"client_id": schema.StringAttribute{
				Computed:    true,
				Description: "The OAuth client ID assigned to this principal by Polaris.",
			},
			"properties": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Key-value metadata attached to the principal.",
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

func (d *principalDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.pd = pd
}

func (d *principalDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state principalDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := state.Name.ValueString()
	if name == "" {
		name = d.pd.ClientID
	}

	httpResp, err := d.pd.Client.GetPrincipal(ctx, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to get principal", err.Error())
		return
	}
	defer func() { _ = httpResp.Body.Close() }()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.Diagnostics.AddError("Principal not found", fmt.Sprintf("No principal with name %q exists.", name))
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Unexpected API response", fmt.Sprintf("GET principal returned HTTP %d.", httpResp.StatusCode))
		return
	}

	var principal polarismanagement.Principal
	if err := json.NewDecoder(httpResp.Body).Decode(&principal); err != nil {
		resp.Diagnostics.AddError("Failed to decode principal", err.Error())
		return
	}

	if principal.Name != "" {
		state.Name = types.StringValue(principal.Name)
	} else {
		state.Name = types.StringValue(name)
	}
	state.ClientID = types.StringPointerValue(principal.ClientId)

	if principal.Properties != nil {
		props, diags := types.MapValueFrom(ctx, types.StringType, *principal.Properties)
		resp.Diagnostics.Append(diags...)
		state.Properties = props
	} else {
		state.Properties = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	if principal.CreateTimestamp != nil {
		state.CreateTimestamp = types.Int64Value(*principal.CreateTimestamp)
	}
	if principal.LastUpdateTimestamp != nil {
		state.LastUpdateTimestamp = types.Int64Value(*principal.LastUpdateTimestamp)
	}
	if principal.EntityVersion != nil {
		state.EntityVersion = types.Int64Value(int64(*principal.EntityVersion))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
