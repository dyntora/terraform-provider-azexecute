package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &applicationResource{}
var _ resource.ResourceWithConfigure = &applicationResource{}
var _ resource.ResourceWithImportState = &applicationResource{}

type applicationResource struct{ client *azclient.Client }

type applicationResourceModel struct {
	ID                               types.String `tfsdk:"id"`
	DisplayName                      types.String `tfsdk:"display_name"`
	Description                      types.String `tfsdk:"description"`
	BusinessJustification            types.String `tfsdk:"business_justification"`
	TechnicalRequirements            types.String `tfsdk:"technical_requirements"`
	IntendedAudience                 types.String `tfsdk:"intended_audience"`
	DataAccessRequirements           types.String `tfsdk:"data_access_requirements"`
	ComplianceNotes                  types.String `tfsdk:"compliance_notes"`
	ExpectedGoLiveDate               types.String `tfsdk:"expected_go_live_date"`
	ProjectName                      types.String `tfsdk:"project_name"`
	DepartmentOwner                  types.String `tfsdk:"department_owner"`
	BusinessCriticality              types.Int64  `tfsdk:"business_criticality"`
	RequiresElevatedPermissions      types.Bool   `tfsdk:"requires_elevated_permissions"`
	ElevatedPermissionsJustification types.String `tfsdk:"elevated_permissions_justification"`
	Environment                      types.String `tfsdk:"environment"`
	ContactEmail                     types.String `tfsdk:"contact_email"`
	ContactPhone                     types.String `tfsdk:"contact_phone"`
	APIPermissionRequests            types.Set    `tfsdk:"api_permission_request"`
	ConfigureRegistration            types.Bool   `tfsdk:"configure_registration"`
	SignInAudience                   types.String `tfsdk:"sign_in_audience"`
	IsFallbackPublicClient           types.Bool   `tfsdk:"is_fallback_public_client"`
	IdentifierURIs                   types.Set    `tfsdk:"identifier_uris"`
	WebHomePageURL                   types.String `tfsdk:"web_home_page_url"`
	WebLogoutURL                     types.String `tfsdk:"web_logout_url"`
	WebEnableAccessTokenIssuance     types.Bool   `tfsdk:"web_enable_access_token_issuance"`
	WebEnableIDTokenIssuance         types.Bool   `tfsdk:"web_enable_id_token_issuance"`
	WebRedirectURIs                  types.Set    `tfsdk:"web_redirect_uris"`
	SpaRedirectURIs                  types.Set    `tfsdk:"spa_redirect_uris"`
	PublicClientRedirectURIs         types.Set    `tfsdk:"public_client_redirect_uris"`
	RequestedAccessTokenVersion      types.Int64  `tfsdk:"requested_access_token_version"`
	PollIntervalSeconds              types.Int64  `tfsdk:"poll_interval_seconds"`
	CreateTimeoutMinutes             types.Int64  `tfsdk:"create_timeout_minutes"`
	Status                           types.String `tfsdk:"status"`
	StatusReason                     types.String `tfsdk:"status_reason"`
	RequestID                        types.Int64  `tfsdk:"request_id"`
	ApplicationEntityID              types.Int64  `tfsdk:"application_entity_id"`
	ApplicationID                    types.String `tfsdk:"application_id"`
	ApplicationObjectID              types.String `tfsdk:"application_object_id"`
}

type permissionRequestModel struct {
	TargetType                   types.String `tfsdk:"target_type"`
	TargetApplicationEntityID    types.Int64  `tfsdk:"target_application_entity_id"`
	TargetExternalAPIAppID       types.String `tfsdk:"target_external_api_app_id"`
	TargetExternalAPIDisplayName types.String `tfsdk:"target_external_api_display_name"`
	GrantType                    types.String `tfsdk:"grant_type"`
	Justification                types.String `tfsdk:"justification"`
	Permissions                  types.Set    `tfsdk:"permission"`
}

type permissionModel struct {
	ID                   types.String `tfsdk:"id"`
	DisplayName          types.String `tfsdk:"display_name"`
	Value                types.String `tfsdk:"value"`
	RequiresAdminConsent types.Bool   `tfsdk:"requires_admin_consent"`
}

func NewApplicationResource() resource.Resource { return &applicationResource{} }

