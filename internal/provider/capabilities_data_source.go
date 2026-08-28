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
	ID                             types.String `tfsdk:"id"`
	APIVersion                     types.String `tfsdk:"api_version"`
	Enabled                        types.Bool   `tfsdk:"enabled"`
	AllowApplicationCreation       types.Bool   `tfsdk:"allow_application_creation"`
	AllowApplicationDeletion       types.Bool   `tfsdk:"allow_application_deletion"`
	AllowAPIPermissionRequests     types.Bool   `tfsdk:"allow_api_permission_requests"`
	AllowRegistrationConfiguration types.Bool   `tfsdk:"allow_registration_configuration"`
	UseApplicationRequestFlow      types.Bool   `tfsdk:"use_application_request_flow"`
	UseAPIPermissionRequestFlow    types.Bool   `tfsdk:"use_api_permission_request_flow"`
	IncludedMetadataFields         types.Set    `tfsdk:"included_metadata_fields"`
	RequiredMetadataFields         types.Set    `tfsdk:"required_metadata_fields"`
}

func NewCapabilitiesDataSource() datasource.DataSource { return &capabilitiesDataSource{} }
func (d *capabilitiesDataSource) Metadata(_ context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_capabilities"
}
func (d *capabilitiesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Description: "Reads the tenant policy enforced by the AZExecute Terraform API.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Computed: true}, "api_version": schema.StringAttribute{Computed: true}, "enabled": schema.BoolAttribute{Computed: true},
		"allow_application_creation": schema.BoolAttribute{Computed: true}, "allow_application_deletion": schema.BoolAttribute{Computed: true},
		"allow_api_permission_requests": schema.BoolAttribute{Computed: true}, "allow_registration_configuration": schema.BoolAttribute{Computed: true},
		"use_application_request_flow": schema.BoolAttribute{Computed: true}, "use_api_permission_request_flow": schema.BoolAttribute{Computed: true},
		"included_metadata_fields": schema.SetAttribute{Computed: true, ElementType: types.StringType, Description: "Metadata fields enabled by the tenant."},
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
	state := capabilitiesDataSourceModel{ID: types.StringValue("tenant"), APIVersion: types.StringValue(result.APIVersion), Enabled: types.BoolValue(result.Enabled), AllowApplicationCreation: types.BoolValue(result.AllowApplicationCreation), AllowApplicationDeletion: types.BoolValue(result.AllowApplicationDeletion), AllowAPIPermissionRequests: types.BoolValue(result.AllowAPIPermissionRequests), AllowRegistrationConfiguration: types.BoolValue(result.AllowRegistrationConfiguration), UseApplicationRequestFlow: types.BoolValue(result.UseApplicationRequestFlow), UseAPIPermissionRequestFlow: types.BoolValue(result.UseAPIPermissionRequestFlow), IncludedMetadataFields: included, RequiredMetadataFields: required}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}
