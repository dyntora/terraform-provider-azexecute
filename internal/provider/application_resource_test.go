package provider

import (
	"strings"
	"testing"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
