package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &realmDataSource{}

type realmDataSource struct {
	realm string
}

type realmDataSourceModel struct {
	Realm types.String `tfsdk:"realm"`
}

func newRealmDataSource() datasource.DataSource {
	return &realmDataSource{}
}

func (d *realmDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_realm"
}

func (d *realmDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Returns the realm configured in the provider. Useful for passing the realm value to other resources or outputs without repeating the configuration.",
		Attributes: map[string]schema.Attribute{
			"realm": schema.StringAttribute{
				Computed:    true,
				Description: "The realm configured in the provider block (or POLARIS_REALM). Empty string if no realm was set.",
			},
		},
	}
}

func (d *realmDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.realm = pd.Realm
}

func (d *realmDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	resp.Diagnostics.Append(resp.State.Set(ctx, &realmDataSourceModel{
		Realm: types.StringValue(d.realm),
	})...)
}