func (r *applicationResource) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_application"
}

func (r *applicationResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	replaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	response.Schema = schema.Schema{
		Description: "Creates an AZExecute-governed Microsoft Entra application registration, its metadata, and optional API permission requests.",
		Attributes: map[string]schema.Attribute{
			"id":                                 schema.StringAttribute{Computed: true, Description: "Stable Terraform resource UUID used for API idempotency."},
			"display_name":                       schema.StringAttribute{Required: true, PlanModifiers: replaceString},
			"description":                        schema.StringAttribute{Optional: true, PlanModifiers: replaceString},
			"business_justification":             schema.StringAttribute{Required: true},
			"technical_requirements":             schema.StringAttribute{Optional: true},
			"intended_audience":                  schema.StringAttribute{Optional: true},
			"data_access_requirements":           schema.StringAttribute{Optional: true},
			"compliance_notes":                   schema.StringAttribute{Optional: true},
			"expected_go_live_date":              schema.StringAttribute{Optional: true, Description: "RFC 3339 date or timestamp."},
			"project_name":                       schema.StringAttribute{Optional: true},
			"department_owner":                   schema.StringAttribute{Optional: true},
			"business_criticality":               schema.Int64Attribute{Optional: true, Computed: true, Description: "Value from 1 to 5; defaults to 3."},
			"requires_elevated_permissions":      schema.BoolAttribute{Optional: true, Computed: true},
			"elevated_permissions_justification": schema.StringAttribute{Optional: true},
			"environment":                        schema.StringAttribute{Optional: true},
			"contact_email":                      schema.StringAttribute{Optional: true},
			"contact_phone":                      schema.StringAttribute{Optional: true},
			"configure_registration":             schema.BoolAttribute{Optional: true, Description: "Set true to manage the registration fields below."},
			"sign_in_audience":                   schema.StringAttribute{Optional: true, Computed: true},
			"is_fallback_public_client":          schema.BoolAttribute{Optional: true, Computed: true},
			"identifier_uris":                    schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"web_home_page_url":                  schema.StringAttribute{Optional: true, Computed: true},
			"web_logout_url":                     schema.StringAttribute{Optional: true, Computed: true},
			"web_enable_access_token_issuance":   schema.BoolAttribute{Optional: true, Computed: true},
			"web_enable_id_token_issuance":       schema.BoolAttribute{Optional: true, Computed: true},
			"web_redirect_uris":                  schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"spa_redirect_uris":                  schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"public_client_redirect_uris":        schema.SetAttribute{Optional: true, Computed: true, ElementType: types.StringType},
			"requested_access_token_version":     schema.Int64Attribute{Optional: true, Computed: true},
			"poll_interval_seconds":              schema.Int64Attribute{Optional: true, Description: "Approval/provisioning poll interval; defaults to 5."},
			"create_timeout_minutes":             schema.Int64Attribute{Optional: true, Description: "Maximum wait for approval and provisioning; defaults to 60."},
			"status":                             schema.StringAttribute{Computed: true},
			"status_reason":                      schema.StringAttribute{Computed: true},
			"request_id":                         schema.Int64Attribute{Computed: true},
			"application_entity_id":              schema.Int64Attribute{Computed: true},
			"application_id":                     schema.StringAttribute{Computed: true, Description: "Microsoft Entra application (client) ID."},
			"application_object_id":              schema.StringAttribute{Computed: true, Description: "Microsoft Entra application object ID."},
		},
		Blocks: map[string]schema.Block{
			"api_permission_request": schema.SetNestedBlock{
				Description:   "API permissions requested as part of application provisioning. Changes replace the application in v0.1.",
				PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"target_type":                      schema.StringAttribute{Required: true, Description: "ExternalApi or InternalApplication."},
						"target_application_entity_id":     schema.Int64Attribute{Optional: true},
						"target_external_api_app_id":       schema.StringAttribute{Optional: true},
						"target_external_api_display_name": schema.StringAttribute{Optional: true},
						"grant_type":                       schema.StringAttribute{Required: true, Description: "AppRole, DelegatedScope, or AuthorizedClient."},
						"justification":                    schema.StringAttribute{Optional: true},
					},
					Blocks: map[string]schema.Block{
						"permission": schema.SetNestedBlock{NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
							"id":                     schema.StringAttribute{Required: true},
							"display_name":           schema.StringAttribute{Optional: true},
							"value":                  schema.StringAttribute{Optional: true},
							"requires_admin_consent": schema.BoolAttribute{Optional: true},
						}}},
					},
				},
			},
		},
	}
}

