package provider

import (
	"context"
	"fmt"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &catalogDataSource{}

type catalogDataSource struct {
	client *polarismanagement.APIClient
}

func newCatalogDataSource() datasource.DataSource {
	return &catalogDataSource{}
}

// catalogDataSourceAttributes returns the read-only catalog attributes, derived from
// the polaris_catalog resource schema. Used by polaris_catalog and polaris_catalogs.
func catalogDataSourceAttributes() map[string]schema.Attribute {
	var resp resource.SchemaResponse
	(&catalogResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	return computedAttributes(resp.Schema.Attributes)
}

func (d *catalogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog"
}

func (d *catalogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := catalogDataSourceAttributes()
	attrs["name"] = schema.StringAttribute{
		Required:    true,
		Description: "The name of the catalog.",
	}
	resp.Schema = schema.Schema{
		Description: "Fetches a Polaris catalog by name. Connection secrets (`client_secret`, `bearer_token`) are never returned by the API and are always null.",
		Attributes:  attrs,
	}
}

func (d *catalogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *catalogDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config catalogResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := config.Name.ValueString()
	catalog, err := getCatalog(ctx, d.client, name)
	if isNotFoundError(err) {
		resp.Diagnostics.AddError("Catalog not found", fmt.Sprintf("No catalog with name %q exists.", name))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to get catalog", err.Error())
		return
	}

	state := catalogDataFromAPI(ctx, catalog, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}

// catalogDataFromAPI converts an API catalog for the data sources: properties are an
// empty map rather than null when the catalog has none.
func catalogDataFromAPI(ctx context.Context, c *catalogPayload, ds *diag.Diagnostics) catalogResourceModel {
	return catalogFromAPI(ctx, c, catalogResourceModel{
		Properties:       types.MapValueMust(types.StringType, map[string]attr.Value{}),
		ConnectionConfig: types.ObjectNull(catalogConnectionAttrTypes),
	}, ds)
}
