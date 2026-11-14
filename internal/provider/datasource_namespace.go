package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &namespaceDataSource{}

type namespaceDataSource struct {
	client *polarismanagement.APIClient
}

func newNamespaceDataSource() datasource.DataSource {
	return &namespaceDataSource{}
}

func (d *namespaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_namespace"
}

func (d *namespaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches a namespace of a Polaris catalog.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog.",
			},
			"namespace": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Namespace path as an ordered list of levels, e.g. `[\"sales\", \"eu\"]`.",
			},
			"properties": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Namespace properties, without `location`. An empty map when the namespace has none.",
			},
			"location": schema.StringAttribute{
				Computed:    true,
				Description: "Storage location of the namespace.",
			},
		},
	}
}

func (d *namespaceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *namespaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config namespaceResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	catalog := config.Catalog.ValueString()
	ns := listToStringSlice(ctx, config.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := getNamespace(ctx, d.client, catalog, ns)
	if isNotFoundError(err) {
		resp.Diagnostics.AddError("Namespace not found", fmt.Sprintf("No namespace %q exists in catalog %q.", strings.Join(ns, "."), catalog))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to get namespace", err.Error())
		return
	}

	// An empty prior map makes "no properties" read back as {} rather than null.
	prior := namespaceResourceModel{Properties: types.MapValueMust(types.StringType, nil), Location: types.StringNull()}
	state := namespaceFromAPI(ctx, catalog, got, prior, &resp.Diagnostics)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	}
}