func (r *applicationResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	r.client = clientFromProviderData(request.ProviderData, &response.Diagnostics)
}

func (r *applicationResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan applicationResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}

	resourceID, err := newUUID()
	if err != nil {
		response.Diagnostics.AddError("Unable to generate resource ID", err.Error())
		return
	}
	create, err := createRequestFromModel(ctx, plan, resourceID)
	if err != nil {
		response.Diagnostics.AddError("Invalid application configuration", err.Error())
		return
	}

	result, err := r.client.CreateApplication(ctx, create)
	if err != nil {
		response.Diagnostics.AddError("Unable to create AZExecute application", err.Error())
		return
	}
	desired := plan
	// Persist the server's stable identity before waiting. Terraform can then resume the
	// same request after cancellation, timeout, or a provider crash instead of creating
	// a second application.
	mapApplicationToModel(ctx, result, &plan, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}

	pollSeconds := modelIntOr(desired.PollIntervalSeconds, 5)
	timeoutMinutes := modelIntOr(desired.CreateTimeoutMinutes, 60)
	if pollSeconds < 1 || pollSeconds > 300 || timeoutMinutes < 1 || timeoutMinutes > 1440 {
		response.Diagnostics.AddError("Invalid polling configuration", "poll_interval_seconds must be 1-300 and create_timeout_minutes must be 1-1440.")
		return
	}
	deadline := time.Now().Add(time.Duration(timeoutMinutes) * time.Minute)
	for result.Status != "Ready" {
		if result.Status == "Rejected" {
			response.Diagnostics.AddError("Application request rejected", stringPointerValue(result.StatusReason, "The AZExecute application request was rejected."))
			return
		}
		if time.Now().After(deadline) {
			response.Diagnostics.AddError("Application creation timed out", "AZExecute is still awaiting approval or provisioning. Increase create_timeout_minutes and apply again; the API resource ID makes the retry idempotent.")
			return
		}
		tflog.Info(ctx, "Waiting for AZExecute application", map[string]any{"resource_id": resourceID, "status": result.Status})
		select {
		case <-ctx.Done():
			response.Diagnostics.AddError("Application creation interrupted", ctx.Err().Error())
			return
		case <-time.After(time.Duration(pollSeconds) * time.Second):
		}
		result, err = r.client.GetApplication(ctx, resourceID)
		if err != nil {
			response.Diagnostics.AddError("Unable to read provisioning status", err.Error())
			return
		}
	}

	if boolValue(desired.ConfigureRegistration, false) {
		update, updateErr := updateRequestFromModel(desired, result)
		if updateErr != nil {
			response.Diagnostics.AddError("Invalid registration configuration", updateErr.Error())
			return
		}
		result, err = r.client.UpdateApplication(ctx, resourceID, update)
		if err != nil {
			response.Diagnostics.AddError("Unable to configure application registration", err.Error())
			return
		}
	}

	mapApplicationToModel(ctx, result, &desired, &response.Diagnostics)
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &desired)...)
}

func (r *applicationResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state applicationResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}
	wasIncomplete := state.Status.IsNull() || state.Status.IsUnknown() || state.Status.ValueString() != "Ready"
	result, err := r.client.GetApplication(ctx, state.ID.ValueString())
	if azclient.IsNotFound(err) {
		response.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		response.Diagnostics.AddError("Unable to read AZExecute application", err.Error())
		return
	}
	if result.Status != "Ready" {
		result, err = r.waitForApplication(ctx, result, modelIntOr(state.PollIntervalSeconds, 5), modelIntOr(state.CreateTimeoutMinutes, 60))
		if err != nil {
			response.Diagnostics.AddError("AZExecute application is not ready", err.Error())
			return
		}
	}
	if wasIncomplete && boolValue(state.ConfigureRegistration, false) {
		update, updateErr := updateRequestFromModel(state, result)
		if updateErr != nil {
			response.Diagnostics.AddError("Unable to resume registration configuration", updateErr.Error())
			return
		}
		result, err = r.client.UpdateApplication(ctx, state.ID.ValueString(), update)
		if err != nil {
			response.Diagnostics.AddError("Unable to resume registration configuration", err.Error())
			return
		}
	}
	mapApplicationToModel(ctx, result, &state, &response.Diagnostics)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &state)...)
	}
}

