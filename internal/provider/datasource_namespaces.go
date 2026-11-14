package provider

import (
	"context"
	"fmt"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &namespacesDataSource{}

type namespacesDataSource struct {
	client *polarismanagement.APIClient
}

type namespacesDataSourceModel struct {
	Catalog    types.String `tfsdk:"catalog"`
	Parent     types.List   `tfsdk:"parent"`
	Namespaces types.List   `tfsdk:"namespaces"`
}

func newNamespacesDataSource() datasource.DataSource {
	return &namespacesDataSource{}
}

func (d *namespacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespaces"
}

func (d *namespacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the namespaces of a Polaris catalog: the top-level ones, or the direct children of `parent`.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog.",
			},
			"parent": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Namespace whose direct children are listed. Omit it to list the top-level namespaces.",
			},
			"namespaces": schema.ListAttribute{
				Computed:    true,
				ElementType: types.ListType{ElemType: types.StringType},
				Description: "The namespaces, each as an ordered list of levels (full path, including `parent`), in the order returned by Polaris.",
			},
		},
	}
}

func (d *namespacesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *namespacesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state namespacesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var parent []string
	if !state.Parent.IsNull() {
		parent = listToStringSlice(ctx, state.Parent, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}
	namespaces, err := listNamespaces(ctx, d.client, state.Catalog.ValueString(), parent)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list namespaces", err.Error())
		return
	}
	if namespaces == nil {
		namespaces = [][]string{}
	}

	list, diags := types.ListValueFrom(ctx, types.ListType{ElemType: types.StringType}, namespaces)
	resp.Diagnostics.Append(diags...)
	state.Namespaces = list
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}
