package provider

import (
	"context"
	"fmt"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &applicationRequestResource{}
var _ resource.ResourceWithConfigure = &applicationRequestResource{}
var _ resource.ResourceWithImportState = &applicationRequestResource{}
var _ resource.ResourceWithModifyPlan = &applicationRequestResource{}
var _ resource.ResourceWithMoveState = &applicationRequestResource{}
var _ resource.ResourceWithUpgradeState = &applicationRequestResource{}

type applicationRequestResource struct{ client *azclient.Client }

type applicationRequestResourceModel struct {
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
	OwnerObjectIDs                   types.Set    `tfsdk:"owner_object_ids"`
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
	Status                           types.String `tfsdk:"status"`
	StatusReason                     types.String `tfsdk:"status_reason"`
	RequestID                        types.Int64  `tfsdk:"request_id"`
	ApplicationEntityID              types.String `tfsdk:"application_entity_id"`
	ApplicationID                    types.String `tfsdk:"application_id"`
	ApplicationObjectID              types.String `tfsdk:"application_object_id"`
}

func NewApplicationRequestResource() resource.Resource { return &applicationRequestResource{} }

func (r *applicationRequestResource) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_application_request"
}

func (r *applicationRequestResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = managedApplicationSchema(false)
	response.Schema.Description = "Submits an AZExecute application request and records its status without waiting for human approval or background provisioning."
}

func (r *applicationRequestResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	r.client = clientFromProviderData(request.ProviderData, &response.Diagnostics)
}

func (r *applicationRequestResource) ModifyPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse) {
	if r.client == nil || request.Plan.Raw.IsNull() {
		return
	}

	var plan applicationRequestResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}

	capabilities, err := r.client.Capabilities(ctx)
	if err != nil {
		response.Diagnostics.AddError("Unable to validate AZExecute tenant policy", err.Error())
		return
	}

	errors := validateApplicationPlan(plan.toApplicationModel(), capabilities, request.State.Raw.IsNull())
	if len(errors) > 0 {
		response.Diagnostics.AddError("Invalid application request configuration for this AZExecute tenant", strings.Join(errors, "\n"))
	}
}

func (r *applicationRequestResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan applicationRequestResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}

	resourceID, err := newUUID()
	if err != nil {
		response.Diagnostics.AddError("Unable to generate request resource ID", err.Error())
		return
	}
	applicationPlan := plan.toApplicationModel()
	create, err := createRequestFromModel(ctx, applicationPlan, resourceID)
	if err != nil {
		response.Diagnostics.AddError("Invalid application request configuration", err.Error())
		return
	}

	result, err := r.client.CreateApplication(ctx, create)
	if err != nil {
		response.Diagnostics.AddError("Unable to submit AZExecute application request", err.Error())
		return
	}

	// Automatic tenants can occasionally complete before POST returns. Apply registration
	// settings immediately in that case; approval-based requests apply them on a later run.
	if result.Status == "Ready" && (boolValue(applicationPlan.ConfigureRegistration, false) || setIsConfigured(applicationPlan.OwnerObjectIDs)) {
		update, updateErr := updateRequestFromModel(applicationPlan, result)
		if updateErr != nil {
			response.Diagnostics.AddError("Invalid registration configuration", updateErr.Error())
			return
		}
		result, err = r.client.UpdateApplication(ctx, result.ResourceID, update)
		if err != nil {
			response.Diagnostics.AddError("Unable to configure approved application registration", err.Error())
			return
		}
	}

	mapApplicationToRequestModel(ctx, result, &plan, &response.Diagnostics)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	}
}

func (r *applicationRequestResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state applicationRequestResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}

	result, err := r.client.GetApplication(ctx, state.ID.ValueString())
	if azclient.IsNotFound(err) {
		response.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		response.Diagnostics.AddError("Unable to read AZExecute application request", err.Error())
		return
	}

	mapApplicationToRequestModel(ctx, result, &state, &response.Diagnostics)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &state)...)
	}
}

