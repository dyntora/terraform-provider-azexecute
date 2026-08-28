package provider

import (
	"context"
	"strings"
	"testing"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestNewUUIDIsStableFormatAndUnique(t *testing.T) {
	t.Parallel()
	first, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newUUID()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 36 || strings.Count(first, "-") != 4 {
		t.Fatalf("invalid UUID format: %s", first)
	}
	if first == second {
		t.Fatal("generated duplicate UUID")
	}
}

func TestExpectedGoLiveDatePreservesEquivalentConfiguredForm(t *testing.T) {
	t.Parallel()
	server := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)
	result := expectedGoLiveDateValue(types.StringValue("2026-09-01"), server)
	if result.ValueString() != "2026-09-01" {
		t.Fatalf("configured date changed unexpectedly: %s", result.ValueString())
	}
	imported := expectedGoLiveDateValue(types.StringNull(), server)
	if imported.ValueString() != "2026-09-01T00:00:00Z" {
		t.Fatalf("unexpected imported date: %s", imported.ValueString())
	}
}

func TestMetadataDefaultsAndValidation(t *testing.T) {
	t.Parallel()
	model := applicationResourceModel{BusinessJustification: types.StringValue("Needed for deployment"), BusinessCriticality: types.Int64Null(), RequiresElevatedPermissions: types.BoolNull()}
	metadata, err := metadataFromModel(model)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.BusinessCriticality != 3 || metadata.RequiresElevatedPermissions {
		t.Fatalf("unexpected defaults: %#v", metadata)
	}
	model.BusinessCriticality = types.Int64Value(6)
	if _, err := metadataFromModel(model); err == nil {
		t.Fatal("expected criticality validation error")
	}
}

func TestValidateApplicationPlanUsesTenantMetadataRequirements(t *testing.T) {
	t.Parallel()
	model := applicationResourceModel{
		DisplayName:           types.StringValue("Pipeline application"),
		BusinessJustification: types.StringUnknown(),
		ProjectName:           types.StringNull(),
		APIPermissionRequests: types.SetNull(types.StringType),
	}
	capabilities := &azclient.Capabilities{
		Enabled:                  true,
		AllowApplicationCreation: true,
		RequiredMetadataFields:   []string{"project_name"},
	}

	errors := validateApplicationPlan(model, capabilities, true)
	if len(errors) != 1 || !strings.Contains(errors[0], "project_name") {
		t.Fatalf("expected a project_name tenant-policy error, got %#v", errors)
	}
}

func TestValidateApplicationPlanRejectsDisabledOperationsBeforeApply(t *testing.T) {
	t.Parallel()
	model := applicationResourceModel{
		DisplayName:           types.StringValue("Pipeline application"),
		BusinessJustification: types.StringNull(),
		ConfigureRegistration: types.BoolValue(true),
		APIPermissionRequests: types.SetNull(types.StringType),
	}
	capabilities := &azclient.Capabilities{
		Enabled:                        true,
		AllowApplicationCreation:       false,
		AllowRegistrationConfiguration: false,
	}

	errors := validateApplicationPlan(model, capabilities, true)
	joined := strings.Join(errors, "\n")
	if !strings.Contains(joined, "creation is disabled") || !strings.Contains(joined, "configure_registration") {
		t.Fatalf("expected operation policy errors, got %#v", errors)
	}
}

func TestSynchronousApplicationRejectsApprovalTenantButRequestDoesNot(t *testing.T) {
	t.Parallel()
	model := applicationResourceModel{
		DisplayName:           types.StringValue("Pipeline application"),
		APIPermissionRequests: types.SetNull(types.StringType),
	}
	capabilities := &azclient.Capabilities{
		Enabled:                   true,
		AllowApplicationCreation:  true,
		UseApplicationRequestFlow: true,
	}

	synchronousErrors := validateSynchronousApplicationPlan(model, capabilities, true)
	if len(synchronousErrors) != 1 || !strings.Contains(synchronousErrors[0], "azexecute_application_request") {
		t.Fatalf("expected approval-aware resource guidance, got %#v", synchronousErrors)
	}
	if requestErrors := validateApplicationPlan(model, capabilities, true); len(requestErrors) != 0 {
		t.Fatalf("approval-aware request should accept an approval tenant, got %#v", requestErrors)
	}
}

func TestTenantControlledMetadataIsNeverStaticallyRequired(t *testing.T) {
	t.Parallel()
	providerSchema := managedApplicationSchema(false)
	metadataFields := []string{
		"business_justification",
		"technical_requirements",
		"intended_audience",
		"data_access_requirements",
		"compliance_notes",
		"expected_go_live_date",
		"project_name",
		"department_owner",
		"business_criticality",
		"requires_elevated_permissions",
		"elevated_permissions_justification",
		"environment",
		"contact_email",
		"contact_phone",
	}

	for _, name := range metadataFields {
		attribute, ok := providerSchema.Attributes[name]
		if !ok {
			t.Fatalf("tenant metadata field %q is missing from the resource schema", name)
		}

		optional, required := attributeFlags(attribute)
		if !optional || required {
			t.Errorf("tenant metadata field %q must be Optional and not Required", name)
		}
	}

	optional, required := attributeFlags(providerSchema.Attributes["display_name"])
	if optional || !required {
		t.Fatal("display_name must remain the only statically required top-level application field")
	}

	for name, attribute := range providerSchema.Attributes {
		if name == "display_name" {
			continue
		}

		_, required := attributeFlags(attribute)
		if required {
			t.Errorf("top-level application attribute %q must not be statically required", name)
		}
	}
}

