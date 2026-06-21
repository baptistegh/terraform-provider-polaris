package provider

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/baptistegh/terraform-provider-polaris/pkg/polarismanagement"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	defaultRealmHeader = "Polaris-Realm"
	tokenScope         = "PRINCIPAL_ROLE:ALL"
)

var _ provider.Provider = &PolarisProvider{}

type PolarisProvider struct {
	version string
}

type providerModel struct {
	BaseURL               types.String `tfsdk:"base_url"`
	ClientID              types.String `tfsdk:"client_id"`
	ClientSecret          types.String `tfsdk:"client_secret"`
	Token                 types.String `tfsdk:"token"`
	Realm                 types.String `tfsdk:"realm"`
	RealmHeader           types.String `tfsdk:"realm_header"`
	TLSInsecureSkipVerify types.Bool   `tfsdk:"tls_insecure_skip_verify"`
}

// ProviderData is passed to every resource and data source via their Configure method.
type ProviderData struct {
	Client   *polarismanagement.Client
	Realm    string
	ClientID string
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &PolarisProvider{version: version}
	}
}

func (p *PolarisProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "polaris"
	resp.Version = p.version
}

func (p *PolarisProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Base URL of the Polaris server (e.g. http://localhost:8181). Env: POLARIS_BASE_URL.",
			},
			"client_id": schema.StringAttribute{
				Optional:    true,
				Description: "OAuth2 client ID. Used with client_secret to exchange for a Bearer token. Env: POLARIS_CLIENT_ID.",
			},
			"client_secret": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "OAuth2 client secret. Used with client_id to exchange for a Bearer token. Env: POLARIS_CLIENT_SECRET.",
			},
			"token": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Pre-fetched Bearer token. Mutually exclusive with client_id/client_secret. Env: POLARIS_TOKEN.",
			},
			"realm": schema.StringAttribute{
				Optional:    true,
				Description: "Realm identifier for multi-tenant Polaris deployments, sent as a request header. Env: POLARIS_REALM.",
			},
			"realm_header": schema.StringAttribute{
				Optional:    true,
				Description: "HTTP header name used to pass the realm. Defaults to \"Polaris-Realm\".",
			},
			"tls_insecure_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Description: "Disable TLS certificate verification. For development use only.",
			},
		},
	}
}

func (p *PolarisProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := coalesce(config.BaseURL.ValueString(), os.Getenv("POLARIS_BASE_URL"))
	clientID := coalesce(config.ClientID.ValueString(), os.Getenv("POLARIS_CLIENT_ID"))
	clientSecret := coalesce(config.ClientSecret.ValueString(), os.Getenv("POLARIS_CLIENT_SECRET"))
	token := coalesce(config.Token.ValueString(), os.Getenv("POLARIS_TOKEN"))
	realm := coalesce(config.Realm.ValueString(), os.Getenv("POLARIS_REALM"))
	realmHeader := config.RealmHeader.ValueString()
	if realmHeader == "" {
		realmHeader = defaultRealmHeader
	}

	if baseURL == "" {
		resp.Diagnostics.AddError(
			"Missing base_url",
			"Set base_url in the provider block or via the POLARIS_BASE_URL environment variable.",
		)
		return
	}

	hasClientCreds := clientID != "" && clientSecret != ""
	hasToken := token != ""

	switch {
	case hasClientCreds && hasToken:
		resp.Diagnostics.AddError(
			"Conflicting credentials",
			"Provide either client_id/client_secret or token, not both.",
		)
		return
	case !hasClientCreds && !hasToken:
		resp.Diagnostics.AddError(
			"Missing credentials",
			"Provide either client_id + client_secret (or POLARIS_CLIENT_ID/POLARIS_CLIENT_SECRET) or a pre-fetched token (POLARIS_TOKEN).",
		)
		return
	case clientID != "" && clientSecret == "":
		resp.Diagnostics.AddError("Incomplete credentials", "client_secret is required when client_id is set.")
		return
	case clientSecret != "" && clientID == "":
		resp.Diagnostics.AddError("Incomplete credentials", "client_id is required when client_secret is set.")
		return
	}

	httpClient := &http.Client{}
	if config.TLSInsecureSkipVerify.ValueBool() {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}

	if hasClientCreds {
		fetched, err := fetchToken(ctx, httpClient, baseURL, clientID, clientSecret, realm, realmHeader)
		if err != nil {
			resp.Diagnostics.AddError("Failed to fetch OAuth token", err.Error())
			return
		}
		token = fetched
	}

	managementURL := strings.TrimRight(baseURL, "/") + "/api/management/v1"
	client, err := polarismanagement.NewClient(
		managementURL,
		polarismanagement.WithHTTPClient(httpClient),
		polarismanagement.WithRequestEditorFn(func(_ context.Context, r *http.Request) error {
			r.Header.Set("Authorization", "Bearer "+token)
			if realm != "" {
				r.Header.Set(realmHeader, realm)
			}
			return nil
		}),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create Polaris client", err.Error())
		return
	}

	pd := &ProviderData{Client: client, Realm: realm, ClientID: clientID}
	resp.DataSourceData = pd
	resp.ResourceData = pd
}

func (p *PolarisProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		newPrincipalResource,
		newPrincipalRoleResource,
		newCatalogRoleResource,
		newPrincipalRoleAssignmentResource,
		newCatalogRoleAssignmentResource,
		newCatalogGrantResource,
		newNamespaceGrantResource,
		newTableGrantResource,
		newViewGrantResource,
		newPolicyGrantResource,
	}
}

func (p *PolarisProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		newRealmDataSource,
		newPrincipalDataSource,
	}
}

func (p *PolarisProvider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

func fetchToken(ctx context.Context, httpClient *http.Client, baseURL, clientID, clientSecret, realm, realmHeader string) (string, error) {
	tokenURL := strings.TrimRight(baseURL, "/") + "/api/catalog/v1/oauth/tokens"

	body := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {tokenScope},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if realm != "" {
		req.Header.Set(realmHeader, realm)
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = res.Body.Close() }()

	var tok tokenResponse
	if err := json.NewDecoder(res.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("decoding token response (HTTP %d): %w", res.StatusCode, err)
	}

	if tok.AccessToken == "" {
		if tok.Error != "" {
			return "", fmt.Errorf("%s: %s", tok.Error, tok.ErrorDesc)
		}
		return "", fmt.Errorf("no access_token in response (HTTP %d)", res.StatusCode)
	}

	return tok.AccessToken, nil
}

// coalesce returns the first non-empty string.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
