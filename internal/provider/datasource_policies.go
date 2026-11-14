package provider

import (
	"context"
	"fmt"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &policiesDataSource{}

type policiesDataSource struct {
	client *polarismanagement.APIClient
}

type policiesDataSourceModel struct {
	Catalog    types.String `tfsdk:"catalog"`
	Namespace  types.List   `tfsdk:"namespace"`
	PolicyType types.String `tfsdk:"policy_type"`
	Names      types.List   `tfsdk:"names"`
}

func newPoliciesDataSource() datasource.DataSource {
	return &policiesDataSource{}
}

func (d *policiesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policies"
}

func (d *policiesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the policies defined in a namespace of a Polaris catalog. Use `polaris_policy` for the details of one.",
		Attributes: map[string]schema.Attribute{
			"catalog": schema.StringAttribute{
				Required:    true,
				Description: "The name of the catalog.",
			},
			"namespace": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Namespace the policies are defined in, as an ordered list of levels.",
			},
			"policy_type": schema.StringAttribute{
				Optional:    true,
				Description: "Only list policies of this type, e.g. `system.data-compaction`.",
			},
			"names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Names of the policies, in the order returned by Polaris.",
			},
		},
	}
}

func (d *policiesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *policiesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state policiesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, state.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	names, err := listPolicies(ctx, d.client, state.Catalog.ValueString(), ns, state.PolicyType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to list policies", err.Error())
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