func attributeFlags(attribute schema.Attribute) (optional bool, required bool) {
	switch value := attribute.(type) {
	case schema.StringAttribute:
		return value.Optional, value.Required
	case schema.Int64Attribute:
		return value.Optional, value.Required
	case schema.BoolAttribute:
		return value.Optional, value.Required
	case schema.SetAttribute:
		return value.Optional, value.Required
	default:
		return false, false
	}
}

func TestApplicationRequestStateConversionDropsOnlyWaitSettings(t *testing.T) {
	t.Parallel()
	source := applicationResourceModel{
		ID:                   types.StringValue("11111111-2222-4333-8444-555555555555"),
		DisplayName:          types.StringValue("Existing application"),
		Status:               types.StringValue("Ready"),
		ApplicationID:        types.StringValue("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"),
		PollIntervalSeconds:  types.Int64Value(5),
		CreateTimeoutMinutes: types.Int64Value(60),
	}

	var request applicationRequestResourceModel
	request.setFromApplicationModel(source)
	roundTrip := request.toApplicationModel()

	if roundTrip.ID != source.ID || roundTrip.DisplayName != source.DisplayName ||
		roundTrip.Status != source.Status || roundTrip.ApplicationID != source.ApplicationID {
		t.Fatalf("request state migration lost managed values: %#v", roundTrip)
	}
	if !roundTrip.PollIntervalSeconds.IsNull() || !roundTrip.CreateTimeoutMinutes.IsNull() {
		t.Fatal("asynchronous request unexpectedly retained synchronous wait settings")
	}
}

func TestApplicationRequestMoveStateAcceptsSynchronousResource(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	sourceSchema := managedApplicationSchema(true)
	targetSchema := managedApplicationSchema(false)
	source := applicationResourceModel{
		ID:                   types.StringValue("11111111-2222-4333-8444-555555555555"),
		DisplayName:          types.StringValue("Existing application"),
		Status:               types.StringValue("Ready"),
		PollIntervalSeconds:  types.Int64Value(5),
		CreateTimeoutMinutes: types.Int64Value(60),
	}
	sourceState := tfsdk.State{
		Raw:    tftypes.NewValue(sourceSchema.Type().TerraformType(ctx), nil),
		Schema: sourceSchema,
	}
	for attribute, value := range map[string]any{
		"id":                     source.ID.ValueString(),
		"display_name":           source.DisplayName.ValueString(),
		"status":                 source.Status.ValueString(),
		"poll_interval_seconds":  source.PollIntervalSeconds.ValueInt64(),
		"create_timeout_minutes": source.CreateTimeoutMinutes.ValueInt64(),
	} {
		if diagnostics := sourceState.SetAttribute(ctx, path.Root(attribute), value); diagnostics.HasError() {
			t.Fatalf("unable to set source attribute %s: %#v", attribute, diagnostics)
		}
	}

	response := resource.MoveStateResponse{TargetState: tfsdk.State{
		Raw:    tftypes.NewValue(targetSchema.Type().TerraformType(ctx), nil),
		Schema: targetSchema,
	}}
	mover := (&applicationRequestResource{}).MoveState(ctx)[0]
	mover.StateMover(ctx, resource.MoveStateRequest{
		SourceProviderAddress: "registry.terraform.io/dyntora/azexecute",
		SourceSchemaVersion:   0,
		SourceState:           &sourceState,
		SourceTypeName:        "azexecute_application",
	}, &response)
	if response.Diagnostics.HasError() {
		t.Fatalf("state move failed: %#v", response.Diagnostics)
	}

	var target applicationRequestResourceModel
	if diagnostics := response.TargetState.Get(ctx, &target); diagnostics.HasError() {
		t.Fatalf("unable to read target state: %#v", diagnostics)
	}
	if target.ID != source.ID || target.DisplayName != source.DisplayName || target.Status != source.Status {
		t.Fatalf("moved state lost resource values: %#v", target)
	}
}

func TestRegistrationUpdatePreservesUnmanagedFields(t *testing.T) {
	t.Parallel()
	version := int64(2)
	current := &azclient.Application{Registration: &azclient.RegistrationConfiguration{
		SignInAudience: "AzureADMyOrg", ConcurrencyToken: "etag", AppRoles: []any{map[string]any{"id": "role"}},
		API: azclient.APIConfiguration{RequestedAccessTokenVersion: &version, Scopes: []any{map[string]any{"id": "scope"}}},
	}}
	model := applicationResourceModel{BusinessJustification: types.StringValue("Needed for deployment"), ConfigureRegistration: types.BoolValue(true), SignInAudience: types.StringValue("AzureADMultipleOrgs")}
	update, err := updateRequestFromModel(model, current)
	if err != nil {
		t.Fatal(err)
	}
	if update.Registration.SignInAudience != "AzureADMultipleOrgs" || update.Registration.ConcurrencyToken != "etag" {
		t.Fatalf("registration merge lost values: %#v", update.Registration)
	}
	if len(update.Registration.AppRoles) != 1 || len(update.Registration.API.Scopes) != 1 {
		t.Fatal("unmanaged roles or scopes were not preserved")
	}
}
