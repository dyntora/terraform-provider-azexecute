package provider

import (
	"context"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func (r *applicationResource) ModifyPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse) {
	if r.client == nil || request.Plan.Raw.IsNull() {
		return
	}

	var plan applicationResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}

	capabilities, err := r.client.Capabilities(ctx)
	if err != nil {
		response.Diagnostics.AddError("Unable to validate AZExecute tenant policy", err.Error())
		return
	}

	if capabilities.MinimumAdditionalOwners > 0 {
		// Optional+computed owners become unknown in the plan when omitted.
		// Read configuration to distinguish omission from a deferred reference.
		response.Diagnostics.Append(request.Config.GetAttribute(ctx, path.Root("owner_object_ids"), &plan.OwnerObjectIDs)...)
		if response.Diagnostics.HasError() {
			return
		}
	}
	errors := validateSynchronousApplicationPlan(plan, capabilities, request.State.Raw.IsNull())
	validateApplicationDetailsPlan(ctx, request, response, capabilities)
	if len(errors) > 0 {
		response.Diagnostics.AddError(
			"Invalid application configuration for this AZExecute tenant",
			strings.Join(errors, "\n"))
	}
}

func validateSynchronousApplicationPlan(model applicationResourceModel, capabilities *azclient.Capabilities, creating bool) []string {
	errors := validateApplicationPlan(model, capabilities, creating)
	if creating && capabilities.UseApplicationRequestFlow {
		errors = append(errors, "azexecute_application only supports automatic provisioning, but this tenant requires approval; use azexecute_application_request for an approval-aware asynchronous request")
	}
	return errors
}

