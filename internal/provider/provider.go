package provider

import (
	"context"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &azexecuteProvider{}

type azexecuteProvider struct{ version string }

type providerModel struct {
	Endpoint                  types.String `tfsdk:"endpoint"`
	TenantID                  types.String `tfsdk:"tenant_id"`
	ClientID                  types.String `tfsdk:"client_id"`
	ClientSecret              types.String `tfsdk:"client_secret"`
	ClientCertificatePath     types.String `tfsdk:"client_certificate_path"`
	ClientCertificatePassword types.String `tfsdk:"client_certificate_password"`
	SendCertificateChain      types.Bool   `tfsdk:"send_certificate_chain"`
	AccessToken               types.String `tfsdk:"access_token"`
	UseOIDC                   types.Bool   `tfsdk:"use_oidc"`
	OIDCToken                 types.String `tfsdk:"oidc_token"`
	OIDCTokenFilePath         types.String `tfsdk:"oidc_token_file_path"`
	OIDCAudience              types.String `tfsdk:"oidc_audience"`
	UseManagedIdentity        types.Bool   `tfsdk:"use_managed_identity"`
	Scope                     types.String `tfsdk:"scope"`
	RequestTimeout            types.Int64  `tfsdk:"request_timeout_seconds"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider { return &azexecuteProvider{version: version} }
}

func (p *azexecuteProvider) Metadata(_ context.Context, _ provider.MetadataRequest, response *provider.MetadataResponse) {
	response.TypeName = "azexecute"
	response.Version = p.version
}

func (p *azexecuteProvider) Schema(_ context.Context, _ provider.SchemaRequest, response *provider.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "Manage the tenant-approved subset of AZExecute application registrations through its dedicated Terraform API.",
		Attributes: map[string]schema.Attribute{
			"endpoint":                    schema.StringAttribute{Optional: true, Description: "AZExecute API base URL. Defaults to AZEXECUTE_ENDPOINT or https://api.azexecute.com."},
			"tenant_id":                   schema.StringAttribute{Optional: true, Description: "Customer Microsoft Entra tenant ID. Defaults to AZEXECUTE_TENANT_ID, ARM_TENANT_ID, or AZURE_TENANT_ID."},
			"client_id":                   schema.StringAttribute{Optional: true, Description: "Customer automation identity client ID. Defaults to AZEXECUTE_CLIENT_ID, ARM_CLIENT_ID, or AZURE_CLIENT_ID."},
			"client_secret":               schema.StringAttribute{Optional: true, Sensitive: true, Description: "Client secret fallback. Prefer OIDC, managed identity, or a certificate. Defaults to AZEXECUTE_CLIENT_SECRET, ARM_CLIENT_SECRET, or AZURE_CLIENT_SECRET."},
			"client_certificate_path":     schema.StringAttribute{Optional: true, Description: "Path to a PEM or PKCS#12 client certificate containing its private key. Defaults to AZEXECUTE_CLIENT_CERTIFICATE_PATH or ARM_CLIENT_CERTIFICATE_PATH."},
			"client_certificate_password": schema.StringAttribute{Optional: true, Sensitive: true, Description: "Password for a PKCS#12 client certificate. Defaults to AZEXECUTE_CLIENT_CERTIFICATE_PASSWORD or ARM_CLIENT_CERTIFICATE_PASSWORD."},
			"send_certificate_chain":      schema.BoolAttribute{Optional: true, Description: "Send the certificate chain for subject-name/issuer authentication."},
			"access_token":                schema.StringAttribute{Optional: true, Sensitive: true, Description: "Pre-acquired bearer token. Defaults to AZEXECUTE_ACCESS_TOKEN."},
			"use_oidc":                    schema.BoolAttribute{Optional: true, Description: "Use workload identity federation. Automatically enabled for an OIDC token/file or GitHub Actions OIDC environment."},
			"oidc_token":                  schema.StringAttribute{Optional: true, Sensitive: true, Description: "Federated OIDC assertion. Defaults to AZEXECUTE_OIDC_TOKEN or ARM_OIDC_TOKEN."},
			"oidc_token_file_path":        schema.StringAttribute{Optional: true, Description: "File containing a rotating federated assertion. Defaults to AZEXECUTE_OIDC_TOKEN_FILE_PATH, ARM_OIDC_TOKEN_FILE_PATH, or AZURE_FEDERATED_TOKEN_FILE."},
			"oidc_audience":               schema.StringAttribute{Optional: true, Description: "OIDC audience requested from GitHub Actions. Defaults to api://AzureADTokenExchange."},
			"use_managed_identity":        schema.BoolAttribute{Optional: true, Description: "Use an Azure managed identity, optionally selected by client_id. Defaults from AZEXECUTE_USE_MANAGED_IDENTITY or ARM_USE_MSI."},
			"scope":                       schema.StringAttribute{Optional: true, Description: "OAuth scope for the AZExecute API. Defaults to AZEXECUTE_SCOPE or https://api.azexecute.com/.default."},
			"request_timeout_seconds":     schema.Int64Attribute{Optional: true, Description: "Timeout for each API request. Defaults to 30 seconds."},
		},
	}
}

func (p *azexecuteProvider) Configure(ctx context.Context, request provider.ConfigureRequest, response *provider.ConfigureResponse) {
	var config providerModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}

	endpoint := stringValue(config.Endpoint, envFirst("AZEXECUTE_ENDPOINT"), "https://api.azexecute.com")
	tenantID := stringValue(config.TenantID, envFirst("AZEXECUTE_TENANT_ID", "ARM_TENANT_ID", "AZURE_TENANT_ID"), "")
	clientID := stringValue(config.ClientID, envFirst("AZEXECUTE_CLIENT_ID", "ARM_CLIENT_ID", "AZURE_CLIENT_ID"), "")
	clientSecret := stringValue(config.ClientSecret, envFirst("AZEXECUTE_CLIENT_SECRET", "ARM_CLIENT_SECRET", "AZURE_CLIENT_SECRET"), "")
	certificatePath := stringValue(config.ClientCertificatePath, envFirst("AZEXECUTE_CLIENT_CERTIFICATE_PATH", "ARM_CLIENT_CERTIFICATE_PATH"), "")
	certificatePassword := stringValue(config.ClientCertificatePassword, envFirst("AZEXECUTE_CLIENT_CERTIFICATE_PASSWORD", "ARM_CLIENT_CERTIFICATE_PASSWORD"), "")
	accessToken := stringValue(config.AccessToken, envFirst("AZEXECUTE_ACCESS_TOKEN"), "")
	oidcToken := stringValue(config.OIDCToken, envFirst("AZEXECUTE_OIDC_TOKEN", "ARM_OIDC_TOKEN"), "")
	oidcTokenFilePath := stringValue(config.OIDCTokenFilePath, envFirst("AZEXECUTE_OIDC_TOKEN_FILE_PATH", "ARM_OIDC_TOKEN_FILE_PATH", "AZURE_FEDERATED_TOKEN_FILE"), "")
	oidcAudience := stringValue(config.OIDCAudience, envFirst("AZEXECUTE_OIDC_AUDIENCE", "ARM_OIDC_AUDIENCE"), "api://AzureADTokenExchange")
	scope := stringValue(config.Scope, envFirst("AZEXECUTE_SCOPE"), "https://api.azexecute.com/.default")
	timeoutSeconds := int64(30)
	if !config.RequestTimeout.IsNull() && !config.RequestTimeout.IsUnknown() {
		timeoutSeconds = config.RequestTimeout.ValueInt64()
	}
	if timeoutSeconds < 1 || timeoutSeconds > 600 {
		response.Diagnostics.AddError("Invalid request timeout", "request_timeout_seconds must be between 1 and 600.")
		return
	}

	var credential azcore.TokenCredential
	var err error
	if accessToken == "" {
		credential, err = buildCredential(ctx, credentialConfig{
			TenantID: tenantID, ClientID: clientID, ClientSecret: clientSecret,
			CertificatePath: certificatePath, CertificatePassword: certificatePassword,
			SendCertificateChain: boolValue(config.SendCertificateChain, false),
			UseOIDC:              boolValue(config.UseOIDC, false) || envBool("AZEXECUTE_USE_OIDC", "ARM_USE_OIDC"),
			OIDCToken:            oidcToken, OIDCTokenFilePath: oidcTokenFilePath, OIDCAudience: oidcAudience,
			UseManagedIdentity: boolValue(config.UseManagedIdentity, false) || envBool("AZEXECUTE_USE_MANAGED_IDENTITY", "ARM_USE_MSI"),
		})
		if err != nil {
			response.Diagnostics.AddError("Unable to configure Azure authentication", err.Error())
			return
		}
	}

	apiClient, err := azclient.New(endpoint, scope, accessToken, credential, time.Duration(timeoutSeconds)*time.Second)
	if err != nil {
		response.Diagnostics.AddError("Unable to configure AZExecute client", err.Error())
		return
	}

	response.DataSourceData = apiClient
	response.ResourceData = apiClient
}

func envBool(names ...string) bool {
	for _, name := range names {
		switch os.Getenv(name) {
		case "1", "true", "TRUE", "True":
			return true
		}
	}
	return false
}

func (p *azexecuteProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{NewApplicationResource, NewApplicationRequestResource}
}

func (p *azexecuteProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{NewCapabilitiesDataSource, NewApplicationDataSource}
}

func stringValue(value types.String, environment, fallback string) string {
	if !value.IsNull() && !value.IsUnknown() && value.ValueString() != "" {
		return value.ValueString()
	}
	if environment != "" {
		return environment
	}
	return fallback
}

func envFirst(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func clientFromProviderData(data any, diagnostics *diag.Diagnostics) *azclient.Client {
	if data == nil {
		return nil
	}
	client, ok := data.(*azclient.Client)
	if !ok {
		diagnostics.AddError("Unexpected provider data", "The provider returned an unexpected API client type. This is a provider bug.")
		return nil
	}
	return client
}