func (r *applicationResource) waitForApplication(ctx context.Context, result *azclient.Application, pollSeconds, timeoutMinutes int64) (*azclient.Application, error) {
	if pollSeconds < 1 || pollSeconds > 300 || timeoutMinutes < 1 || timeoutMinutes > 1440 {
		return nil, fmt.Errorf("poll_interval_seconds must be 1-300 and create_timeout_minutes must be 1-1440")
	}
	deadline := time.Now().Add(time.Duration(timeoutMinutes) * time.Minute)
	for result.Status != "Ready" {
		if result.Status == "Rejected" {
			return nil, fmt.Errorf("application request rejected: %s", stringPointerValue(result.StatusReason, "no reason was supplied"))
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out while status was %s", result.Status)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(pollSeconds) * time.Second):
		}
		var err error
		result, err = r.client.GetApplication(ctx, result.ResourceID)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *applicationResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan applicationResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}
	current, err := r.client.GetApplication(ctx, plan.ID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Unable to read AZExecute application before update", err.Error())
		return
	}
	update, err := updateRequestFromModel(plan, current)
	if err != nil {
		response.Diagnostics.AddError("Invalid application update", err.Error())
		return
	}
	result, err := r.client.UpdateApplication(ctx, plan.ID.ValueString(), update)
	if err != nil {
		response.Diagnostics.AddError("Unable to update AZExecute application", err.Error())
		return
	}
	mapApplicationToModel(ctx, result, &plan, &response.Diagnostics)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	}
}

func (r *applicationResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state applicationResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}
	err := r.client.DeleteApplication(ctx, state.ID.ValueString())
	if err != nil && !azclient.IsNotFound(err) {
		response.Diagnostics.AddError("Unable to delete AZExecute application", err.Error())
	}
}

func (r *applicationResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
}

func createRequestFromModel(ctx context.Context, model applicationResourceModel, resourceID string) (azclient.ApplicationCreate, error) {
	metadata, err := metadataFromModel(model)
	if err != nil {
		return azclient.ApplicationCreate{}, err
	}
	permissions := make([]azclient.APIPermissionRequest, 0)
	if !model.APIPermissionRequests.IsNull() && !model.APIPermissionRequests.IsUnknown() {
		var blocks []permissionRequestModel
		diagnostics := model.APIPermissionRequests.ElementsAs(ctx, &blocks, false)
		if diagnostics.HasError() {
			return azclient.ApplicationCreate{}, fmt.Errorf("api_permission_request blocks could not be read: %s", diagnostics.Errors()[0].Summary())
		}
		for _, block := range blocks {
			item := azclient.APIPermissionRequest{TargetType: block.TargetType.ValueString(), GrantType: block.GrantType.ValueString(), TargetApplicationEntityID: intPointer(block.TargetApplicationEntityID), TargetExternalAPIAppID: stringPointer(block.TargetExternalAPIAppID), TargetExternalAPIDisplayName: stringPointer(block.TargetExternalAPIDisplayName), Justification: stringPointer(block.Justification)}
			var permissionBlocks []permissionModel
			permissionDiagnostics := block.Permissions.ElementsAs(ctx, &permissionBlocks, false)
			if permissionDiagnostics.HasError() || len(permissionBlocks) == 0 {
				return azclient.ApplicationCreate{}, fmt.Errorf("each api_permission_request needs at least one permission block")
			}
			for _, permission := range permissionBlocks {
				if permission.ID.ValueString() == "" {
					return azclient.ApplicationCreate{}, fmt.Errorf("permission id cannot be empty")
				}
				item.Permissions = append(item.Permissions, azclient.APIPermission{ID: permission.ID.ValueString(), DisplayName: stringPointer(permission.DisplayName), Value: stringPointer(permission.Value), RequiresAdminConsent: boolValue(permission.RequiresAdminConsent, false)})
			}
			permissions = append(permissions, item)
		}
	}
	return azclient.ApplicationCreate{ResourceID: resourceID, DisplayName: model.DisplayName.ValueString(), Description: stringPointer(model.Description), Metadata: metadata, APIPermissionRequests: permissions}, nil
}

