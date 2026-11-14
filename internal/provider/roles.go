package provider

import (
	"context"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Read-only attributes shared by the catalog role, principal role and principal data sources.

func entityAttributes(kind string) map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"name": schema.StringAttribute{
			Computed:    true,
			Description: "The name of the " + kind + ".",
		},
		"properties": schema.MapAttribute{
			Computed:    true,
			ElementType: types.StringType,
			Description: "Key-value metadata attached to the " + kind + ".",
		},
		"create_timestamp": schema.Int64Attribute{
			Computed:    true,
			Description: "Creation time as a Unix epoch timestamp in milliseconds.",
		},
		"last_update_timestamp": schema.Int64Attribute{
			Computed:    true,
			Description: "Last update time as a Unix epoch timestamp in milliseconds.",
		},
		"entity_version": schema.Int64Attribute{
			Computed:    true,
			Description: "Version of the " + kind + " object, used for optimistic concurrency control.",
		},
	}
}

func catalogRoleAttributes() map[string]schema.Attribute {
	return entityAttributes("catalog role")
}

func principalRoleAttributes() map[string]schema.Attribute {
	attrs := entityAttributes("principal role")
	attrs["federated"] = schema.BoolAttribute{
		Computed:    true,
		Description: "Whether the principal role is federated, that is managed by an external identity provider.",
	}
	return attrs
}

func principalAttributes() map[string]schema.Attribute {
	attrs := entityAttributes("principal")
	attrs["client_id"] = schema.StringAttribute{
		Computed:    true,
		Description: "The OAuth client ID assigned to this principal by Polaris.",
	}
	return attrs
}

type catalogRoleModel struct {
	Name                types.String `tfsdk:"name"`
	Properties          types.Map    `tfsdk:"properties"`
	CreateTimestamp     types.Int64  `tfsdk:"create_timestamp"`
	LastUpdateTimestamp types.Int64  `tfsdk:"last_update_timestamp"`
	EntityVersion       types.Int64  `tfsdk:"entity_version"`
}

type principalRoleModel struct {
	Name                types.String `tfsdk:"name"`
	Federated           types.Bool   `tfsdk:"federated"`
	Properties          types.Map    `tfsdk:"properties"`
	CreateTimestamp     types.Int64  `tfsdk:"create_timestamp"`
	LastUpdateTimestamp types.Int64  `tfsdk:"last_update_timestamp"`
	EntityVersion       types.Int64  `tfsdk:"entity_version"`
}

type principalModel struct {
	Name                types.String `tfsdk:"name"`
	ClientID            types.String `tfsdk:"client_id"`
	Properties          types.Map    `tfsdk:"properties"`
	CreateTimestamp     types.Int64  `tfsdk:"create_timestamp"`
	LastUpdateTimestamp types.Int64  `tfsdk:"last_update_timestamp"`
	EntityVersion       types.Int64  `tfsdk:"entity_version"`
}

func catalogRoleData(ctx context.Context, r polarismanagement.CatalogRole, ds *diag.Diagnostics) catalogRoleModel {
	return catalogRoleModel{
		Name:                types.StringValue(r.Name),
		Properties:          propertiesValue(ctx, r.Properties, ds),
		CreateTimestamp:     int64Value(r.CreateTimestamp),
		LastUpdateTimestamp: int64Value(r.LastUpdateTimestamp),
		EntityVersion:       int32Value(r.EntityVersion),
	}
}

func principalRoleData(ctx context.Context, r polarismanagement.PrincipalRole, ds *diag.Diagnostics) principalRoleModel {
	federated := types.BoolValue(false)
	if r.Federated != nil {
		federated = types.BoolValue(*r.Federated)
	}
	return principalRoleModel{
		Name:                types.StringValue(r.Name),
		Federated:           federated,
		Properties:          propertiesValue(ctx, r.Properties, ds),
		CreateTimestamp:     int64Value(r.CreateTimestamp),
		LastUpdateTimestamp: int64Value(r.LastUpdateTimestamp),
		EntityVersion:       int32Value(r.EntityVersion),
	}
}

func principalData(ctx context.Context, p polarismanagement.Principal, ds *diag.Diagnostics) principalModel {
	return principalModel{
		Name:                types.StringValue(p.Name),
		ClientID:            types.StringPointerValue(p.ClientId),
		Properties:          propertiesValue(ctx, p.Properties, ds),
		CreateTimestamp:     int64Value(p.CreateTimestamp),
		LastUpdateTimestamp: int64Value(p.LastUpdateTimestamp),
		EntityVersion:       int32Value(p.EntityVersion),
	}
}

// namesAndList builds the `names` and object list attributes of a list data source.
func namesAndList[T any](ctx context.Context, attrs map[string]schema.Attribute, names []string, items []T, ds *diag.Diagnostics) (types.List, types.List) {
	n, d := types.ListValueFrom(ctx, types.StringType, names)
	ds.Append(d...)
	l, d := types.ListValueFrom(ctx, schema.NestedAttributeObject{Attributes: attrs}.Type(), items)
	ds.Append(d...)
	return n, l
}
