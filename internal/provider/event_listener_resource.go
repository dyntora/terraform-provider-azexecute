package provider

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &eventListenerResource{}
var _ resource.ResourceWithConfigure = &eventListenerResource{}
var _ resource.ResourceWithImportState = &eventListenerResource{}
var _ resource.ResourceWithValidateConfig = &eventListenerResource{}

type eventListenerResource struct{ client *azclient.Client }
type eventListenerModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	EventType           types.String `tfsdk:"event_type"`
	ActionType          types.String `tfsdk:"action_type"`
	IsEnabled           types.Bool   `tfsdk:"enabled"`
	ExecutionOrder      types.Int64  `tfsdk:"execution_order"`
	ApplicationEntityID types.String `tfsdk:"application_entity_id"`
	BroadListener       types.Bool   `tfsdk:"broad_listener"`
	CredentialType      types.String `tfsdk:"credential_type"`
	AutomationTaskID    types.Int64  `tfsdk:"automation_task_id"`
	Parameters          types.Map    `tfsdk:"parameters"`
	TopdeskSettings     types.Object `tfsdk:"topdesk_settings"`
	AuthorizedByUserID  types.String `tfsdk:"authorized_by_user_id"`
}

func NewEventListenerResource() resource.Resource { return &eventListenerResource{} }
func (r *eventListenerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_event_listener"
}
func (r *eventListenerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manages an event listener through the public API, with application ownership and task access enforced by the server. Renewed credential material is never fetched: use protected catalog tokens. Literal parameter values remain in Terraform state; protect the state backend.", Attributes: map[string]schema.Attribute{
		"id":                    schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Description: "Listener ID. Import using this positive integer ID."},
		"name":                  schema.StringAttribute{Required: true},
		"description":           schema.StringAttribute{Optional: true},
		"event_type":            schema.StringAttribute{Required: true, Description: "Event name from GET /api/v1/EventListeners/Catalog."},
		"action_type":           schema.StringAttribute{Computed: true, Description: "AutomationTask or TopdeskCreateIncident, inferred from action settings."},
		"enabled":               schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"execution_order":       schema.Int64Attribute{Optional: true, Computed: true, Default: int64default.StaticInt64(100)},
		"application_entity_id": schema.StringAttribute{Optional: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "AZExecute application entity UUID, exported as application_entity_id by azexecute_application. Required unless broad_listener is true. Not the Terraform resource ID or Entra client/object ID."},
		"broad_listener":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Explicitly opt into listening across applications. Requires Operator/TenantAdmin; omit application_entity_id."},
		"credential_type":       schema.StringAttribute{Optional: true, Description: "Secret or Certificate; omit for all credentials. Protected renewal mappings require an explicit value."},
		"automation_task_id":    schema.Int64Attribute{Optional: true, Description: "Task to run. Choose this or topdesk_settings. Task approvals and caller access still apply."},
		"parameters":            schema.MapAttribute{Optional: true, Computed: true, ElementType: types.StringType, Default: mapdefault.StaticValue(types.MapValueMust(types.StringType, map[string]attr.Value{})), Sensitive: true, Description: "Visible task input names to values/templates. For credentials use an entire protected token, e.g. {{ renewed_secret }}. This stores the reference, never the renewed credential."},
		"topdesk_settings":      schema.SingleNestedAttribute{Optional: true, Attributes: topdeskSchema(), Description: "Operator-only TOPdesk action using an existing integration; mutually exclusive with automation_task_id."},
		"authorized_by_user_id": schema.StringAttribute{Computed: true, Description: "Owner who last authorized the self-service rule; null means operator-managed. Assigned by the API, never configurable."},
	}}
}
func (r *eventListenerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}
func (r *eventListenerResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config eventListenerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(validateEventListener(config)...)
	}
}
func validateEventListener(m eventListenerModel) diag.Diagnostics {
	var d diag.Diagnostics
	if !m.Name.IsUnknown() && (strings.TrimSpace(m.Name.ValueString()) == "" || len([]rune(m.Name.ValueString())) > 160 || m.Name.ValueString() != strings.TrimSpace(m.Name.ValueString())) {
		d.AddError("Invalid listener name", "name must contain 1 to 160 characters with no surrounding whitespace.")
	}
	if !m.Description.IsUnknown() && (len([]rune(m.Description.ValueString())) > 500 || m.Description.ValueString() != strings.TrimSpace(m.Description.ValueString())) {
		d.AddError("Invalid description", "description must contain at most 500 characters with no surrounding whitespace.")
	}
	if !m.EventType.IsUnknown() && strings.TrimSpace(m.EventType.ValueString()) == "" {
		d.AddError("Missing event type", "Use an event_type from the event listener catalog.")
	}
	if !m.ApplicationEntityID.IsUnknown() && !m.BroadListener.IsUnknown() {
		hasApp := !m.ApplicationEntityID.IsNull() && m.ApplicationEntityID.ValueString() != ""
		if hasApp == m.BroadListener.ValueBool() {
			d.AddError("Invalid listener scope", "Set application_entity_id, or explicitly set broad_listener = true without an application ID.")
		}
		if hasApp && !uuidPattern.MatchString(m.ApplicationEntityID.ValueString()) {
			d.AddError("Invalid application entity ID", "application_entity_id must be an AZExecute application entity UUID.")
		}
	}
	if !m.AutomationTaskID.IsUnknown() && !m.TopdeskSettings.IsUnknown() {
		if m.AutomationTaskID.IsNull() == m.TopdeskSettings.IsNull() {
			d.AddError("Invalid listener action", "Set exactly one of automation_task_id or topdesk_settings.")
		}
		if !m.AutomationTaskID.IsNull() && (m.AutomationTaskID.ValueInt64() <= 0 || m.AutomationTaskID.ValueInt64() > 2147483647) {
			d.AddError("Invalid task ID", "automation_task_id must be a positive 32-bit integer.")
		}
	}
	if !m.ExecutionOrder.IsUnknown() && (m.ExecutionOrder.ValueInt64() < 0 || m.ExecutionOrder.ValueInt64() > 10000) {
		d.AddError("Invalid execution order", "execution_order must be between 0 and 10000.")
	}
	if !m.CredentialType.IsNull() && !m.CredentialType.IsUnknown() && m.CredentialType.ValueString() != "Secret" && m.CredentialType.ValueString() != "Certificate" {
		d.AddError("Invalid credential type", "credential_type must be Secret or Certificate, or omitted.")
	}
	if !m.Parameters.IsUnknown() && !m.Parameters.IsNull() {
		if !m.TopdeskSettings.IsNull() && !m.TopdeskSettings.IsUnknown() && len(m.Parameters.Elements()) > 0 {
			d.AddError("Invalid action inputs", "parameters can only be used with automation_task_id.")
		}
		for name, raw := range m.Parameters.Elements() {
			if strings.TrimSpace(name) == "" || name != strings.TrimSpace(name) {
				d.AddError("Invalid input name", "Parameter names cannot be empty or contain surrounding whitespace.")
			}
			value := raw.(types.String)
			if value.IsNull() {
				d.AddError("Invalid input value", "Parameter values cannot be null; omit the input or use an empty string.")
			}
			if !value.IsUnknown() && strings.HasPrefix(strings.ToLower(value.ValueString()), "azx-credential:") {
				d.AddError("Runtime credential cannot be configured", "Use a protected variable token from the catalog instead of encrypted runtime credential material.")
			}
		}
	}
	return d
}
func (r *eventListenerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m eventListenerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	input, d := eventListenerInput(ctx, m)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.CreateEventListener(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create event listener", err.Error()+". If the result was ambiguous, check the application's listeners and import an existing listener before retrying.")
		return
	}
	resp.Diagnostics.Append(setEventListenerModel(ctx, &m, result)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *eventListenerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m eventListenerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	id, err := eventListenerID(m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid listener ID", err.Error())
		return
	}
	result, err := r.client.GetEventListener(ctx, id)
	if azclient.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read event listener", err.Error())
		return
	}
	resp.Diagnostics.Append(setEventListenerModel(ctx, &m, result)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *eventListenerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m eventListenerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	id, err := eventListenerID(m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid listener ID", err.Error())
		return
	}
	input, d := eventListenerInput(ctx, m)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.UpdateEventListener(ctx, id, input)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update event listener", err.Error())
		return
	}
	resp.Diagnostics.Append(setEventListenerModel(ctx, &m, result)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *eventListenerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m eventListenerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	id, err := eventListenerID(m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid listener ID", err.Error())
		return
	}
	if err := r.client.DeleteEventListener(ctx, id); err != nil && !azclient.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete event listener", err.Error())
	}
}
func (r *eventListenerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := eventListenerID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid listener import ID", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), strconv.FormatInt(id, 10))...)
}
func eventListenerID(value string) (int64, error) {
	id, err := strconv.ParseInt(value, 10, 32)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("use a positive integer event listener ID")
	}
	return id, nil
}
func eventListenerInput(ctx context.Context, m eventListenerModel) (azclient.EventListener, diag.Diagnostics) {
	d := validateEventListener(m)
	input := azclient.EventListener{Name: m.Name.ValueString(), Description: m.Description.ValueStringPointer(), EventType: m.EventType.ValueString(), IsEnabled: m.IsEnabled.ValueBool(), ExecutionOrder: m.ExecutionOrder.ValueInt64(), ApplicationEntityID: m.ApplicationEntityID.ValueStringPointer(), CredentialType: m.CredentialType.ValueStringPointer(), ActionType: "AutomationTask"}
	if !m.AutomationTaskID.IsNull() {
		values := map[string]string{}
		d.Append(m.Parameters.ElementsAs(ctx, &values, false)...)
		keys := make([]string, 0, len(values))
		for key := range values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		input.AutomationTaskSettings = &azclient.EventListenerTaskSettings{AutomationTaskID: m.AutomationTaskID.ValueInt64(), Parameters: []azclient.EventListenerParameter{}}
		for _, key := range keys {
			input.AutomationTaskSettings.Parameters = append(input.AutomationTaskSettings.Parameters, azclient.EventListenerParameter{Name: key, Value: values[key]})
		}
	} else if !m.TopdeskSettings.IsNull() {
		input.ActionType = "TopdeskCreateIncident"
		input.TopdeskSettings = map[string]any{}
		for key, value := range m.TopdeskSettings.Attributes() {
			if value.IsNull() || value.IsUnknown() || key == "integration_name" {
				continue
			}
			if key == "integration_id" {
				input.TopdeskSettings["topdeskIntegrationId"] = value.(types.Int64).ValueInt64()
			} else {
				input.TopdeskSettings[topdeskFields[key]] = value.(types.String).ValueString()
			}
		}
	}
	return input, d
}
func setEventListenerModel(ctx context.Context, m *eventListenerModel, source *azclient.EventListener) diag.Diagnostics {
	var d diag.Diagnostics
	m.ID = types.StringValue(strconv.FormatInt(source.ID, 10))
	m.Name = types.StringValue(source.Name)
	if source.Description != nil || m.Description.IsNull() || m.Description.ValueString() != "" {
		m.Description = stringTypeFromPointer(source.Description)
	}
	m.EventType = types.StringValue(source.EventType)
	m.ActionType = types.StringValue(source.ActionType)
	m.IsEnabled = types.BoolValue(source.IsEnabled)
	m.ExecutionOrder = types.Int64Value(source.ExecutionOrder)
	if source.ApplicationEntityID == nil || !strings.EqualFold(m.ApplicationEntityID.ValueString(), *source.ApplicationEntityID) {
		m.ApplicationEntityID = stringTypeFromPointer(source.ApplicationEntityID)
	}
	m.BroadListener = types.BoolValue(source.ApplicationEntityID == nil)
	m.CredentialType = stringTypeFromPointer(source.CredentialType)
	m.AuthorizedByUserID = stringTypeFromPointer(source.AuthorizedByUserID)
	m.AutomationTaskID = types.Int64Null()
	values := map[string]string{}
	if source.AutomationTaskSettings != nil {
		m.AutomationTaskID = types.Int64Value(source.AutomationTaskSettings.AutomationTaskID)
		for _, p := range source.AutomationTaskSettings.Parameters {
			values[p.Name] = p.Value
		}
	}
	var md diag.Diagnostics
	m.Parameters, md = types.MapValueFrom(ctx, types.StringType, values)
	d.Append(md...)
	m.TopdeskSettings = types.ObjectNull(topdeskTypes())
	if source.TopdeskSettings != nil {
		attrs := map[string]attr.Value{}
		for key, apiName := range topdeskFields {
			value := source.TopdeskSettings[apiName]
			if key == "integration_id" {
				if number, ok := value.(float64); ok {
					attrs[key] = types.Int64Value(int64(number))
				} else {
					attrs[key] = types.Int64Null()
				}
			} else if text, ok := value.(string); ok {
				attrs[key] = types.StringValue(text)
			} else {
				attrs[key] = types.StringNull()
			}
		}
		m.TopdeskSettings, md = types.ObjectValue(topdeskTypes(), attrs)
		d.Append(md...)
	}
	return d
}

