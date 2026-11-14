package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func ptr[T any](v T) *T { return &v }

// nullStorage returns a storage_config object with every attribute null except storage_type.
func nullStorage(storageType string, set map[string]attr.Value) types.Object {
	vals := map[string]attr.Value{}
	for k, t := range catalogStorageAttrTypes {
		switch tt := t.(type) {
		case basetypes.StringType:
			vals[k] = types.StringNull()
		case basetypes.BoolType:
			vals[k] = types.BoolNull()
		case basetypes.ListType:
			vals[k] = types.ListNull(tt.ElemType)
		case basetypes.SetType:
			vals[k] = types.SetNull(tt.ElemType)
		}
	}
	vals["storage_type"] = types.StringValue(storageType)
	for k, v := range set {
		vals[k] = v
	}
	return types.ObjectValueMust(catalogStorageAttrTypes, vals)
}

func TestStorageToPayload_omitsNullAndUnknown(t *testing.T) {
	ctx := context.Background()
	var ds diag.Diagnostics
	obj := nullStorage("S3", map[string]attr.Value{
		"role_arn":          types.StringValue("arn:aws:iam::1:role/r"),
		"region":            types.StringUnknown(), // computed, not configured
		"path_style_access": types.BoolValue(false),
		"allowed_locations": types.SetValueMust(types.StringType, []attr.Value{types.StringValue("s3://b/p")}),
		"encryption_keys":   types.ListUnknown(types.StringType),
	})

	p := storageToPayload(ctx, obj, &ds)
	if ds.HasError() {
		t.Fatalf("diagnostics: %v", ds)
	}
	b, _ := json.Marshal(p)
	got := map[string]any{}
	_ = json.Unmarshal(b, &got)
	want := map[string]any{
		"storageType":      "S3",
		"roleArn":          "arn:aws:iam::1:role/r",
		"pathStyleAccess":  false, // explicitly configured false must be sent
		"allowedLocations": []any{"s3://b/p"},
	}
	if len(got) != len(want) {
		t.Fatalf("payload = %s, want only %v", b, want)
	}
	for k, v := range want {
		if gb, _ := json.Marshal(got[k]); string(gb) != mustJSON(v) {
			t.Errorf("%s = %s, want %s", k, gb, mustJSON(v))
		}
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestSplitCatalogProperties(t *testing.T) {
	ctx := context.Background()
	props := map[string]string{catalogBaseLocationProperty: "s3://b/p"}

	var ds diag.Diagnostics
	base, rest := splitCatalogProperties(ctx, props, types.MapNull(types.StringType), &ds)
	if base.ValueString() != "s3://b/p" {
		t.Errorf("base = %q", base.ValueString())
	}
	if !rest.IsNull() {
		t.Errorf("properties not configured: got %v, want null", rest)
	}

	_, rest = splitCatalogProperties(ctx, props, types.MapValueMust(types.StringType, map[string]attr.Value{}), &ds)
	if rest.IsNull() || len(rest.Elements()) != 0 {
		t.Errorf("properties configured as {}: got %v, want empty map", rest)
	}

	props["k"] = "v"
	_, rest = splitCatalogProperties(ctx, props, types.MapNull(types.StringType), &ds)
	if len(rest.Elements()) != 1 || rest.Elements()["k"].(types.String).ValueString() != "v" {
		t.Errorf("got %v, want {k = v} without default-base-location", rest)
	}
	if ds.HasError() {
		t.Fatalf("diagnostics: %v", ds)
	}
}

func TestCatalogProperties_addsBaseLocation(t *testing.T) {
	var ds diag.Diagnostics
	got := catalogProperties(context.Background(), types.StringValue("file:///tmp/c"), types.MapUnknown(types.StringType), &ds)
	if len(got) != 1 || got[catalogBaseLocationProperty] != "file:///tmp/c" {
		t.Errorf("got %v", got)
	}
}

func TestConnectionFromPayload_keepsSecretsAndNullness(t *testing.T) {
	ctx := context.Background()
	var ds diag.Diagnostics

	// What the user configured: secret set, no scopes, no properties.
	prior := connectionFromPayload(ctx, &connectionConfigPayload{
		ConnectionType: "ICEBERG_REST",
		URI:            ptr("https://remote"),
		AuthenticationParameters: &authParametersPayload{
			AuthenticationType: "OAUTH",
			ClientID:           ptr("id"),
			ClientSecret:       ptr("secret"),
		},
	}, types.ObjectNull(catalogConnectionAttrTypes), &ds)

	// What the server returns: no secret, empty properties and scopes.
	server := &connectionConfigPayload{
		ConnectionType: "ICEBERG_REST",
		URI:            ptr("https://remote"),
		Properties:     map[string]string{},
		AuthenticationParameters: &authParametersPayload{
			AuthenticationType: "OAUTH",
			ClientID:           ptr("id"),
			Scopes:             &[]string{},
		},
	}
	got := connectionFromPayload(ctx, server, prior, &ds)
	if ds.HasError() {
		t.Fatalf("diagnostics: %v", ds)
	}
	if !got.Equal(prior) {
		t.Errorf("state after read differs from the configuration:\n got  %v\n want %v", got, prior)
	}

	// After import there is no prior value: the secret stays null.
	imported := connectionFromPayload(ctx, server, types.ObjectNull(catalogConnectionAttrTypes), &ds)
	var m catalogConnectionModel
	var a catalogAuthModel
	ds.Append(imported.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	ds.Append(m.Authentication.As(ctx, &a, basetypes.ObjectAsOptions{})...)
	if !a.ClientSecret.IsNull() {
		t.Errorf("client_secret after import = %v, want null", a.ClientSecret)
	}
	if !m.Properties.IsNull() || !a.Scopes.IsNull() {
		t.Errorf("empty server values without prior: properties=%v scopes=%v, want null", m.Properties, a.Scopes)
	}
}

func TestConnectionFromPayload_nil(t *testing.T) {
	var ds diag.Diagnostics
	if got := connectionFromPayload(context.Background(), nil, types.ObjectNull(catalogConnectionAttrTypes), &ds); !got.IsNull() {
		t.Errorf("got %v, want null", got)
	}
	if got := serviceIdentityFromPayload(&connectionConfigPayload{ServiceIdentity: &serviceIdentityInfo{IdentityType: "AWS_IAM", IamArn: ptr("arn:x")}}); got.ValueString() != "arn:x" {
		t.Errorf("service identity = %v", got)
	}
}

func TestGetCatalog_decodesSubtypeFields(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusOK, `{
		"type": "EXTERNAL", "name": "c1", "entityVersion": 3,
		"properties": {"default-base-location": "s3://b/p"},
		"storageConfigInfo": {"storageType": "S3", "roleArn": "arn:r", "region": "eu-west-1", "pathStyleAccess": true},
		"connectionConfigInfo": {"connectionType": "ICEBERG_REST", "remoteCatalogName": "rc",
			"authenticationParameters": {"authenticationType": "SIGV4", "roleArn": "arn:s", "signingRegion": "eu-west-1"},
			"serviceIdentity": {"identityType": "AWS_IAM", "iamArn": "arn:polaris"}}
	}`)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	c, err := getCatalog(context.Background(), client, "c1")
	if err != nil {
		t.Fatalf("getCatalog: %v", err)
	}
	if c.StorageConfigInfo.RoleArn == nil || *c.StorageConfigInfo.RoleArn != "arn:r" ||
		c.StorageConfigInfo.PathStyleAccess == nil || !*c.StorageConfigInfo.PathStyleAccess {
		t.Errorf("storage subtype fields lost: %+v", c.StorageConfigInfo)
	}
	if c.ConnectionConfigInfo == nil || c.ConnectionConfigInfo.AuthenticationParameters.SigningRegion == nil ||
		*c.ConnectionConfigInfo.ServiceIdentity.IamArn != "arn:polaris" || *c.EntityVersion != 3 {
		t.Errorf("connection fields lost: %+v", c.ConnectionConfigInfo)
	}
}

func TestGetCatalog_notFound(t *testing.T) {
	srv, _ := newTestServer(t, http.StatusNotFound, `{"error":{"message":"no such catalog"}}`)
	client := newManagementClient(srv.URL, srv.Client(), "tok", "", defaultRealmHeader)

	if _, err := getCatalog(context.Background(), client, "missing"); !isNotFoundError(err) {
		t.Errorf("err = %v, want a not-found error", err)
	}
}
