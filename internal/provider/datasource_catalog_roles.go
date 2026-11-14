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

var _ datasource.DataSource = &catalogRolesDataSource{}

type catalogRolesDataSource struct {
	client *polarismanagement.APIClient
}

type catalogRolesDataSourceModel struct {
	Catalog types.String `tfsdk:"catalog"`
	Names   types.List   `tfsdk:"names"`
	Roles   types.List   `tfsdk:"roles"`
}

func newCatalogRolesDataSource() datasource.DataSource {
	return &catalogRolesDataSource{}
}

func (d *catalogRolesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_catalog_roles"
}

func (d *catalogRolesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the roles of a Polaris catalog, including the built-in `catalog_admin` role.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog.",
			},
			"names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Names of the catalog roles, in the order returned by Polaris.",
			},
			"roles": schema.ListNestedAttribute{
				Computed:     true,
				Description:  "The catalog roles.",
				NestedObject: schema.NestedAttributeObject{Attributes: catalogRoleAttributes()},
			},
		},
	}
}

func (d *catalogRolesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *catalogRolesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state catalogRolesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	list, httpResp, err := d.client.DefaultAPI.ListCatalogRoles(ctx, state.Catalog.ValueString()).Execute()
	if err := checkResponse(httpResp, err, http.StatusOK); err != nil {
		resp.Diagnostics.AddError("Failed to list catalog roles", err.Error())
		return
	}

	names := make([]string, 0, len(list.Roles))
	roles := make([]catalogRoleModel, 0, len(list.Roles))
	for _, r := range list.Roles {
		names = append(names, r.Name)
		roles = append(roles, catalogRoleData(ctx, r, &resp.Diagnostics))
	}
	state.Names, state.Roles = namesAndList(ctx, catalogRoleAttributes(), names, roles, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}
