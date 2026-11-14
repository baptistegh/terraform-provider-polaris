package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// computedAttributes turns resource schema attributes into read-only data source
// attributes with the same types, descriptions and sensitivity, so a data source
// that mirrors a resource cannot drift from it. Attributes named in skip are left out.
func computedAttributes(attrs map[string]rschema.Attribute, skip ...string) map[string]schema.Attribute {
	skipped := map[string]bool{}
	for _, s := range skip {
		skipped[s] = true
	}
	out := make(map[string]schema.Attribute, len(attrs))
	for name, a := range attrs {
		if skipped[name] {
			continue
		}
		out[name] = computedAttribute(a)
	}
	return out
}

func computedAttribute(a rschema.Attribute) schema.Attribute {
	desc, sensitive := a.GetDescription(), a.IsSensitive()
	switch v := a.(type) {
	case rschema.StringAttribute:
		return schema.StringAttribute{Computed: true, Description: desc, Sensitive: sensitive}
	case rschema.BoolAttribute:
		return schema.BoolAttribute{Computed: true, Description: desc, Sensitive: sensitive}
	case rschema.Int64Attribute:
		return schema.Int64Attribute{Computed: true, Description: desc, Sensitive: sensitive}
	case rschema.ListAttribute:
		return schema.ListAttribute{Computed: true, ElementType: v.ElementType, Description: desc, Sensitive: sensitive}
	case rschema.SetAttribute:
		return schema.SetAttribute{Computed: true, ElementType: v.ElementType, Description: desc, Sensitive: sensitive}
	case rschema.MapAttribute:
		return schema.MapAttribute{Computed: true, ElementType: v.ElementType, Description: desc, Sensitive: sensitive}
	case rschema.SingleNestedAttribute:
		return schema.SingleNestedAttribute{Computed: true, Attributes: computedAttributes(v.Attributes), Description: desc, Sensitive: sensitive}
	default:
		// Only reached when a resource gains an attribute kind not handled above.
		panic(fmt.Sprintf("computedAttribute: unsupported attribute type %T", a))
	}
}

// propertiesValue converts API properties; missing properties become an empty map
// so data source users can index or lookup() them without a null check.
func propertiesValue(ctx context.Context, p *map[string]string, ds *diag.Diagnostics) types.Map {
	if p == nil {
		return types.MapValueMust(types.StringType, map[string]attr.Value{})
	}
	m, d := types.MapValueFrom(ctx, types.StringType, *p)
	ds.Append(d...)
	return m
}

func int32Value(v *int32) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(int64(*v))
}
