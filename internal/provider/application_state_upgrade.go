package provider

import (
	"context"
	"crypto/md5" // #nosec G501 -- deterministic legacy identifier mapping, not cryptography.
	"encoding/hex"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type applicationResourceModelV0 struct {
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

type applicationRequestResourceModelV0 struct {
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
	Status                           types.String `tfsdk:"status"`
	StatusReason                     types.String `tfsdk:"status_reason"`
	RequestID                        types.Int64  `tfsdk:"request_id"`
	ApplicationEntityID              types.Int64  `tfsdk:"application_entity_id"`
	ApplicationID                    types.String `tfsdk:"application_id"`
	ApplicationObjectID              types.String `tfsdk:"application_object_id"`
}

// Version 1 is the UUID/owner-aware v0.7 schema. Version 2 adds app_roles.
type applicationResourceModelV1 struct {
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
	PollIntervalSeconds              types.Int64  `tfsdk:"poll_interval_seconds"`
	CreateTimeoutMinutes             types.Int64  `tfsdk:"create_timeout_minutes"`
	Status                           types.String `tfsdk:"status"`
	StatusReason                     types.String `tfsdk:"status_reason"`
	RequestID                        types.Int64  `tfsdk:"request_id"`
	ApplicationEntityID              types.String `tfsdk:"application_entity_id"`
	ApplicationID                    types.String `tfsdk:"application_id"`
	ApplicationObjectID              types.String `tfsdk:"application_object_id"`
}

type applicationRequestResourceModelV1 struct {
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

type permissionRequestModelV0 struct {
	TargetType                   types.String `tfsdk:"target_type"`
	TargetApplicationEntityID    types.Int64  `tfsdk:"target_application_entity_id"`
	TargetExternalAPIAppID       types.String `tfsdk:"target_external_api_app_id"`
	TargetExternalAPIDisplayName types.String `tfsdk:"target_external_api_display_name"`
	GrantType                    types.String `tfsdk:"grant_type"`
	Justification                types.String `tfsdk:"justification"`
	Permissions                  types.Set    `tfsdk:"permission"`
}

func (r *applicationResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	legacySchema := managedApplicationSchemaV0(true)
	versionOneSchema := managedApplicationSchemaV1(true)
	return map[int64]resource.StateUpgrader{0: {
		PriorSchema: &legacySchema,
		StateUpgrader: func(ctx context.Context, request resource.UpgradeStateRequest, response *resource.UpgradeStateResponse) {
			var prior applicationResourceModelV0
			response.Diagnostics.Append(request.State.Get(ctx, &prior)...)
			if response.Diagnostics.HasError() {
				return
			}
			upgraded := upgradeApplicationState(ctx, prior, &response.Diagnostics)
			if !response.Diagnostics.HasError() {
				response.Diagnostics.Append(response.State.Set(ctx, &upgraded)...)
			}
		},
	}, 1: {
		PriorSchema: &versionOneSchema,
		StateUpgrader: func(ctx context.Context, request resource.UpgradeStateRequest, response *resource.UpgradeStateResponse) {
			var prior applicationResourceModelV1
			response.Diagnostics.Append(request.State.Get(ctx, &prior)...)
			if !response.Diagnostics.HasError() {
				upgraded := upgradeApplicationStateV1(prior)
				response.Diagnostics.Append(response.State.Set(ctx, &upgraded)...)
			}
		},
	}}
}

func (r *applicationRequestResource) UpgradeState(_ context.Context) map[int64]resource.StateUpgrader {
	legacySchema := managedApplicationSchemaV0(false)
	versionOneSchema := managedApplicationSchemaV1(false)
	return map[int64]resource.StateUpgrader{0: {
		PriorSchema: &legacySchema,
		StateUpgrader: func(ctx context.Context, request resource.UpgradeStateRequest, response *resource.UpgradeStateResponse) {
			var prior applicationRequestResourceModelV0
			response.Diagnostics.Append(request.State.Get(ctx, &prior)...)
			if response.Diagnostics.HasError() {
				return
			}
			upgradedApplication := upgradeApplicationState(ctx, prior.asApplicationState(), &response.Diagnostics)
			if response.Diagnostics.HasError() {
				return
			}
			var upgraded applicationRequestResourceModel
			upgraded.setFromApplicationModel(upgradedApplication)
			response.Diagnostics.Append(response.State.Set(ctx, &upgraded)...)
		},
	}, 1: {
		PriorSchema: &versionOneSchema,
		StateUpgrader: func(ctx context.Context, request resource.UpgradeStateRequest, response *resource.UpgradeStateResponse) {
			var prior applicationRequestResourceModelV1
			response.Diagnostics.Append(request.State.Get(ctx, &prior)...)
			if response.Diagnostics.HasError() {
				return
			}
			upgradedApplication := upgradeApplicationStateV1(prior.asApplicationState())
			var upgraded applicationRequestResourceModel
			upgraded.setFromApplicationModel(upgradedApplication)
			response.Diagnostics.Append(response.State.Set(ctx, &upgraded)...)
		},
	}}
}

func managedApplicationSchemaV1(includeWaitSettings bool) schema.Schema {
	result := managedApplicationSchema(includeWaitSettings)
	result.Version = 1
	delete(result.Attributes, "app_roles")
	return result
}

func upgradeApplicationState(ctx context.Context, prior applicationResourceModelV0, diagnostics *diag.Diagnostics) applicationResourceModel {
	permissionRequests, setDiagnostics := upgradePermissionRequestState(ctx, prior.APIPermissionRequests)
	diagnostics.Append(setDiagnostics...)
	return applicationResourceModel{
		ID: prior.ID, DisplayName: prior.DisplayName, Description: prior.Description, BusinessJustification: prior.BusinessJustification,
		TechnicalRequirements: prior.TechnicalRequirements, IntendedAudience: prior.IntendedAudience, DataAccessRequirements: prior.DataAccessRequirements,
		ComplianceNotes: prior.ComplianceNotes, ExpectedGoLiveDate: prior.ExpectedGoLiveDate, ProjectName: prior.ProjectName, DepartmentOwner: prior.DepartmentOwner,
		BusinessCriticality: prior.BusinessCriticality, RequiresElevatedPermissions: prior.RequiresElevatedPermissions,
		ElevatedPermissionsJustification: prior.ElevatedPermissionsJustification, Environment: prior.Environment, ContactEmail: prior.ContactEmail,
		ContactPhone: prior.ContactPhone, OwnerObjectIDs: types.SetNull(types.StringType), APIPermissionRequests: permissionRequests, ConfigureRegistration: prior.ConfigureRegistration,
		SignInAudience: prior.SignInAudience, IsFallbackPublicClient: prior.IsFallbackPublicClient, IdentifierURIs: prior.IdentifierURIs,
		WebHomePageURL: prior.WebHomePageURL, WebLogoutURL: prior.WebLogoutURL, WebEnableAccessTokenIssuance: prior.WebEnableAccessTokenIssuance,
		WebEnableIDTokenIssuance: prior.WebEnableIDTokenIssuance, WebRedirectURIs: prior.WebRedirectURIs, SpaRedirectURIs: prior.SpaRedirectURIs,
		PublicClientRedirectURIs: prior.PublicClientRedirectURIs, RequestedAccessTokenVersion: prior.RequestedAccessTokenVersion,
		AppRoles:            types.SetNull(appRoleObjectType()),
		PollIntervalSeconds: prior.PollIntervalSeconds, CreateTimeoutMinutes: prior.CreateTimeoutMinutes, Status: prior.Status, StatusReason: prior.StatusReason,
		RequestID: prior.RequestID, ApplicationEntityID: stringFromLegacyID(prior.ApplicationEntityID), ApplicationID: prior.ApplicationID,
		ApplicationObjectID: prior.ApplicationObjectID,
	}
}

func upgradeApplicationStateV1(prior applicationResourceModelV1) applicationResourceModel {
	return applicationResourceModel{
		ID: prior.ID, DisplayName: prior.DisplayName, Description: prior.Description, BusinessJustification: prior.BusinessJustification,
		TechnicalRequirements: prior.TechnicalRequirements, IntendedAudience: prior.IntendedAudience, DataAccessRequirements: prior.DataAccessRequirements,
		ComplianceNotes: prior.ComplianceNotes, ExpectedGoLiveDate: prior.ExpectedGoLiveDate, ProjectName: prior.ProjectName, DepartmentOwner: prior.DepartmentOwner,
		BusinessCriticality: prior.BusinessCriticality, RequiresElevatedPermissions: prior.RequiresElevatedPermissions,
		ElevatedPermissionsJustification: prior.ElevatedPermissionsJustification, Environment: prior.Environment, ContactEmail: prior.ContactEmail,
		ContactPhone: prior.ContactPhone, OwnerObjectIDs: prior.OwnerObjectIDs, APIPermissionRequests: prior.APIPermissionRequests,
		ConfigureRegistration: prior.ConfigureRegistration, SignInAudience: prior.SignInAudience, IsFallbackPublicClient: prior.IsFallbackPublicClient,
		IdentifierURIs: prior.IdentifierURIs, WebHomePageURL: prior.WebHomePageURL, WebLogoutURL: prior.WebLogoutURL,
		WebEnableAccessTokenIssuance: prior.WebEnableAccessTokenIssuance, WebEnableIDTokenIssuance: prior.WebEnableIDTokenIssuance,
		WebRedirectURIs: prior.WebRedirectURIs, SpaRedirectURIs: prior.SpaRedirectURIs, PublicClientRedirectURIs: prior.PublicClientRedirectURIs,
		RequestedAccessTokenVersion: prior.RequestedAccessTokenVersion, AppRoles: types.SetNull(appRoleObjectType()),
		PollIntervalSeconds: prior.PollIntervalSeconds, CreateTimeoutMinutes: prior.CreateTimeoutMinutes, Status: prior.Status,
		StatusReason: prior.StatusReason, RequestID: prior.RequestID, ApplicationEntityID: prior.ApplicationEntityID,
		ApplicationID: prior.ApplicationID, ApplicationObjectID: prior.ApplicationObjectID,
	}
}

func upgradePermissionRequestState(ctx context.Context, prior types.Set) (types.Set, diag.Diagnostics) {
	elementType := permissionRequestObjectType()
	if prior.IsNull() {
		return types.SetNull(elementType), nil
	}
	if prior.IsUnknown() {
		return types.SetUnknown(elementType), nil
	}
	var legacy []permissionRequestModelV0
	diagnostics := prior.ElementsAs(ctx, &legacy, false)
	if diagnostics.HasError() {
		return types.SetNull(elementType), diagnostics
	}
	upgraded := make([]permissionRequestModel, 0, len(legacy))
	for _, item := range legacy {
		upgraded = append(upgraded, permissionRequestModel{
			TargetType: item.TargetType, TargetApplicationEntityID: stringFromLegacyID(item.TargetApplicationEntityID),
			TargetExternalAPIAppID: item.TargetExternalAPIAppID, TargetExternalAPIDisplayName: item.TargetExternalAPIDisplayName,
			GrantType: item.GrantType, Justification: item.Justification, Permissions: item.Permissions,
		})
	}
	result, convertedDiagnostics := types.SetValueFrom(ctx, elementType, upgraded)
	diagnostics.Append(convertedDiagnostics...)
	return result, diagnostics
}

func permissionRequestObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"target_type": types.StringType, "target_application_entity_id": types.StringType,
		"target_external_api_app_id": types.StringType, "target_external_api_display_name": types.StringType,
		"grant_type": types.StringType, "justification": types.StringType,
		"permission": types.SetType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{
			"id": types.StringType, "display_name": types.StringType, "value": types.StringType, "requires_admin_consent": types.BoolType,
		}}},
	}}
}

func appRoleObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"id": types.StringType, "display_name": types.StringType, "value": types.StringType,
		"description": types.StringType, "is_enabled": types.BoolType,
		"allow_users_and_groups": types.BoolType, "allow_applications": types.BoolType,
	}}
}

func stringFromLegacyID(value types.Int64) types.String {
	if value.IsNull() {
		return types.StringNull()
	}
	if value.IsUnknown() {
		return types.StringUnknown()
	}
	legacyID := strconv.FormatInt(value.ValueInt64(), 10)
	digest := md5.Sum([]byte("azexecute-application-entity:" + legacyID)) // #nosec G401 -- must match the database migration.
	hexValue := hex.EncodeToString(digest[:])
	return types.StringValue(hexValue[0:8] + "-" + hexValue[8:12] + "-" + hexValue[12:16] + "-" + hexValue[16:20] + "-" + hexValue[20:32])
}

func (m applicationRequestResourceModelV0) asApplicationState() applicationResourceModelV0 {
	return applicationResourceModelV0{
		ID: m.ID, DisplayName: m.DisplayName, Description: m.Description, BusinessJustification: m.BusinessJustification,
		TechnicalRequirements: m.TechnicalRequirements, IntendedAudience: m.IntendedAudience, DataAccessRequirements: m.DataAccessRequirements,
		ComplianceNotes: m.ComplianceNotes, ExpectedGoLiveDate: m.ExpectedGoLiveDate, ProjectName: m.ProjectName, DepartmentOwner: m.DepartmentOwner,
		BusinessCriticality: m.BusinessCriticality, RequiresElevatedPermissions: m.RequiresElevatedPermissions,
		ElevatedPermissionsJustification: m.ElevatedPermissionsJustification, Environment: m.Environment, ContactEmail: m.ContactEmail,
		ContactPhone: m.ContactPhone, APIPermissionRequests: m.APIPermissionRequests, ConfigureRegistration: m.ConfigureRegistration,
		SignInAudience: m.SignInAudience, IsFallbackPublicClient: m.IsFallbackPublicClient, IdentifierURIs: m.IdentifierURIs,
		WebHomePageURL: m.WebHomePageURL, WebLogoutURL: m.WebLogoutURL, WebEnableAccessTokenIssuance: m.WebEnableAccessTokenIssuance,
		WebEnableIDTokenIssuance: m.WebEnableIDTokenIssuance, WebRedirectURIs: m.WebRedirectURIs, SpaRedirectURIs: m.SpaRedirectURIs,
		PublicClientRedirectURIs: m.PublicClientRedirectURIs, RequestedAccessTokenVersion: m.RequestedAccessTokenVersion,
		Status: m.Status, StatusReason: m.StatusReason, RequestID: m.RequestID, ApplicationEntityID: m.ApplicationEntityID,
		ApplicationID: m.ApplicationID, ApplicationObjectID: m.ApplicationObjectID,
	}
}