func topdeskTypes() map[string]attr.Type {
	result := map[string]attr.Type{}
	for key := range topdeskFields {
		if key == "integration_id" {
			result[key] = types.Int64Type
		} else {
			result[key] = types.StringType
		}
	}
	return result
}
func topdeskSchema() map[string]schema.Attribute {
	result := map[string]schema.Attribute{}
	for key := range topdeskFields {
		if key == "integration_id" {
			result[key] = schema.Int64Attribute{Required: true}
		} else {
			result[key] = schema.StringAttribute{Optional: key != "integration_name", Computed: true}
		}
	}
	return result
}

var topdeskFields = map[string]string{
	"integration_id":          "topdeskIntegrationId",
	"integration_name":        "topdeskIntegrationName",
	"operation_type":          "topdeskOperationType",
	"incident_reference_name": "topdeskIncidentReferenceName",
	"incident_number_or_id":   "topdeskIncidentNumberOrId",
	"brief_description":       "topdeskBriefDescription",
	"request_text":            "topdeskRequestText",
	"action_text":             "topdeskActionText",
	"caller_id":               "topdeskCallerId",
	"caller_display_name":     "topdeskCallerDisplayName",
	"caller_lookup_value":     "topdeskCallerLookupValue",
	"caller_dynamic_name":     "topdeskCallerDynamicName",
	"caller_email":            "topdeskCallerEmail",
	"entry_type_id":           "topdeskEntryTypeId",
	"entry_type_name":         "topdeskEntryTypeName",
	"incident_line_status":    "topdeskIncidentLineStatus",
	"processing_status_id":    "topdeskProcessingStatusId",
	"processing_status_name":  "topdeskProcessingStatusName",
	"operator_group_id":       "topdeskOperatorGroupId",
	"operator_group_name":     "topdeskOperatorGroupName",
	"operator_id":             "topdeskOperatorId",
	"operator_name":           "topdeskOperatorName",
	"category_id":             "topdeskCategoryId",
	"category_name":           "topdeskCategoryName",
	"subcategory_id":          "topdeskSubcategoryId",
	"subcategory_name":        "topdeskSubcategoryName",
}
