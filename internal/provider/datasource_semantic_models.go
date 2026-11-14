package provider

import (
	"context"
	"fmt"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &semanticModelsDataSource{}

type semanticModelsDataSource struct {
	client *polarismanagement.APIClient
}

type semanticModelsDataSourceModel struct {
	Catalog   types.String `tfsdk:"catalog"`
	Namespace types.List   `tfsdk:"namespace"`
	Names     types.List   `tfsdk:"names"`
}

func newSemanticModelsDataSource() datasource.DataSource {
	return &semanticModelsDataSource{}
}

func (d *semanticModelsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_semantic_models"
}

func (d *semanticModelsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the semantic models of a namespace. Use `polaris_semantic_model` for the document of one. Requires the `ENABLE_SEMANTIC_MODELS` feature on the server.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog.",
			},
			"namespace": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Namespace the semantic models are stored in, as an ordered list of levels.",
			},
			"names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Names of the semantic models, in the order returned by Polaris.",
			},
		},
	}
}

func (d *semanticModelsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *semanticModelsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state semanticModelsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	names, err := listSemanticModels(ctx, d.client, state.Catalog.ValueString(), ns)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list semantic models", err.Error())
		return
	}
	if names == nil {
		names = []string{}
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, names)
	resp.Diagnostics.Append(diags...)
	state.Names = list
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}
