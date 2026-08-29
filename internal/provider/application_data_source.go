package provider

import (
	"context"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &applicationDataSource{}
var _ datasource.DataSourceWithConfigure = &applicationDataSource{}

type applicationDataSource struct{ client *azclient.Client }
type applicationDataSourceModel struct {
	ID                    types.String `tfsdk:"id"`
	DisplayName           types.String `tfsdk:"display_name"`
	Description           types.String `tfsdk:"description"`
	Status                types.String `tfsdk:"status"`
	StatusReason          types.String `tfsdk:"status_reason"`
	RequestID             types.Int64  `tfsdk:"request_id"`
	ApplicationEntityID   types.String `tfsdk:"application_entity_id"`
	ApplicationID         types.String `tfsdk:"application_id"`
	ApplicationObjectID   types.String `tfsdk:"application_object_id"`
	BusinessJustification types.String `tfsdk:"business_justification"`
	OwnerObjectIDs        types.Set    `tfsdk:"owner_object_ids"`
}

func NewApplicationDataSource() datasource.DataSource { return &applicationDataSource{} }
func (d *applicationDataSource) Metadata(_ context.Context, request datasource.MetadataRequest, response *datasource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_application"
}
func (d *applicationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, response *datasource.SchemaResponse) {
	response.Schema = schema.Schema{Description: "Reads an application created through the AZExecute Terraform API by its stable resource UUID.", Attributes: map[string]schema.Attribute{
		"id": schema.StringAttribute{Required: true}, "display_name": schema.StringAttribute{Computed: true}, "description": schema.StringAttribute{Computed: true},
		"status": schema.StringAttribute{Computed: true}, "status_reason": schema.StringAttribute{Computed: true}, "request_id": schema.Int64Attribute{Computed: true},
		"application_entity_id": schema.StringAttribute{Computed: true, Description: "AZExecute application entity UUID."}, "application_id": schema.StringAttribute{Computed: true}, "application_object_id": schema.StringAttribute{Computed: true},
		"business_justification": schema.StringAttribute{Computed: true},
		"owner_object_ids":       schema.SetAttribute{Computed: true, ElementType: types.StringType},
	}}
}
func (d *applicationDataSource) Configure(_ context.Context, request datasource.ConfigureRequest, response *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(request.ProviderData, &response.Diagnostics)
}
func (d *applicationDataSource) Read(ctx context.Context, request datasource.ReadRequest, response *datasource.ReadResponse) {
	var state applicationDataSourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || d.client == nil {
		return
	}
	result, err := d.client.GetApplication(ctx, state.ID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Unable to read AZExecute application", err.Error())
		return
	}
	state.DisplayName = types.StringValue(result.DisplayName)
	state.Description = stringTypeFromPointer(result.Description)
	state.Status = types.StringValue(result.Status)
	state.StatusReason = stringTypeFromPointer(result.StatusReason)
	state.RequestID = types.Int64Value(result.RequestID)
	state.ApplicationEntityID = stringTypeFromPointer(result.ApplicationEntityID)
	state.ApplicationID = stringTypeFromPointer(result.ApplicationID)
	state.ApplicationObjectID = stringTypeFromPointer(result.ApplicationObjectID)
	state.BusinessJustification = types.StringValue(result.Metadata.BusinessJustification)
	var ownerDiagnostics diag.Diagnostics
	state.OwnerObjectIDs, ownerDiagnostics = types.SetValueFrom(ctx, types.StringType, result.OwnerObjectIDs)
	response.Diagnostics.Append(ownerDiagnostics...)
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}
