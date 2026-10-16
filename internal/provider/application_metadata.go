package provider

import (
	"fmt"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func metadataDifferences(plan applicationResourceModel, result *azclient.Application) []string {
	actual := plan
	mapApplicationMetadataToModel(result.Metadata, &actual)
	fields := []struct {
		name              string
		planned, returned attr.Value
		computed          bool
	}{
		{"business_justification", plan.BusinessJustification, actual.BusinessJustification, true},
		{"technical_requirements", plan.TechnicalRequirements, actual.TechnicalRequirements, false},
		{"intended_audience", plan.IntendedAudience, actual.IntendedAudience, false},
		{"data_access_requirements", plan.DataAccessRequirements, actual.DataAccessRequirements, false},
		{"compliance_notes", plan.ComplianceNotes, actual.ComplianceNotes, false},
		{"expected_go_live_date", plan.ExpectedGoLiveDate, actual.ExpectedGoLiveDate, false},
		{"project_name", plan.ProjectName, actual.ProjectName, false},
		{"department_owner", plan.DepartmentOwner, actual.DepartmentOwner, false},
		{"business_criticality", plan.BusinessCriticality, actual.BusinessCriticality, true},
		{"requires_elevated_permissions", plan.RequiresElevatedPermissions, actual.RequiresElevatedPermissions, true},
		{"elevated_permissions_justification", plan.ElevatedPermissionsJustification, actual.ElevatedPermissionsJustification, false},
		{"environment", plan.Environment, actual.Environment, false},
		{"contact_email", plan.ContactEmail, actual.ContactEmail, false},
		{"contact_phone", plan.ContactPhone, actual.ContactPhone, false},
	}
	var differences []string
	for _, field := range fields {
		if field.planned.IsUnknown() || field.computed && field.planned.IsNull() {
			continue
		}
		if !field.planned.Equal(field.returned) {
			differences = append(differences, field.name)
		}
	}
	return differences
}

// Report the fields and durable identity, never the potentially sensitive values.
// Call after storing the actual response so an error does not lose the resource.
func diagnoseMetadataResult(plan applicationResourceModel, result *azclient.Application, diagnostics *diag.Diagnostics) {
	fields := metadataDifferences(plan, result)
	if len(fields) == 0 {
		return
	}
	diagnostics.AddError("AZExecute did not retain the requested metadata",
		fmt.Sprintf("Request %d (Terraform resource %s) exists, but AZExecute returned different values for: %s. The returned identity and metadata have been recorded in Terraform state.\n\nCheck the request and upgrade AZExecute if it is discarding these fields, then run terraform plan. Ready applications support metadata correction in place.\n\nA failed create may be marked for replacement by Terraform. To keep the existing application, reconcile/import its resource before applying a replacement plan. Recreating an application does not fix a metadata mismatch.", result.RequestID, result.ResourceID, strings.Join(fields, ", ")))
}
