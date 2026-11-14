package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &catalogRoleDataSource{}

type catalogRoleDataSource struct {
	client *polarismanagement.APIClient
}

type catalogRoleDataSourceModel struct {
	Catalog             types.String `tfsdk:"catalog"`
	Name                types.String `tfsdk:"name"`
	Properties          types.Map    `tfsdk:"properties"`
	CreateTimestamp     types.Int64  `tfsdk:"create_timestamp"`
	LastUpdateTimestamp types.Int64  `tfsdk:"last_update_timestamp"`
	EntityVersion       types.Int64  `tfsdk:"entity_version"`
}

func newCatalogRoleDataSource() datasource.DataSource {
	return &catalogRoleDataSource{}
}

func (d *catalogRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog_role"
}

func (d *catalogRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := catalogRoleAttributes()
	attrs["catalog"] = schema.StringAttribute{Required: true, Description: "The name of the catalog the role belongs to."}
	attrs["name"] = schema.StringAttribute{Required: true, Description: "The name of the catalog role."}
	resp.Schema = schema.Schema{
		Description: "Fetches a Polaris catalog role by catalog and name.",
		Attributes:  attrs,
	}
}

func (d *catalogRoleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	pd, ok := req.ProviderData.(*ProviderData)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("expected *ProviderData, got %T", req.ProviderData))
		return
	}
	d.client = pd.Client
}

func (d *catalogRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config catalogRoleDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	catalog, name := config.Catalog.ValueString(), config.Name.ValueString()
	role, httpResp, err := d.client.DefaultAPI.GetCatalogRole(ctx, catalog, name).Execute()
	if isNotFound(httpResp) {
		resp.Diagnostics.AddError("Catalog role not found", fmt.Sprintf("No catalog role %q exists in catalog %q.", name, catalog))
		return
	}
	if err := checkResponse(httpResp, err, http.StatusOK); err != nil {
		resp.Diagnostics.AddError("Failed to get catalog role", err.Error())
		return
	}

	m := catalogRoleData(ctx, *role, &resp.Diagnostics)
	state := catalogRoleDataSourceModel{
		Catalog:             config.Catalog,
		Name:                m.Name,
		Properties:          m.Properties,
		CreateTimestamp:     m.CreateTimestamp,
		LastUpdateTimestamp: m.LastUpdateTimestamp,
		EntityVersion:       m.EntityVersion,
	}
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}
