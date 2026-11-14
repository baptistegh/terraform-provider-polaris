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

var _ datasource.DataSource = &catalogsDataSource{}

type catalogsDataSource struct {
	client *polarismanagement.APIClient
}

type catalogsDataSourceModel struct {
	Names    types.List `tfsdk:"names"`
	Catalogs types.List `tfsdk:"catalogs"`
}

func newCatalogsDataSource() datasource.DataSource {
	return &catalogsDataSource{}
}

func (d *catalogsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalogs"
}

func (d *catalogsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the catalogs of the realm.",
		Attributes: map[string]schema.Attribute{
			"names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Names of all catalogs, in the order returned by Polaris.",
			},
			"catalogs": schema.ListNestedAttribute{
				Computed:    true,
				Description: "All catalogs, with the same attributes as the `polaris_catalog` data source.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: catalogDataSourceAttributes(),
				},
			},
		},
	}
}

func (d *catalogsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *catalogsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var list struct {
		Catalogs []catalogPayload `json:"catalogs"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, "/catalogs", nil, http.StatusOK, &list); err != nil {
		resp.Diagnostics.AddError("Failed to list catalogs", err.Error())
		return
	}

	names := make([]string, 0, len(list.Catalogs))
	catalogs := make([]catalogResourceModel, 0, len(list.Catalogs))
	for i := range list.Catalogs {
		names = append(names, list.Catalogs[i].Name)
		catalogs = append(catalogs, catalogDataFromAPI(ctx, &list.Catalogs[i], &resp.Diagnostics))
	}

	var state catalogsDataSourceModel
	state.Names, state.Catalogs = namesAndList(ctx, catalogDataSourceAttributes(), names, catalogs, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}
