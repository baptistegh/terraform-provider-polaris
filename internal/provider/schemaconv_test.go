package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// The catalog data sources derive their schema from the resource: every attribute,
// nested ones included, must be read-only with the same description and sensitivity.
func TestComputedAttributes_catalog(t *testing.T) {
	var rs resource.SchemaResponse
	(&catalogResource{}).Schema(context.Background(), resource.SchemaRequest{}, &rs)

	got := catalogDataSourceAttributes()
	if len(got) != len(rs.Schema.Attributes) {
		t.Fatalf("got %d attributes, want %d", len(got), len(rs.Schema.Attributes))
	}

	var check func(prefix string, attrs map[string]schema.Attribute)
	check = func(prefix string, attrs map[string]schema.Attribute) {
		for name, a := range attrs {
			if !a.IsComputed() || a.IsOptional() || a.IsRequired() {
				t.Errorf("%s%s is not read-only", prefix, name)
			}
			if a.GetDescription() == "" {
				t.Errorf("%s%s has no description", prefix, name)
			}
			if n, ok := a.(schema.SingleNestedAttribute); ok {
				check(prefix+name+".", n.Attributes)
			}
		}
	}
	check("", got)

	conn := got["connection_config"].(schema.SingleNestedAttribute)
	auth := conn.Attributes["authentication"].(schema.SingleNestedAttribute)
	if !auth.Attributes["client_secret"].IsSensitive() || !auth.Attributes["bearer_token"].IsSensitive() {
		t.Error("connection secrets lost their sensitivity")
	}
}

func TestComputedAttributes_skip(t *testing.T) {
	var rs resource.SchemaResponse
	(&catalogResource{}).Schema(context.Background(), resource.SchemaRequest{}, &rs)
	if _, ok := computedAttributes(rs.Schema.Attributes, "name")["name"]; ok {
		t.Error("skipped attribute is present")
	}
}