func (r *applicationRequestResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan applicationRequestResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}

	current, err := r.client.GetApplication(ctx, plan.ID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Unable to read AZExecute application request before update", err.Error())
		return
	}
	if current.Status != "Ready" {
		response.Diagnostics.AddError(
			"Application request is not ready for updates",
			fmt.Sprintf("AZExecute reports status %q. Approve and provision the request, then run Terraform again before changing its metadata or registration.", current.Status))
		return
	}

	update, err := updateRequestFromModel(plan.toApplicationModel(), current)
	if err != nil {
		response.Diagnostics.AddError("Invalid approved application update", err.Error())
		return
	}
	result, err := r.client.UpdateApplication(ctx, plan.ID.ValueString(), update)
	if err != nil {
		response.Diagnostics.AddError("Unable to update approved AZExecute application", err.Error())
		return
	}

	mapApplicationToRequestModel(ctx, result, &plan, &response.Diagnostics)
	if !response.Diagnostics.HasError() {
		response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
	}
}

func (r *applicationRequestResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state applicationRequestResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}
	if err := r.client.DeleteApplication(ctx, state.ID.ValueString()); err != nil && !azclient.IsNotFound(err) {
		response.Diagnostics.AddError("Unable to delete AZExecute application request", err.Error())
	}
}

func (r *applicationRequestResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
}

// MoveState provides a lossless upgrade path for configurations that used the
// original synchronous resource before approval-aware requests were available.
func (r *applicationRequestResource) MoveState(_ context.Context) []resource.StateMover {
	legacySourceSchema := managedApplicationSchemaV0(true)
	currentSourceSchema := managedApplicationSchema(true)
	return []resource.StateMover{{
		SourceSchema: &legacySourceSchema,
		StateMover: func(ctx context.Context, request resource.MoveStateRequest, response *resource.MoveStateResponse) {
			if request.SourceTypeName != "azexecute_application" ||
				!strings.HasSuffix(request.SourceProviderAddress, "/dyntora/azexecute") ||
				request.SourceSchemaVersion != 0 || request.SourceState == nil {
				return
			}

			var source applicationResourceModelV0
			response.Diagnostics.Append(request.SourceState.Get(ctx, &source)...)
			if response.Diagnostics.HasError() {
				return
			}

			upgraded := upgradeApplicationState(ctx, source, &response.Diagnostics)
			if response.Diagnostics.HasError() {
				return
			}
			var target applicationRequestResourceModel
			target.setFromApplicationModel(upgraded)
			response.Diagnostics.Append(response.TargetState.Set(ctx, &target)...)
		},
	}, {
		SourceSchema: &currentSourceSchema,
		StateMover: func(ctx context.Context, request resource.MoveStateRequest, response *resource.MoveStateResponse) {
			if request.SourceTypeName != "azexecute_application" ||
				!strings.HasSuffix(request.SourceProviderAddress, "/dyntora/azexecute") ||
				request.SourceSchemaVersion != 1 || request.SourceState == nil {
				return
			}

			var source applicationResourceModel
			response.Diagnostics.Append(request.SourceState.Get(ctx, &source)...)
			if response.Diagnostics.HasError() {
				return
			}

			var target applicationRequestResourceModel
			target.setFromApplicationModel(source)
			response.Diagnostics.Append(response.TargetState.Set(ctx, &target)...)
		},
	}}
}

func mapApplicationToRequestModel(ctx context.Context, source *azclient.Application, target *applicationRequestResourceModel, diagnostics *diag.Diagnostics) {
	applicationModel := target.toApplicationModel()
	mapApplicationToModel(ctx, source, &applicationModel, diagnostics)
	target.setFromApplicationModel(applicationModel)
}

