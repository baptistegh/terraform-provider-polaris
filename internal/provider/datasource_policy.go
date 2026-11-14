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

var _ datasource.DataSource = &policyDataSource{}

type policyDataSource struct {
	client *polarismanagement.APIClient
}

func newPolicyDataSource() datasource.DataSource {
	return &policyDataSource{}
}

func (d *policyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (d *policyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	// Same attributes as the polaris_policy resource, read-only except the identifiers.
	var rs resource.SchemaResponse
	(&policyResource{}).Schema(context.Background(), resource.SchemaRequest{}, &rs)
	attrs := computedAttributes(rs.Schema.Attributes)
	attrs["catalog"] = schema.StringAttribute{Required: true, Description: "The name of the catalog."}
	attrs["namespace"] = schema.ListAttribute{Required: true, ElementType: types.StringType, Description: "Namespace the policy is defined in, as an ordered list of levels."}
	attrs["name"] = schema.StringAttribute{Required: true, Description: "The name of the policy."}
	resp.Schema = schema.Schema{
		Description: "Fetches a Polaris policy.",
		Attributes:  attrs,
	}
}

func (d *policyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *policyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config policyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ns := listToStringSlice(ctx, config.Namespace, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	got, err := getPolicy(ctx, d.client, config.Catalog.ValueString(), ns, config.Name.ValueString())
	if isNotFoundError(err) {
		resp.Diagnostics.AddError("Policy not found", fmt.Sprintf("No policy %q exists in namespace %q of catalog %q.", config.Name.ValueString(), strings.Join(ns, "."), config.Catalog.ValueString()))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to get policy", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, policyFromAPI(config, *got))...)
}
