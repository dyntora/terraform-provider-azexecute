package provider

import (
	"context"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &capabilitiesDataSource{}
var _ datasource.DataSourceWithConfigure = &capabilitiesDataSource{}

type capabilitiesDataSource struct{ client *azclient.Client }
type capabilitiesDataSourceModel struct {
	SupportsPermissionRequestUpdates         types.Bool   `tfsdk:"supports_permission_request_updates"`
	AutoApproveDelegatedPermissions          types.Bool   `tfsdk:"auto_approve_delegated_permissions"`
	GrantAdminConsentForDelegatedPermissions types.Bool   `tfsdk:"grant_admin_consent_for_delegated_permissions"`
	PreservesHiddenMetadata                  types.Bool   `tfsdk:"preserves_hidden_metadata"`
	SupportsApplicationDetailsUpdates        types.Bool   `tfsdk:"supports_application_details_updates"`
	MinimumAdditionalOwners                  types.Int64  `tfsdk:"minimum_additional_owners"`
	SupportsRegistrationRequests             types.Bool   `tfsdk:"supports_registration_requests"`
	ID                                       types.String `tfsdk:"id"`
	APIVersion                               types.String `tfsdk:"api_version"`
	Enabled                                  types.Bool   `tfsdk:"enabled"`
	AllowApplicationCreation                 types.Bool   `tfsdk:"allow_application_creation"`
	AllowApplicationDeletion                 types.Bool   `tfsdk:"allow_application_deletion"`
	AllowAPIPermissionRequests               types.Bool   `tfsdk:"allow_api_permission_requests"`
	AllowRegistrationConfiguration           types.Bool   `tfsdk:"allow_registration_configuration"`
	UseApplicationRequestFlow                types.Bool   `tfsdk:"use_application_request_flow"`
	UseAPIPermissionRequestFlow              types.Bool   `tfsdk:"use_api_permission_request_flow"`
	IncludedMetadataFields                   types.Set    `tfsdk:"included_metadata_fields"`
	RequiredMetadataFields                   types.Set    `tfsdk:"required_metadata_fields"`
}

func NewCapabilitiesDataSource() datasource.DataSource { return &capabilitiesDataSource{} }
func (d *capabilitiesDataSource) Metadata(_ context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_capabilities"
}
func (d *capabilitiesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Description: "Reads the tenant policy enforced by the AZExecute Terraform API.", Attributes: map[string]schema.Attribute{
		"supports_permission_request_updates":           schema.BoolAttribute{Computed: true, Description: "Whether permission requests can change in place without replacing the application."},
		"auto_approve_delegated_permissions":            schema.BoolAttribute{Computed: true, Description: "Automatically approve Terraform delegated requests only when all scopes are enabled and defined as user-consentable. Null when the server does not expose this policy."},
		"grant_admin_consent_for_delegated_permissions": schema.BoolAttribute{Computed: true, Description: "Grant tenant-wide consent after approval of eligible Terraform delegated scopes, independently of automatic approval. Null when the server does not expose this policy."},
		"preserves_hidden_metadata":                     schema.BoolAttribute{Computed: true, Description: "Whether hidden metadata fields retain supplied values. False or absent on older APIs requires visibility checks before writes."},
		"supports_application_details_updates":          schema.BoolAttribute{Computed: true},
		"minimum_additional_owners":                     schema.Int64Attribute{Computed: true, Description: "Minimum additional Entra user owners, excluding the original requester. Service principals do not count."},
		"supports_registration_requests":                schema.BoolAttribute{Computed: true},
		"id":                                            schema.StringAttribute{Computed: true}, "api_version": schema.StringAttribute{Computed: true}, "enabled": schema.BoolAttribute{Computed: true},
		"allow_application_creation": schema.BoolAttribute{Computed: true}, "allow_application_deletion": schema.BoolAttribute{Computed: true},
		"allow_api_permission_requests": schema.BoolAttribute{Computed: true}, "allow_registration_configuration": schema.BoolAttribute{Computed: true},
		"use_application_request_flow": schema.BoolAttribute{Computed: true}, "use_api_permission_request_flow": schema.BoolAttribute{Computed: true, Description: "Whether application permissions and authorized-client requests require review. Delegated scopes use their own policy."},
		"included_metadata_fields": schema.SetAttribute{Computed: true, ElementType: types.StringType, Description: "Metadata fields visible in tenant forms. Visibility does not prohibit storing valid values."},
		"required_metadata_fields": schema.SetAttribute{Computed: true, ElementType: types.StringType, Description: "Metadata fields required by the tenant."},
	}}
}
func (d *capabilitiesDataSource) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(request.ProviderData, &response.Diagnostics)
}
func (d *capabilitiesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, response *datasource.ReadResponse) {
	if d.client == nil {
		return
	}
	result, err := d.client.Capabilities(ctx)
	if err != nil {
		response.Diagnostics.AddError("Unable to read AZExecute Terraform capabilities", err.Error())
		return
	}
	included, includedDiagnostics := types.SetValueFrom(ctx, types.StringType, result.IncludedMetadataFields)
	response.Diagnostics.Append(includedDiagnostics...)
	required, requiredDiagnostics := types.SetValueFrom(ctx, types.StringType, result.RequiredMetadataFields)
	response.Diagnostics.Append(requiredDiagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	state := capabilitiesDataSourceModel{SupportsApplicationDetailsUpdates: types.BoolValue(result.SupportsApplicationDetailsUpdates), MinimumAdditionalOwners: types.Int64Value(result.MinimumAdditionalOwners), SupportsRegistrationRequests: types.BoolValue(result.SupportsRegistrationRequests), ID: types.StringValue("tenant"), APIVersion: types.StringValue(result.APIVersion), Enabled: types.BoolValue(result.Enabled), AllowApplicationCreation: types.BoolValue(result.AllowApplicationCreation), AllowApplicationDeletion: types.BoolValue(result.AllowApplicationDeletion), AllowAPIPermissionRequests: types.BoolValue(result.AllowAPIPermissionRequests), AllowRegistrationConfiguration: types.BoolValue(result.AllowRegistrationConfiguration), UseApplicationRequestFlow: types.BoolValue(result.UseApplicationRequestFlow), UseAPIPermissionRequestFlow: types.BoolValue(result.UseAPIPermissionRequestFlow), IncludedMetadataFields: included, RequiredMetadataFields: required}
	state.AutoApproveDelegatedPermissions = types.BoolPointerValue(result.AutoApproveDelegatedPermissions)
	state.GrantAdminConsentForDelegatedPermissions = types.BoolPointerValue(result.GrantAdminConsentForDelegatedPermissions)
	state.PreservesHiddenMetadata = types.BoolValue(result.PreservesHiddenMetadata)
	state.SupportsPermissionRequestUpdates = types.BoolValue(result.SupportsPermissionRequestUpdates)
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}
