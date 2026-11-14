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

var _ datasource.DataSource = &principalRolesDataSource{}

type principalRolesDataSource struct {
	client *polarismanagement.APIClient
}

type principalRolesDataSourceModel struct {
	Names types.List `tfsdk:"names"`
	Roles types.List `tfsdk:"roles"`
}

func newPrincipalRolesDataSource() datasource.DataSource {
	return &principalRolesDataSource{}
}

func (d *principalRolesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_principal_roles"
}

func (d *principalRolesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the principal roles of the realm, including the built-in `service_admin` role.",
		Attributes: map[string]schema.Attribute{
			"names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Names of the principal roles, in the order returned by Polaris.",
			},
			"roles": schema.ListNestedAttribute{
				Computed:     true,
				Description:  "The principal roles.",
				NestedObject: schema.NestedAttributeObject{Attributes: principalRoleAttributes()},
			},
		},
	}
}

func (d *principalRolesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *principalRolesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	list, httpResp, err := d.client.DefaultAPI.ListPrincipalRoles(ctx).Execute()
	if err := checkResponse(httpResp, err, http.StatusOK); err != nil {
		resp.Diagnostics.AddError("Failed to list principal roles", err.Error())
		return
	}

	names := make([]string, 0, len(list.Roles))
	roles := make([]principalRoleModel, 0, len(list.Roles))
	for _, r := range list.Roles {
		names = append(names, r.Name)
		roles = append(roles, principalRoleData(ctx, r, &resp.Diagnostics))
	}
	var state principalRolesDataSourceModel
	state.Names, state.Roles = namesAndList(ctx, principalRoleAttributes(), names, roles, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}
