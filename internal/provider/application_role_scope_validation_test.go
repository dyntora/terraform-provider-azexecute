package provider

import (
	"context"
	"strings"
	"testing"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestRoleScopeCollisionsAreRejectedDuringPlanAndCreate(t *testing.T) {
	for _, test := range []struct {
		role, scope string
		collision   bool
	}{
		{"Shipments.Read", "Shipments.Read", true},
		{"Schedules.Read", "Schedules.Read", true},
		{"Invoices.Read", "Invoices.Read", true},
		{"Shipments.Read", "shipments.READ", true},
		{"Declarations.Read", "Declarations.ReadWrite", false},
		{"Gate.Operate", "Gate.Access", false},
		{"Shipments.Read", "Shipments.Read.Delegated", false},
	} {
		t.Run(test.role+"/"+test.scope, func(t *testing.T) {
			ctx := context.Background()
			model := roleScopeModel(t, test.role, test.scope)
			capabilities := &azclient.Capabilities{Enabled: true, AllowApplicationCreation: true,
				AllowRegistrationConfiguration: true, SupportsRegistrationRequests: true}
			errors := validateApplicationPlan(model, capabilities, true)
			if (len(errors) > 0) != test.collision {
				t.Fatalf("unexpected plan validation: %v", errors)
			}
			if test.collision && !strings.Contains(strings.Join(errors, " "), "both app_roles and exposed_scopes") {
				t.Fatalf("missing collision guidance: %v", errors)
			}
			_, err := createRequestFromModel(ctx, model, "test-resource")
			if (err != nil) != test.collision {
				t.Fatalf("unexpected creation validation: %v", err)
			}
		})
	}
}

func TestRoleScopeCollisionChecksIncludeUnmanagedExistingPermissions(t *testing.T) {
	for _, manageRoles := range []bool{false, true} {
		model := roleScopeModel(t, "Shipments.Read", "Shipments.Read")
		current := &azclient.Application{Registration: &azclient.RegistrationConfiguration{
			AppRoles: []azclient.AppRoleConfiguration{{Value: "Shipments.Read"}},
			API:      azclient.APIConfiguration{Scopes: []azclient.PermissionScopeConfiguration{{Value: "Shipments.Read"}}},
		}}
		if manageRoles {
			model.ExposedScopes = types.SetNull(scopeObjectType())
		} else {
			model.AppRoles = types.SetNull(appRoleObjectType())
		}
		_, err := updateRequestFromModel(context.Background(), model, current)
		if err == nil || !strings.Contains(err.Error(), "both app_roles and exposed_scopes") {
			t.Fatalf("manageRoles=%v: expected collision with preserved permissions, got %v", manageRoles, err)
		}
	}
}

func TestRoleScopePlanDefersUnknownCollections(t *testing.T) {
	for _, unknownRoles := range []bool{false, true} {
		model := roleScopeModel(t, "Shipments.Read", "Shipments.Read")
		if unknownRoles {
			model.AppRoles = types.SetUnknown(appRoleObjectType())
		} else {
			model.ExposedScopes = types.SetUnknown(scopeObjectType())
		}
		errors := validateApplicationPlan(model, &azclient.Capabilities{Enabled: true, AllowApplicationCreation: true,
			AllowRegistrationConfiguration: true, SupportsRegistrationRequests: true}, true)
		if len(errors) > 0 {
			t.Fatalf("unknownRoles=%v: unknown values must be deferred until apply: %v", unknownRoles, errors)
		}
	}
}

func roleScopeModel(t *testing.T, roleValue, scopeValue string) applicationResourceModel {
	t.Helper()
	scopes, diagnostics := types.SetValueFrom(context.Background(), scopeObjectType(), []scopeModel{{
		ID: types.StringValue("11111111-2222-4333-8444-555555555555"), Value: types.StringValue(scopeValue),
		AdminConsentDisplayName: types.StringValue("Access API"), AdminConsentDescription: types.StringValue("Access this API"),
		ConsentType: types.StringValue("Admin"), IsEnabled: types.BoolValue(true),
	}})
	if diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	return applicationResourceModel{
		DisplayName: types.StringValue("API"), ConfigureRegistration: types.BoolValue(true),
		AppRoles: appRoleSet(t, appRoleModel{
			ID: types.StringValue("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"), DisplayName: types.StringValue("Reader"),
			Value: types.StringValue(roleValue), Description: types.StringValue("Read data"),
			IsEnabled: types.BoolValue(true), AllowApplications: types.BoolValue(true),
		}),
		ExposedScopes:             scopes,
		PreAuthorizedApplications: types.SetNull(clientObjectType()),
	}
}