func metadataFromModel(model applicationResourceModel) (azclient.ApplicationMetadata, error) {
	var goLive *time.Time
	if value := stringPointer(model.ExpectedGoLiveDate); value != nil {
		parsed, err := time.Parse(time.RFC3339, *value)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", *value)
			if err != nil {
				return azclient.ApplicationMetadata{}, fmt.Errorf("expected_go_live_date must be RFC 3339 or YYYY-MM-DD")
			}
		}
		goLive = &parsed
	}
	criticality := modelIntOr(model.BusinessCriticality, 3)
	if criticality < 1 || criticality > 5 {
		return azclient.ApplicationMetadata{}, fmt.Errorf("business_criticality must be between 1 and 5")
	}
	return azclient.ApplicationMetadata{BusinessJustification: model.BusinessJustification.ValueString(), TechnicalRequirements: stringPointer(model.TechnicalRequirements), IntendedAudience: stringPointer(model.IntendedAudience), DataAccessRequirements: stringPointer(model.DataAccessRequirements), ComplianceNotes: stringPointer(model.ComplianceNotes), ExpectedGoLiveDate: goLive, ProjectName: stringPointer(model.ProjectName), DepartmentOwner: stringPointer(model.DepartmentOwner), BusinessCriticality: criticality, RequiresElevatedPermissions: boolValue(model.RequiresElevatedPermissions, false), ElevatedPermissionsJustification: stringPointer(model.ElevatedPermissionsJustification), Environment: stringPointer(model.Environment), ContactEmail: stringPointer(model.ContactEmail), ContactPhone: stringPointer(model.ContactPhone)}, nil
}

func updateRequestFromModel(model applicationResourceModel, current *azclient.Application) (azclient.ApplicationUpdate, error) {
	metadata, err := metadataFromModel(model)
	if err != nil {
		return azclient.ApplicationUpdate{}, err
	}
	update := azclient.ApplicationUpdate{Metadata: metadata}
	if boolValue(model.ConfigureRegistration, false) {
		if current.Registration == nil {
			return update, fmt.Errorf("the tenant API did not return registration configuration; confirm the Terraform registration policy is enabled")
		}
		registration := *current.Registration
		registration.SignInAudience = modelStringOr(model.SignInAudience, registration.SignInAudience, "AzureADMyOrg")
		registration.IsFallbackPublicClient = boolValue(model.IsFallbackPublicClient, registration.IsFallbackPublicClient)
		registration.IdentifierUris = uriValues(model.IdentifierURIs, registration.IdentifierUris)
		registration.Web.HomePageURL = stringPointerOr(model.WebHomePageURL, registration.Web.HomePageURL)
		registration.Web.LogoutURL = stringPointerOr(model.WebLogoutURL, registration.Web.LogoutURL)
		registration.Web.EnableAccessTokenIssuance = boolValue(model.WebEnableAccessTokenIssuance, registration.Web.EnableAccessTokenIssuance)
		registration.Web.EnableIDTokenIssuance = boolValue(model.WebEnableIDTokenIssuance, registration.Web.EnableIDTokenIssuance)
		registration.Web.RedirectUris = redirectValues(model.WebRedirectURIs, registration.Web.RedirectUris)
		registration.Spa.RedirectUris = redirectValues(model.SpaRedirectURIs, registration.Spa.RedirectUris)
		registration.PublicClient.RedirectUris = redirectValues(model.PublicClientRedirectURIs, registration.PublicClient.RedirectUris)
		if !model.RequestedAccessTokenVersion.IsNull() && !model.RequestedAccessTokenVersion.IsUnknown() {
			value := model.RequestedAccessTokenVersion.ValueInt64()
			registration.API.RequestedAccessTokenVersion = &value
		}
		update.Registration = &registration
	}
	return update, nil
}

