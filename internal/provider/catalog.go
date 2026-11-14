package provider

import (
	"context"
	"net/http"
	"net/url"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Catalogs are called through doJSON rather than the generated client, like grants:
// the storage and connection configurations are polymorphic (allOf + discriminator),
// and the generated models only keep the base fields. StorageConfigInfo drops every
// S3/Azure/GCS field, and Catalog has no connectionConfigInfo at all.

const catalogBaseLocationProperty = "default-base-location"

type catalogPayload struct {
	Type                 string                   `json:"type"`
	Name                 string                   `json:"name"`
	Properties           map[string]string        `json:"properties"`
	CreateTimestamp      *int64                   `json:"createTimestamp,omitempty"`
	LastUpdateTimestamp  *int64                   `json:"lastUpdateTimestamp,omitempty"`
	EntityVersion        *int64                   `json:"entityVersion,omitempty"`
	StorageConfigInfo    storageConfigPayload     `json:"storageConfigInfo"`
	ConnectionConfigInfo *connectionConfigPayload `json:"connectionConfigInfo,omitempty"`
}

type storageConfigPayload struct {
	StorageType      string   `json:"storageType"`
	AllowedLocations []string `json:"allowedLocations,omitempty"`
	StorageName      *string  `json:"storageName,omitempty"`
	// S3
	RoleArn                    *string   `json:"roleArn,omitempty"`
	ExternalID                 *string   `json:"externalId,omitempty"`
	UserArn                    *string   `json:"userArn,omitempty"`
	EncryptionKeys             *[]string `json:"encryptionKeys,omitempty"`
	DecryptionKeys             *[]string `json:"decryptionKeys,omitempty"`
	Region                     *string   `json:"region,omitempty"`
	Endpoint                   *string   `json:"endpoint,omitempty"`
	StsEndpoint                *string   `json:"stsEndpoint,omitempty"`
	StsUnavailable             *bool     `json:"stsUnavailable,omitempty"`
	EndpointInternal           *string   `json:"endpointInternal,omitempty"`
	PathStyleAccess            *bool     `json:"pathStyleAccess,omitempty"`
	KmsUnavailable             *bool     `json:"kmsUnavailable,omitempty"`
	CredentialVendingMechanism *string   `json:"credentialVendingMechanism,omitempty"`
	// Azure
	TenantID           *string `json:"tenantId,omitempty"`
	MultiTenantAppName *string `json:"multiTenantAppName,omitempty"`
	ConsentURL         *string `json:"consentUrl,omitempty"`
	Hierarchical       *bool   `json:"hierarchical,omitempty"`
	// GCS
	GcsServiceAccount *string `json:"gcsServiceAccount,omitempty"`
}

type connectionConfigPayload struct {
	ConnectionType           string                 `json:"connectionType"`
	URI                      *string                `json:"uri,omitempty"`
	RemoteCatalogName        *string                `json:"remoteCatalogName,omitempty"`
	Warehouse                *string                `json:"warehouse,omitempty"`
	Properties               map[string]string      `json:"properties,omitempty"`
	AuthenticationParameters *authParametersPayload `json:"authenticationParameters,omitempty"`
	ServiceIdentity          *serviceIdentityInfo   `json:"serviceIdentity,omitempty"`
}

type authParametersPayload struct {
	AuthenticationType string    `json:"authenticationType"`
	TokenURI           *string   `json:"tokenUri,omitempty"`
	ClientID           *string   `json:"clientId,omitempty"`
	ClientSecret       *string   `json:"clientSecret,omitempty"`
	Scopes             *[]string `json:"scopes,omitempty"`
	BearerToken        *string   `json:"bearerToken,omitempty"`
	RoleArn            *string   `json:"roleArn,omitempty"`
	RoleSessionName    *string   `json:"roleSessionName,omitempty"`
	ExternalID         *string   `json:"externalId,omitempty"`
	SigningRegion      *string   `json:"signingRegion,omitempty"`
	SigningName        *string   `json:"signingName,omitempty"`
	SessionPolicy      *string   `json:"sessionPolicy,omitempty"`
}

type serviceIdentityInfo struct {
	IdentityType string  `json:"identityType"`
	IamArn       *string `json:"iamArn,omitempty"`
}

type updateCatalogPayload struct {
	CurrentEntityVersion int64                `json:"currentEntityVersion"`
	Properties           map[string]string    `json:"properties"`
	StorageConfigInfo    storageConfigPayload `json:"storageConfigInfo"`
}

func catalogPath(name string) string {
	return "/catalogs/" + url.PathEscape(name)
}

func createCatalog(ctx context.Context, client *polarismanagement.APIClient, c catalogPayload) (*catalogPayload, error) {
	var out catalogPayload
	if err := doJSON(ctx, client, http.MethodPost, "/catalogs", map[string]any{"catalog": c}, http.StatusCreated, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// getCatalog returns the catalog; use isNotFoundError on the error to detect a missing one.
func getCatalog(ctx context.Context, client *polarismanagement.APIClient, name string) (*catalogPayload, error) {
	var out catalogPayload
	if err := doJSON(ctx, client, http.MethodGet, catalogPath(name), nil, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func updateCatalog(ctx context.Context, client *polarismanagement.APIClient, name string, u updateCatalogPayload) (*catalogPayload, error) {
	var out catalogPayload
	if err := doJSON(ctx, client, http.MethodPut, catalogPath(name), u, http.StatusOK, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func deleteCatalog(ctx context.Context, client *polarismanagement.APIClient, name string) error {
	return doJSON(ctx, client, http.MethodDelete, catalogPath(name), nil, http.StatusNoContent, nil)
}

// ---------------------------------------------------------------------------
// Terraform models

type catalogStorageModel struct {
	StorageType                types.String `tfsdk:"storage_type"`
	AllowedLocations           types.Set    `tfsdk:"allowed_locations"`
	StorageName                types.String `tfsdk:"storage_name"`
	RoleArn                    types.String `tfsdk:"role_arn"`
	ExternalID                 types.String `tfsdk:"external_id"`
	UserArn                    types.String `tfsdk:"user_arn"`
	EncryptionKeys             types.List   `tfsdk:"encryption_keys"`
	DecryptionKeys             types.List   `tfsdk:"decryption_keys"`
	Region                     types.String `tfsdk:"region"`
	Endpoint                   types.String `tfsdk:"endpoint"`
	StsEndpoint                types.String `tfsdk:"sts_endpoint"`
	StsUnavailable             types.Bool   `tfsdk:"sts_unavailable"`
	EndpointInternal           types.String `tfsdk:"endpoint_internal"`
	PathStyleAccess            types.Bool   `tfsdk:"path_style_access"`
	KmsUnavailable             types.Bool   `tfsdk:"kms_unavailable"`
	CredentialVendingMechanism types.String `tfsdk:"credential_vending_mechanism"`
	TenantID                   types.String `tfsdk:"tenant_id"`
	MultiTenantAppName         types.String `tfsdk:"multi_tenant_app_name"`
	ConsentURL                 types.String `tfsdk:"consent_url"`
	Hierarchical               types.Bool   `tfsdk:"hierarchical"`
	GcsServiceAccount          types.String `tfsdk:"gcs_service_account"`
}

var catalogStorageAttrTypes = map[string]attr.Type{
	"storage_type":                 types.StringType,
	"allowed_locations":            types.SetType{ElemType: types.StringType},
	"storage_name":                 types.StringType,
	"role_arn":                     types.StringType,
	"external_id":                  types.StringType,
	"user_arn":                     types.StringType,
	"encryption_keys":              types.ListType{ElemType: types.StringType},
	"decryption_keys":              types.ListType{ElemType: types.StringType},
	"region":                       types.StringType,
	"endpoint":                     types.StringType,
	"sts_endpoint":                 types.StringType,
	"sts_unavailable":              types.BoolType,
	"endpoint_internal":            types.StringType,
	"path_style_access":            types.BoolType,
	"kms_unavailable":              types.BoolType,
	"credential_vending_mechanism": types.StringType,
	"tenant_id":                    types.StringType,
	"multi_tenant_app_name":        types.StringType,
	"consent_url":                  types.StringType,
	"hierarchical":                 types.BoolType,
	"gcs_service_account":          types.StringType,
}

type catalogConnectionModel struct {
	ConnectionType    types.String `tfsdk:"connection_type"`
	URI               types.String `tfsdk:"uri"`
	RemoteCatalogName types.String `tfsdk:"remote_catalog_name"`
	Warehouse         types.String `tfsdk:"warehouse"`
	Properties        types.Map    `tfsdk:"properties"`
	Authentication    types.Object `tfsdk:"authentication"`
}

var catalogConnectionAttrTypes = map[string]attr.Type{
	"connection_type":     types.StringType,
	"uri":                 types.StringType,
	"remote_catalog_name": types.StringType,
	"warehouse":           types.StringType,
	"properties":          types.MapType{ElemType: types.StringType},
	"authentication":      types.ObjectType{AttrTypes: catalogAuthAttrTypes},
}

type catalogAuthModel struct {
	Type            types.String `tfsdk:"type"`
	TokenURI        types.String `tfsdk:"token_uri"`
	ClientID        types.String `tfsdk:"client_id"`
	ClientSecret    types.String `tfsdk:"client_secret"`
	Scopes          types.List   `tfsdk:"scopes"`
	BearerToken     types.String `tfsdk:"bearer_token"`
	RoleArn         types.String `tfsdk:"role_arn"`
	RoleSessionName types.String `tfsdk:"role_session_name"`
	ExternalID      types.String `tfsdk:"external_id"`
	SigningRegion   types.String `tfsdk:"signing_region"`
	SigningName     types.String `tfsdk:"signing_name"`
	SessionPolicy   types.String `tfsdk:"session_policy"`
}

var catalogAuthAttrTypes = map[string]attr.Type{
	"type":              types.StringType,
	"token_uri":         types.StringType,
	"client_id":         types.StringType,
	"client_secret":     types.StringType,
	"scopes":            types.ListType{ElemType: types.StringType},
	"bearer_token":      types.StringType,
	"role_arn":          types.StringType,
	"role_session_name": types.StringType,
	"external_id":       types.StringType,
	"signing_region":    types.StringType,
	"signing_name":      types.StringType,
	"session_policy":    types.StringType,
}

// ---------------------------------------------------------------------------
// Terraform values -> API payloads. Null and unknown values are left out.

func optString(v types.String) *string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	s := v.ValueString()
	return &s
}

func optBool(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	b := v.ValueBool()
	return &b
}

func optStrings(ctx context.Context, v basetypes.ListValue, ds *diag.Diagnostics) *[]string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	var out []string
	ds.Append(v.ElementsAs(ctx, &out, false)...)
	return &out
}

func optStringMap(ctx context.Context, v types.Map, ds *diag.Diagnostics) map[string]string {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	out := map[string]string{}
	ds.Append(v.ElementsAs(ctx, &out, false)...)
	return out
}

func storageToPayload(ctx context.Context, obj types.Object, ds *diag.Diagnostics) storageConfigPayload {
	var m catalogStorageModel
	ds.Append(obj.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	p := storageConfigPayload{
		StorageType:                m.StorageType.ValueString(),
		StorageName:                optString(m.StorageName),
		RoleArn:                    optString(m.RoleArn),
		ExternalID:                 optString(m.ExternalID),
		UserArn:                    optString(m.UserArn),
		EncryptionKeys:             optStrings(ctx, m.EncryptionKeys, ds),
		DecryptionKeys:             optStrings(ctx, m.DecryptionKeys, ds),
		Region:                     optString(m.Region),
		Endpoint:                   optString(m.Endpoint),
		StsEndpoint:                optString(m.StsEndpoint),
		StsUnavailable:             optBool(m.StsUnavailable),
		EndpointInternal:           optString(m.EndpointInternal),
		PathStyleAccess:            optBool(m.PathStyleAccess),
		KmsUnavailable:             optBool(m.KmsUnavailable),
		CredentialVendingMechanism: optString(m.CredentialVendingMechanism),
		TenantID:                   optString(m.TenantID),
		MultiTenantAppName:         optString(m.MultiTenantAppName),
		ConsentURL:                 optString(m.ConsentURL),
		Hierarchical:               optBool(m.Hierarchical),
		GcsServiceAccount:          optString(m.GcsServiceAccount),
	}
	if !m.AllowedLocations.IsNull() && !m.AllowedLocations.IsUnknown() {
		ds.Append(m.AllowedLocations.ElementsAs(ctx, &p.AllowedLocations, false)...)
	}
	return p
}

func connectionToPayload(ctx context.Context, obj types.Object, ds *diag.Diagnostics) *connectionConfigPayload {
	if obj.IsNull() || obj.IsUnknown() {
		return nil
	}
	var m catalogConnectionModel
	ds.Append(obj.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	p := &connectionConfigPayload{
		ConnectionType:    m.ConnectionType.ValueString(),
		URI:               optString(m.URI),
		RemoteCatalogName: optString(m.RemoteCatalogName),
		Warehouse:         optString(m.Warehouse),
		Properties:        optStringMap(ctx, m.Properties, ds),
	}
	if !m.Authentication.IsNull() && !m.Authentication.IsUnknown() {
		var a catalogAuthModel
		ds.Append(m.Authentication.As(ctx, &a, basetypes.ObjectAsOptions{})...)
		p.AuthenticationParameters = &authParametersPayload{
			AuthenticationType: a.Type.ValueString(),
			TokenURI:           optString(a.TokenURI),
			ClientID:           optString(a.ClientID),
			ClientSecret:       optString(a.ClientSecret),
			Scopes:             optStrings(ctx, a.Scopes, ds),
			BearerToken:        optString(a.BearerToken),
			RoleArn:            optString(a.RoleArn),
			RoleSessionName:    optString(a.RoleSessionName),
			ExternalID:         optString(a.ExternalID),
			SigningRegion:      optString(a.SigningRegion),
			SigningName:        optString(a.SigningName),
			SessionPolicy:      optString(a.SessionPolicy),
		}
	}
	return p
}

// catalogProperties merges default_base_location into the properties map sent to the API.
func catalogProperties(ctx context.Context, baseLocation types.String, props types.Map, ds *diag.Diagnostics) map[string]string {
	out := optStringMap(ctx, props, ds)
	if out == nil {
		out = map[string]string{}
	}
	out[catalogBaseLocationProperty] = baseLocation.ValueString()
	return out
}

// ---------------------------------------------------------------------------
// API payloads -> Terraform values.

func stringListValue(ctx context.Context, v *[]string, ds *diag.Diagnostics) types.List {
	if v == nil {
		return types.ListNull(types.StringType)
	}
	l, d := types.ListValueFrom(ctx, types.StringType, *v)
	ds.Append(d...)
	return l
}

func storageFromPayload(ctx context.Context, p storageConfigPayload, ds *diag.Diagnostics) types.Object {
	locations, d := types.SetValueFrom(ctx, types.StringType, p.AllowedLocations)
	ds.Append(d...)
	if p.AllowedLocations == nil {
		locations = types.SetNull(types.StringType)
	}
	m := catalogStorageModel{
		StorageType:                types.StringValue(p.StorageType),
		AllowedLocations:           locations,
		StorageName:                types.StringPointerValue(p.StorageName),
		RoleArn:                    types.StringPointerValue(p.RoleArn),
		ExternalID:                 types.StringPointerValue(p.ExternalID),
		UserArn:                    types.StringPointerValue(p.UserArn),
		EncryptionKeys:             stringListValue(ctx, p.EncryptionKeys, ds),
		DecryptionKeys:             stringListValue(ctx, p.DecryptionKeys, ds),
		Region:                     types.StringPointerValue(p.Region),
		Endpoint:                   types.StringPointerValue(p.Endpoint),
		StsEndpoint:                types.StringPointerValue(p.StsEndpoint),
		StsUnavailable:             types.BoolPointerValue(p.StsUnavailable),
		EndpointInternal:           types.StringPointerValue(p.EndpointInternal),
		PathStyleAccess:            types.BoolPointerValue(p.PathStyleAccess),
		KmsUnavailable:             types.BoolPointerValue(p.KmsUnavailable),
		CredentialVendingMechanism: types.StringPointerValue(p.CredentialVendingMechanism),
		TenantID:                   types.StringPointerValue(p.TenantID),
		MultiTenantAppName:         types.StringPointerValue(p.MultiTenantAppName),
		ConsentURL:                 types.StringPointerValue(p.ConsentURL),
		Hierarchical:               types.BoolPointerValue(p.Hierarchical),
		GcsServiceAccount:          types.StringPointerValue(p.GcsServiceAccount),
	}
	obj, d := types.ObjectValueFrom(ctx, catalogStorageAttrTypes, m)
	ds.Append(d...)
	return obj
}

// connectionFromPayload converts the API connection config. The API never returns
// secrets (client_secret, bearer_token), so they are taken from prior, the plan or
// state value. prior also decides whether an empty properties map or scope list
// returned by the server is stored as null (not configured) or empty.
func connectionFromPayload(ctx context.Context, p *connectionConfigPayload, prior types.Object, ds *diag.Diagnostics) types.Object {
	if p == nil {
		return types.ObjectNull(catalogConnectionAttrTypes)
	}
	var prev catalogConnectionModel
	var prevAuth catalogAuthModel
	hasPrev := !prior.IsNull() && !prior.IsUnknown()
	if hasPrev {
		ds.Append(prior.As(ctx, &prev, basetypes.ObjectAsOptions{})...)
		if !prev.Authentication.IsNull() && !prev.Authentication.IsUnknown() {
			ds.Append(prev.Authentication.As(ctx, &prevAuth, basetypes.ObjectAsOptions{})...)
		}
	}

	props := types.MapNull(types.StringType)
	if len(p.Properties) > 0 || (hasPrev && !prev.Properties.IsNull()) {
		var d diag.Diagnostics
		props, d = types.MapValueFrom(ctx, types.StringType, p.Properties)
		ds.Append(d...)
		if p.Properties == nil {
			props = types.MapValueMust(types.StringType, map[string]attr.Value{})
		}
	}

	auth := types.ObjectNull(catalogAuthAttrTypes)
	if a := p.AuthenticationParameters; a != nil {
		scopes := stringListValue(ctx, a.Scopes, ds)
		if a.Scopes != nil && len(*a.Scopes) == 0 && (prevAuth.Scopes.IsNull() || prevAuth.Scopes.IsUnknown()) {
			scopes = types.ListNull(types.StringType)
		}
		am := catalogAuthModel{
			Type:            types.StringValue(a.AuthenticationType),
			TokenURI:        types.StringPointerValue(a.TokenURI),
			ClientID:        types.StringPointerValue(a.ClientID),
			ClientSecret:    keepSecret(a.ClientSecret, prevAuth.ClientSecret),
			Scopes:          scopes,
			BearerToken:     keepSecret(a.BearerToken, prevAuth.BearerToken),
			RoleArn:         types.StringPointerValue(a.RoleArn),
			RoleSessionName: types.StringPointerValue(a.RoleSessionName),
			ExternalID:      types.StringPointerValue(a.ExternalID),
			SigningRegion:   types.StringPointerValue(a.SigningRegion),
			SigningName:     types.StringPointerValue(a.SigningName),
			SessionPolicy:   types.StringPointerValue(a.SessionPolicy),
		}
		var d diag.Diagnostics
		auth, d = types.ObjectValueFrom(ctx, catalogAuthAttrTypes, am)
		ds.Append(d...)
	}

	m := catalogConnectionModel{
		ConnectionType:    types.StringValue(p.ConnectionType),
		URI:               types.StringPointerValue(p.URI),
		RemoteCatalogName: types.StringPointerValue(p.RemoteCatalogName),
		Warehouse:         types.StringPointerValue(p.Warehouse),
		Properties:        props,
		Authentication:    auth,
	}
	obj, d := types.ObjectValueFrom(ctx, catalogConnectionAttrTypes, m)
	ds.Append(d...)
	return obj
}

// keepSecret returns the server value if it sent one, otherwise the previous value.
func keepSecret(server *string, prev types.String) types.String {
	if server != nil {
		return types.StringValue(*server)
	}
	if prev.IsUnknown() {
		return types.StringNull()
	}
	return prev
}

func serviceIdentityFromPayload(p *connectionConfigPayload) types.String {
	if p == nil || p.ServiceIdentity == nil {
		return types.StringNull()
	}
	return types.StringPointerValue(p.ServiceIdentity.IamArn)
}

// splitCatalogProperties separates default-base-location from the other properties.
// prevProps decides whether "no other properties" is stored as null or as an empty map.
func splitCatalogProperties(ctx context.Context, props map[string]string, prevProps types.Map, ds *diag.Diagnostics) (types.String, types.Map) {
	base := types.StringNull()
	rest := map[string]string{}
	for k, v := range props {
		if k == catalogBaseLocationProperty {
			base = types.StringValue(v)
			continue
		}
		rest[k] = v
	}
	if len(rest) == 0 && (prevProps.IsNull() || prevProps.IsUnknown()) {
		return base, types.MapNull(types.StringType)
	}
	m, d := types.MapValueFrom(ctx, types.StringType, rest)
	ds.Append(d...)
	return base, m
}

func int64Value(v *int64) types.Int64 {
	if v == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*v)
}
