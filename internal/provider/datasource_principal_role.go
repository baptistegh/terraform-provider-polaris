package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var _ datasource.DataSource = &principalRoleDataSource{}

type principalRoleDataSource struct {
	client *polarismanagement.APIClient
}

func newPrincipalRoleDataSource() datasource.DataSource {
	return &principalRoleDataSource{}
}

func (d *principalRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_principal_role"
}

func (d *principalRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := principalRoleAttributes()
	attrs["name"] = schema.StringAttribute{Required: true, Description: "The name of the principal role."}
	resp.Schema = schema.Schema{
		Description: "Fetches a Polaris principal role by name.",
		Attributes:  attrs,
	}
}

func (d *principalRoleDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *principalRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config principalRoleModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := config.Name.ValueString()
	role, httpResp, err := d.client.DefaultAPI.GetPrincipalRole(ctx, name).Execute()
	if isNotFound(httpResp) {
		resp.Diagnostics.AddError("Principal role not found", fmt.Sprintf("No principal role with name %q exists.", name))
		return
	}
	if err := checkResponse(httpResp, err, http.StatusOK); err != nil {
		resp.Diagnostics.AddError("Failed to get principal role", err.Error())
		return
	}

	state := principalRoleData(ctx, *role, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}