func mapApplicationToModel(ctx context.Context, source *azclient.Application, target *applicationResourceModel, diagnostics *diag.Diagnostics) {
	target.ID = types.StringValue(source.ResourceID)
	target.DisplayName = types.StringValue(source.DisplayName)
	target.Description = stringTypeFromPointer(source.Description)
	target.BusinessJustification = types.StringValue(source.Metadata.BusinessJustification)
	target.TechnicalRequirements = stringTypeFromPointer(source.Metadata.TechnicalRequirements)
	target.IntendedAudience = stringTypeFromPointer(source.Metadata.IntendedAudience)
	target.DataAccessRequirements = stringTypeFromPointer(source.Metadata.DataAccessRequirements)
	target.ComplianceNotes = stringTypeFromPointer(source.Metadata.ComplianceNotes)
	if source.Metadata.ExpectedGoLiveDate == nil {
		target.ExpectedGoLiveDate = types.StringNull()
	} else {
		target.ExpectedGoLiveDate = expectedGoLiveDateValue(target.ExpectedGoLiveDate, *source.Metadata.ExpectedGoLiveDate)
	}
	target.ProjectName = stringTypeFromPointer(source.Metadata.ProjectName)
	target.DepartmentOwner = stringTypeFromPointer(source.Metadata.DepartmentOwner)
	target.BusinessCriticality = types.Int64Value(source.Metadata.BusinessCriticality)
	target.RequiresElevatedPermissions = types.BoolValue(source.Metadata.RequiresElevatedPermissions)
	target.ElevatedPermissionsJustification = stringTypeFromPointer(source.Metadata.ElevatedPermissionsJustification)
	target.Environment = stringTypeFromPointer(source.Metadata.Environment)
	target.ContactEmail = stringTypeFromPointer(source.Metadata.ContactEmail)
	target.ContactPhone = stringTypeFromPointer(source.Metadata.ContactPhone)
	target.Status = types.StringValue(source.Status)
	target.StatusReason = stringTypeFromPointer(source.StatusReason)
	target.RequestID = types.Int64Value(source.RequestID)
	target.ApplicationEntityID = int64TypeFromPointer(source.ApplicationEntityID)
	target.ApplicationID = stringTypeFromPointer(source.ApplicationID)
	target.ApplicationObjectID = stringTypeFromPointer(source.ApplicationObjectID)

	if source.Registration != nil {
		registration := source.Registration
		target.SignInAudience = types.StringValue(registration.SignInAudience)
		target.IsFallbackPublicClient = types.BoolValue(registration.IsFallbackPublicClient)
		target.WebHomePageURL = stringTypeFromPointer(registration.Web.HomePageURL)
		target.WebLogoutURL = stringTypeFromPointer(registration.Web.LogoutURL)
		target.WebEnableAccessTokenIssuance = types.BoolValue(registration.Web.EnableAccessTokenIssuance)
		target.WebEnableIDTokenIssuance = types.BoolValue(registration.Web.EnableIDTokenIssuance)
		target.RequestedAccessTokenVersion = int64TypeFromPointer(registration.API.RequestedAccessTokenVersion)
		var setDiagnostics diag.Diagnostics
		target.IdentifierURIs, setDiagnostics = types.SetValueFrom(ctx, types.StringType, uriStrings(registration.IdentifierUris))
		diagnostics.Append(setDiagnostics...)
		target.WebRedirectURIs, setDiagnostics = types.SetValueFrom(ctx, types.StringType, redirectStrings(registration.Web.RedirectUris))
		diagnostics.Append(setDiagnostics...)
		target.SpaRedirectURIs, setDiagnostics = types.SetValueFrom(ctx, types.StringType, redirectStrings(registration.Spa.RedirectUris))
		diagnostics.Append(setDiagnostics...)
		target.PublicClientRedirectURIs, setDiagnostics = types.SetValueFrom(ctx, types.StringType, redirectStrings(registration.PublicClient.RedirectUris))
		diagnostics.Append(setDiagnostics...)
	} else {
		if boolValue(target.ConfigureRegistration, false) {
			normalizeUnknownRegistrationValues(target)
		} else {
			target.SignInAudience = types.StringNull()
			target.IsFallbackPublicClient = types.BoolNull()
			target.IdentifierURIs = types.SetNull(types.StringType)
			target.WebHomePageURL = types.StringNull()
			target.WebLogoutURL = types.StringNull()
			target.WebEnableAccessTokenIssuance = types.BoolNull()
			target.WebEnableIDTokenIssuance = types.BoolNull()
			target.WebRedirectURIs = types.SetNull(types.StringType)
			target.SpaRedirectURIs = types.SetNull(types.StringType)
			target.PublicClientRedirectURIs = types.SetNull(types.StringType)
			target.RequestedAccessTokenVersion = types.Int64Null()
		}
	}
}