func (m applicationRequestResourceModel) toApplicationModel() applicationResourceModel {
	return applicationResourceModel{
		ID: m.ID, DisplayName: m.DisplayName, Description: m.Description, BusinessJustification: m.BusinessJustification,
		TechnicalRequirements: m.TechnicalRequirements, IntendedAudience: m.IntendedAudience, DataAccessRequirements: m.DataAccessRequirements,
		ComplianceNotes: m.ComplianceNotes, ExpectedGoLiveDate: m.ExpectedGoLiveDate, ProjectName: m.ProjectName, DepartmentOwner: m.DepartmentOwner,
		BusinessCriticality: m.BusinessCriticality, RequiresElevatedPermissions: m.RequiresElevatedPermissions,
		ElevatedPermissionsJustification: m.ElevatedPermissionsJustification, Environment: m.Environment, ContactEmail: m.ContactEmail,
		ContactPhone: m.ContactPhone, OwnerObjectIDs: m.OwnerObjectIDs, APIPermissionRequests: m.APIPermissionRequests, ConfigureRegistration: m.ConfigureRegistration,
		SignInAudience: m.SignInAudience, IsFallbackPublicClient: m.IsFallbackPublicClient, IdentifierURIs: m.IdentifierURIs,
		WebHomePageURL: m.WebHomePageURL, WebLogoutURL: m.WebLogoutURL, WebEnableAccessTokenIssuance: m.WebEnableAccessTokenIssuance,
		WebEnableIDTokenIssuance: m.WebEnableIDTokenIssuance, WebRedirectURIs: m.WebRedirectURIs, SpaRedirectURIs: m.SpaRedirectURIs,
		PublicClientRedirectURIs: m.PublicClientRedirectURIs, RequestedAccessTokenVersion: m.RequestedAccessTokenVersion,
		Status: m.Status, StatusReason: m.StatusReason, RequestID: m.RequestID, ApplicationEntityID: m.ApplicationEntityID,
		ApplicationID: m.ApplicationID, ApplicationObjectID: m.ApplicationObjectID,
	}
}

func (m *applicationRequestResourceModel) setFromApplicationModel(source applicationResourceModel) {
	m.ID, m.DisplayName, m.Description, m.BusinessJustification = source.ID, source.DisplayName, source.Description, source.BusinessJustification
	m.TechnicalRequirements, m.IntendedAudience = source.TechnicalRequirements, source.IntendedAudience
	m.DataAccessRequirements, m.ComplianceNotes, m.ExpectedGoLiveDate = source.DataAccessRequirements, source.ComplianceNotes, source.ExpectedGoLiveDate
	m.ProjectName, m.DepartmentOwner, m.BusinessCriticality = source.ProjectName, source.DepartmentOwner, source.BusinessCriticality
	m.RequiresElevatedPermissions, m.ElevatedPermissionsJustification = source.RequiresElevatedPermissions, source.ElevatedPermissionsJustification
	m.Environment, m.ContactEmail, m.ContactPhone, m.OwnerObjectIDs = source.Environment, source.ContactEmail, source.ContactPhone, source.OwnerObjectIDs
	m.APIPermissionRequests, m.ConfigureRegistration = source.APIPermissionRequests, source.ConfigureRegistration
	m.SignInAudience, m.IsFallbackPublicClient, m.IdentifierURIs = source.SignInAudience, source.IsFallbackPublicClient, source.IdentifierURIs
	m.WebHomePageURL, m.WebLogoutURL = source.WebHomePageURL, source.WebLogoutURL
	m.WebEnableAccessTokenIssuance, m.WebEnableIDTokenIssuance = source.WebEnableAccessTokenIssuance, source.WebEnableIDTokenIssuance
	m.WebRedirectURIs, m.SpaRedirectURIs = source.WebRedirectURIs, source.SpaRedirectURIs
	m.PublicClientRedirectURIs, m.RequestedAccessTokenVersion = source.PublicClientRedirectURIs, source.RequestedAccessTokenVersion
	m.Status, m.StatusReason, m.RequestID = source.Status, source.StatusReason, source.RequestID
	m.ApplicationEntityID, m.ApplicationID, m.ApplicationObjectID = source.ApplicationEntityID, source.ApplicationID, source.ApplicationObjectID
}
