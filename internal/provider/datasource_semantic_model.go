package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &semanticModelDataSource{}

type semanticModelDataSource struct {
	client *polarismanagement.APIClient
}

func newSemanticModelDataSource() datasource.DataSource {
	return &semanticModelDataSource{}
}

func (d *semanticModelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_semantic_model"
}

func (d *semanticModelDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	// Same attributes as the polaris_semantic_model resource, read-only except the identifiers.
	var rs resource.SchemaResponse
	(&semanticModelResource{}).Schema(context.Background(), resource.SchemaRequest{}, &rs)
	attrs := computedAttributes(rs.Schema.Attributes)
	attrs["catalog"] = schema.StringAttribute{Required: true, Description: "The name of the catalog."}
	attrs["namespace"] = schema.ListAttribute{Required: true, ElementType: types.StringType, Description: "Namespace the semantic model is stored in, as an ordered list of levels."}
	attrs["name"] = schema.StringAttribute{Required: true, Description: "The name of the semantic model."}
	resp.Schema = schema.Schema{
		Description: "Fetches a Polaris semantic model. Requires the `ENABLE_SEMANTIC_MODELS` feature on the server.",
		Attributes:  attrs,
	}
}

func (d *semanticModelDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *semanticModelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config semanticModelResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, config.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := getSemanticModel(ctx, d.client, config.Catalog.ValueString(), ns, config.Name.ValueString())
	if isNotFoundError(err) {
		resp.Diagnostics.AddError("Semantic model not found", fmt.Sprintf("No semantic model %q exists in namespace %q of catalog %q.", config.Name.ValueString(), strings.Join(ns, "."), config.Catalog.ValueString()))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to get semantic model", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, semanticModelFromAPI(config, *got))...)
}
