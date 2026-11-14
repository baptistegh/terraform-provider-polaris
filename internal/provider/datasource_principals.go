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

var _ datasource.DataSource = &principalsDataSource{}

type principalsDataSource struct {
	client *polarismanagement.APIClient
}

type principalsDataSourceModel struct {
	Names      types.List `tfsdk:"names"`
	Principals types.List `tfsdk:"principals"`
}

func newPrincipalsDataSource() datasource.DataSource {
	return &principalsDataSource{}
}

func (d *principalsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_principals"
}

func (d *principalsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the principals of the realm.",
		Attributes: map[string]schema.Attribute{
			"names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Names of the principals, in the order returned by Polaris.",
			},
			"principals": schema.ListNestedAttribute{
				Computed:     true,
				Description:  "The principals. Credentials are never included.",
				NestedObject: schema.NestedAttributeObject{Attributes: principalAttributes()},
			},
		},
	}
}

func (d *principalsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *principalsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, httpResp, err := d.client.DefaultAPI.ListPrincipals(ctx).Execute()
	if err := checkResponse(httpResp, err, http.StatusOK); err != nil {
		resp.Diagnostics.AddError("Failed to list principals", err.Error())
		return
	}

	names := make([]string, 0, len(list.Principals))
	principals := make([]principalModel, 0, len(list.Principals))
	for _, p := range list.Principals {
		names = append(names, p.Name)
		principals = append(principals, principalData(ctx, p, &resp.Diagnostics))
	}
	var state principalsDataSourceModel
	state.Names, state.Principals = namesAndList(ctx, principalAttributes(), names, principals, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}