func validateApplicationPlan(model applicationResourceModel, capabilities *azclient.Capabilities, creating bool) []string {
	var errors []string
	if !capabilities.Enabled {
		errors = append(errors, "the AZExecute Terraform API is disabled by the tenant policy")
	}
	if creating && !capabilities.AllowApplicationCreation {
		errors = append(errors, "Terraform application creation is disabled by the tenant policy")
	}
	if boolValue(model.ConfigureRegistration, false) && !capabilities.AllowRegistrationConfiguration {
		errors = append(errors, "configure_registration cannot be enabled because registration configuration is disabled by the tenant policy")
	}
	if creating && boolValue(model.ConfigureRegistration, false) && !capabilities.SupportsRegistrationRequests {
		errors = append(errors, "this API does not support registration configuration in requests; upgrade AZExecute before submitting configure_registration = true")
	}
	if setIsConfigured(model.ExposedScopes) || setIsConfigured(model.PreAuthorizedApplications) {
		if !boolValue(model.ConfigureRegistration, false) {
			errors = append(errors, "exposed_scopes and pre_authorized_applications require configure_registration = true")
		}
		if _, err := scopesFromModel(context.Background(), model.ExposedScopes, nil); fullyKnownSet(model.ExposedScopes) && err != nil {
			errors = append(errors, err.Error())
		}
		if _, err := clientsFromModel(context.Background(), model.PreAuthorizedApplications, nil); fullyKnownSet(model.PreAuthorizedApplications) && err != nil {
			errors = append(errors, err.Error())
		}
	}
	if setIsConfigured(model.AppRoles) {
		if !boolValue(model.ConfigureRegistration, false) {
			errors = append(errors, "app_roles requires configure_registration = true")
		} else if _, err := appRolesFromModel(context.Background(), model.AppRoles, nil); err != nil {
			errors = append(errors, err.Error())
		}
	}
	if boolValue(model.ConfigureRegistration, false) && setIsConfigured(model.AppRoles) && setIsConfigured(model.ExposedScopes) &&
		fullyKnownSet(model.AppRoles) && fullyKnownSet(model.ExposedScopes) {
		roles, roleErr := appRolesFromModel(context.Background(), model.AppRoles, nil)
		scopes, scopeErr := scopesFromModel(context.Background(), model.ExposedScopes, nil)
		if roleErr == nil && scopeErr == nil {
			if err := validateRoleAndScopeValues(roles, scopes); err != nil {
				errors = append(errors, err.Error())
			}
		}
	}
	if !model.APIPermissionRequests.IsNull() && !model.APIPermissionRequests.IsUnknown() && len(model.APIPermissionRequests.Elements()) > 0 && !capabilities.AllowAPIPermissionRequests {
		errors = append(errors, "api_permission_request cannot be used because API permission requests are disabled by the tenant policy")
	}

	stringFields := map[string]types.String{
		"business_justification":             model.BusinessJustification,
		"project_name":                       model.ProjectName,
		"department_owner":                   model.DepartmentOwner,
		"environment":                        model.Environment,
		"intended_audience":                  model.IntendedAudience,
		"technical_requirements":             model.TechnicalRequirements,
		"data_access_requirements":           model.DataAccessRequirements,
		"compliance_notes":                   model.ComplianceNotes,
		"elevated_permissions_justification": model.ElevatedPermissionsJustification,
		"contact_email":                      model.ContactEmail,
		"contact_phone":                      model.ContactPhone,
	}
	required := make(map[string]struct{}, len(capabilities.RequiredMetadataFields))
	for _, field := range capabilities.RequiredMetadataFields {
		required[field] = struct{}{}
	}
	for field, value := range stringFields {
		if _, ok := required[field]; !ok {
			continue
		}
		if field == "elevated_permissions_justification" && !boolValue(model.RequiresElevatedPermissions, false) {
			continue
		}
		if missingString(value) {
			errors = append(errors, fmt.Sprintf("%s is required by the tenant metadata policy", field))
		}
	}
	if _, ok := required["expected_go_live_date"]; ok && missingString(model.ExpectedGoLiveDate) {
		errors = append(errors, "expected_go_live_date is required by the tenant metadata policy")
	}

	errors = append(errors, validateStringLength("display_name", model.DisplayName, 1, 200)...)
	errors = append(errors, validateStringLength("description", model.Description, 0, 500)...)
	errors = append(errors, validateStringLength("business_justification", model.BusinessJustification, 5, 1000)...)
	errors = append(errors, validateStringLength("technical_requirements", model.TechnicalRequirements, 0, 500)...)
	errors = append(errors, validateStringLength("intended_audience", model.IntendedAudience, 0, 200)...)
	errors = append(errors, validateStringLength("data_access_requirements", model.DataAccessRequirements, 0, 500)...)
	errors = append(errors, validateStringLength("compliance_notes", model.ComplianceNotes, 0, 300)...)
	errors = append(errors, validateStringLength("project_name", model.ProjectName, 0, 100)...)
	errors = append(errors, validateStringLength("department_owner", model.DepartmentOwner, 0, 100)...)
	errors = append(errors, validateStringLength("elevated_permissions_justification", model.ElevatedPermissionsJustification, 0, 500)...)
	errors = append(errors, validateStringLength("environment", model.Environment, 0, 50)...)
	errors = append(errors, validateStringLength("contact_email", model.ContactEmail, 0, 200)...)
	errors = append(errors, validateStringLength("contact_phone", model.ContactPhone, 0, 20)...)

	if !model.ContactEmail.IsNull() && !model.ContactEmail.IsUnknown() && strings.TrimSpace(model.ContactEmail.ValueString()) != "" {
		if _, err := mail.ParseAddress(model.ContactEmail.ValueString()); err != nil {
			errors = append(errors, "contact_email must be a valid email address")
		}
	}
	if !model.ExpectedGoLiveDate.IsNull() && !model.ExpectedGoLiveDate.IsUnknown() {
		value := model.ExpectedGoLiveDate.ValueString()
		if _, err := time.Parse(time.RFC3339, value); err != nil {
			if _, dateErr := time.Parse("2006-01-02", value); dateErr != nil {
				errors = append(errors, "expected_go_live_date must be an RFC 3339 date or timestamp")
			}
		}
	}
	if !model.BusinessCriticality.IsNull() && !model.BusinessCriticality.IsUnknown() {
		value := model.BusinessCriticality.ValueInt64()
		if value < 1 || value > 5 {
			errors = append(errors, "business_criticality must be between 1 and 5")
		}
	}
	if value := modelIntOr(model.PollIntervalSeconds, 5); value < 1 || value > 300 {
		errors = append(errors, "poll_interval_seconds must be between 1 and 300")
	}
	if value := modelIntOr(model.CreateTimeoutMinutes, 60); value < 1 || value > 1440 {
		errors = append(errors, "create_timeout_minutes must be between 1 and 1440")
	}
	if capabilities.MinimumAdditionalOwners > 0 && !model.OwnerObjectIDs.IsUnknown() && (creating || !model.OwnerObjectIDs.IsNull()) && fullyKnownSet(model.OwnerObjectIDs) {
		owners := make(map[string]struct{})
		for _, element := range model.OwnerObjectIDs.Elements() {
			if value, ok := element.(types.String); ok && !value.IsNull() {
				owners[strings.ToLower(value.ValueString())] = struct{}{}
			}
		}
		if int64(len(owners)) < capabilities.MinimumAdditionalOwners {
			errors = append(errors, fmt.Sprintf("owner_object_ids must contain at least %d additional Entra user owners required by tenant policy. The requester and service principals do not count; the API verifies owner identities during apply. Supply owners here when creating the request, rather than adding them later with azexecute_application_owner resources.", capabilities.MinimumAdditionalOwners))
		}
	}
	if setIsConfigured(model.OwnerObjectIDs) && fullyKnownSet(model.OwnerObjectIDs) {
		for _, element := range model.OwnerObjectIDs.Elements() {
			value, ok := element.(types.String)
			if !ok || value.IsNull() || value.IsUnknown() || !uuidPattern.MatchString(value.ValueString()) {
				errors = append(errors, "owner_object_ids must contain Microsoft Entra object UUIDs")
				break
			}
		}
	}

	return errors
}

func missingString(value types.String) bool {
	return value.IsNull() || value.IsUnknown() || strings.TrimSpace(value.ValueString()) == "" || strings.EqualFold(strings.TrimSpace(value.ValueString()), "Not provided")
}

func validateRoleAndScopeValues(roles []azclient.AppRoleConfiguration, scopes []azclient.PermissionScopeConfiguration) error {
	values := make(map[string]bool, len(roles))
	for _, role := range roles {
		values[strings.ToLower(strings.TrimSpace(role.Value))] = true
	}
	for _, scope := range scopes {
		value := strings.TrimSpace(scope.Value)
		if value != "" && values[strings.ToLower(value)] {
			return fmt.Errorf("permission value %q is used by both app_roles and exposed_scopes; app roles and exposed scopes must have distinct values", value)
		}
	}
	return nil
}

func validateStringLength(name string, value types.String, minimum, maximum int) []string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	length := len([]rune(strings.TrimSpace(value.ValueString())))
	if minimum > 0 && length < minimum {
		return []string{fmt.Sprintf("%s must contain at least %d characters", name, minimum)}
	}
	if maximum > 0 && length > maximum {
		return []string{fmt.Sprintf("%s must contain no more than %d characters", name, maximum)}
	}
	return nil
}

func fullyKnownSet(value types.Set) bool {
	v, err := value.ToTerraformValue(context.Background())
	return err == nil && v.IsFullyKnown()
}