func (m applicationRequestResourceModelV1) asApplicationState() applicationResourceModelV1 {
	return applicationResourceModelV1{
		ID: m.ID, DisplayName: m.DisplayName, Description: m.Description, BusinessJustification: m.BusinessJustification,
		TechnicalRequirements: m.TechnicalRequirements, IntendedAudience: m.IntendedAudience, DataAccessRequirements: m.DataAccessRequirements,
		ComplianceNotes: m.ComplianceNotes, ExpectedGoLiveDate: m.ExpectedGoLiveDate, ProjectName: m.ProjectName, DepartmentOwner: m.DepartmentOwner,
		BusinessCriticality: m.BusinessCriticality, RequiresElevatedPermissions: m.RequiresElevatedPermissions,
		ElevatedPermissionsJustification: m.ElevatedPermissionsJustification, Environment: m.Environment, ContactEmail: m.ContactEmail,
		ContactPhone: m.ContactPhone, OwnerObjectIDs: m.OwnerObjectIDs, APIPermissionRequests: m.APIPermissionRequests,
		ConfigureRegistration: m.ConfigureRegistration, SignInAudience: m.SignInAudience, IsFallbackPublicClient: m.IsFallbackPublicClient,
		IdentifierURIs: m.IdentifierURIs, WebHomePageURL: m.WebHomePageURL, WebLogoutURL: m.WebLogoutURL,
		WebEnableAccessTokenIssuance: m.WebEnableAccessTokenIssuance, WebEnableIDTokenIssuance: m.WebEnableIDTokenIssuance,
		WebRedirectURIs: m.WebRedirectURIs, SpaRedirectURIs: m.SpaRedirectURIs, PublicClientRedirectURIs: m.PublicClientRedirectURIs,
		RequestedAccessTokenVersion: m.RequestedAccessTokenVersion, Status: m.Status, StatusReason: m.StatusReason,
		RequestID: m.RequestID, ApplicationEntityID: m.ApplicationEntityID, ApplicationID: m.ApplicationID,
		ApplicationObjectID: m.ApplicationObjectID,
	}
}