func expectedGoLiveDateValue(existing types.String, serverValue time.Time) types.String {
	if !existing.IsNull() && !existing.IsUnknown() {
		value := existing.ValueString()
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", value)
		}
		if err == nil && parsed.Equal(serverValue) {
			return existing
		}
	}
	return types.StringValue(serverValue.Format(time.RFC3339))
}

func normalizeUnknownRegistrationValues(target *applicationResourceModel) {
	if target.SignInAudience.IsUnknown() {
		target.SignInAudience = types.StringNull()
	}
	if target.IsFallbackPublicClient.IsUnknown() {
		target.IsFallbackPublicClient = types.BoolNull()
	}
	if target.IdentifierURIs.IsUnknown() {
		target.IdentifierURIs = types.SetNull(types.StringType)
	}
	if target.WebHomePageURL.IsUnknown() {
		target.WebHomePageURL = types.StringNull()
	}
	if target.WebLogoutURL.IsUnknown() {
		target.WebLogoutURL = types.StringNull()
	}
	if target.WebEnableAccessTokenIssuance.IsUnknown() {
		target.WebEnableAccessTokenIssuance = types.BoolNull()
	}
	if target.WebEnableIDTokenIssuance.IsUnknown() {
		target.WebEnableIDTokenIssuance = types.BoolNull()
	}
	if target.WebRedirectURIs.IsUnknown() {
		target.WebRedirectURIs = types.SetNull(types.StringType)
	}
	if target.SpaRedirectURIs.IsUnknown() {
		target.SpaRedirectURIs = types.SetNull(types.StringType)
	}
	if target.PublicClientRedirectURIs.IsUnknown() {
		target.PublicClientRedirectURIs = types.SetNull(types.StringType)
	}
	if target.RequestedAccessTokenVersion.IsUnknown() {
		target.RequestedAccessTokenVersion = types.Int64Null()
	}
}

func newUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}

func modelIntOr(value types.Int64, fallback int64) int64 {
	if value.IsNull() || value.IsUnknown() {
		return fallback
	}
	return value.ValueInt64()
}
func boolValue(value types.Bool, fallback bool) bool {
	if value.IsNull() || value.IsUnknown() {
		return fallback
	}
	return value.ValueBool()
}
func stringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return nil
	}
	result := value.ValueString()
	return &result
}
func intPointer(value types.Int64) *int64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := value.ValueInt64()
	return &result
}
func modelStringOr(value types.String, current, fallback string) string {
	if !value.IsNull() && !value.IsUnknown() {
		return value.ValueString()
	}
	if current != "" {
		return current
	}
	return fallback
}
func stringPointerOr(value types.String, current *string) *string {
	if !value.IsNull() && !value.IsUnknown() {
		result := value.ValueString()
		if result == "" {
			return nil
		}
		return &result
	}
	return current
}
func stringPointerValue(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}
func stringTypeFromPointer(value *string) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(*value)
}
func int64TypeFromPointer(value *int64) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*value)
}
func uriStrings(values []azclient.URIValue) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.Value)
	}
	return result
}
func redirectStrings(values []azclient.RedirectURI) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.Value)
	}
	return result
}

func uriValues(value types.Set, current []azclient.URIValue) []azclient.URIValue {
	if value.IsNull() || value.IsUnknown() {
		return current
	}
	result := make([]azclient.URIValue, 0, len(value.Elements()))
	for _, element := range value.Elements() {
		if item, ok := element.(types.String); ok {
			result = append(result, azclient.URIValue{Value: item.ValueString()})
		}
	}
	return result
}
func redirectValues(value types.Set, current []azclient.RedirectURI) []azclient.RedirectURI {
	if value.IsNull() || value.IsUnknown() {
		return current
	}
	result := make([]azclient.RedirectURI, 0, len(value.Elements()))
	for _, element := range value.Elements() {
		if item, ok := element.(types.String); ok {
			result = append(result, azclient.RedirectURI{Value: item.ValueString()})
		}
	}
	return result
}
