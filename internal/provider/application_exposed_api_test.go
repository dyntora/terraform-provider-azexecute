package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func requestedScopeSet(t *testing.T) types.Set {
	t.Helper()
	v, d := types.SetValueFrom(context.Background(), scopeObjectType(), []scopeModel{{
		ID: types.StringValue("11111111-2222-4333-8444-555555555555"), Value: types.StringValue("access_as_user"),
		AdminConsentDisplayName: types.StringValue("Access API"), AdminConsentDescription: types.StringValue("Access this API as the signed-in user"),
		UserConsentDisplayName: types.StringNull(), UserConsentDescription: types.StringNull(), ConsentType: types.StringValue("Admin"), IsEnabled: types.BoolValue(true),
	}})
	if d.HasError() {
		t.Fatal(d)
	}
	return v
}
func TestExposedScopesPreserveOmittedClearExplicitAndDetectDrift(t *testing.T) {
	ctx := context.Background()
	set := requestedScopeSet(t)
	expected, err := scopesFromModel(ctx, set, nil)
	if err != nil {
		t.Fatal(err)
	}
	preserved, err := scopesFromModel(ctx, types.SetNull(scopeObjectType()), expected)
	if err != nil || len(preserved) != 1 {
		t.Fatal("omitted scopes were not preserved", err)
	}
	cleared, err := scopesFromModel(ctx, types.SetValueMust(scopeObjectType(), nil), expected)
	if err != nil || cleared == nil || len(cleared) != 0 {
		t.Fatal("explicit empty did not clear", err)
	}
	desired := applicationResourceModel{ExposedScopes: set, PreAuthorizedApplications: types.SetNull(clientObjectType())}
	if exposedAPIDiffers(ctx, desired, azclient.APIConfiguration{Scopes: expected}) {
		t.Fatal("matching scopes differ")
	}
	expected[0].IsEnabled = false
	if !exposedAPIDiffers(ctx, desired, azclient.APIConfiguration{Scopes: expected}) {
		t.Fatal("scope drift was missed")
	}
}
func TestPendingAndRejectedRequestsReadStoredRegistrationWithoutLiveObject(t *testing.T) {
	ctx := context.Background()
	for _, status := range []string{"PendingApproval", "Provisioning", "Rejected"} {
		desired := applicationResourceModel{ConfigureRegistration: types.BoolValue(true), ExposedScopes: requestedScopeSet(t), IdentifierURIs: stringSet(t, "api://{applicationId}")}
		create, err := createRequestFromModel(ctx, desired, "11111111-2222-4333-8444-555555555555")
		if err != nil {
			t.Fatal(err)
		}
		source := &azclient.Application{Status: status, RequestedRegistration: create.Registration, Metadata: azclient.ApplicationMetadata{BusinessCriticality: 3}}
		var d diag.Diagnostics
		actual := desired
		mapApplicationToModel(ctx, source, &actual, &d)
		if d.HasError() || !actual.ExposedScopes.Equal(desired.ExposedScopes) || !actual.IdentifierURIs.Equal(desired.IdentifierURIs) {
			t.Fatalf("%s lost request configuration: %v", status, d)
		}
		if source.Registration != nil {
			t.Fatal("stored request was exposed as live registration")
		}
	}
}
func TestRequestCreateSubmitsWholeConfigurationWithoutPostApprovalRegistrationPut(t *testing.T) {
	for _, status := range []string{"PendingApproval", "Ready", "Rejected"} {
		t.Run(status, func(t *testing.T) {
			ctx := context.Background()
			var received azclient.ApplicationCreate
			var methods []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methods = append(methods, r.Method+" "+r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "capabilities") {
					json.NewEncoder(w).Encode(azclient.Capabilities{SupportsRegistrationRequests: true})
					return
				}
				if r.Method != "POST" {
					t.Errorf("unexpected registration mutation: %s", r.Method)
					w.WriteHeader(500)
					return
				}
				if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
					t.Error(err)
				}
				json.NewEncoder(w).Encode(azclient.Application{ResourceID: received.ResourceID, Status: status, DisplayName: received.DisplayName, Registration: func() *azclient.RegistrationConfiguration {
					if status == "Ready" {
						return received.Registration
					}
					return nil
				}(), RequestedRegistration: received.Registration, Metadata: received.Metadata})
			}))
			defer server.Close()
			api, err := azclient.New(server.URL, "ignored", "test", nil, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			s := managedApplicationSchema(false)
			plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
			for k, v := range map[string]any{"display_name": "API app", "configure_registration": true, "exposed_scopes": requestedScopeSet(t), "identifier_uris": stringSet(t, "api://{applicationId}")} {
				if d := plan.SetAttribute(ctx, path.Root(k), v); d.HasError() {
					t.Fatal(d)
				}
			}
			result := resource.CreateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}}
			(&applicationRequestResource{client: api}).Create(ctx, resource.CreateRequest{Plan: plan}, &result)
			if result.Diagnostics.HasError() {
				t.Fatal(result.Diagnostics)
			}
			if received.Registration == nil || len(received.Registration.API.Scopes) != 1 {
				t.Fatal("POST omitted requested scopes")
			}
			if len(methods) != 2 {
				t.Fatalf("unexpected API calls: %v", methods)
			}
			var state applicationRequestResourceModel
			if d := result.State.Get(ctx, &state); d.HasError() {
				t.Fatal(d)
			}
			if state.Status.ValueString() != status || state.ID.ValueString() == "" || !state.ExposedScopes.Equal(requestedScopeSet(t)) {
				t.Fatal("pending request did not retain identity/configuration")
			}
		})
	}
}

func TestRegistrationRequestRefusesOldAPIWithoutPosting(t *testing.T) {
	posted := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			posted = true
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"apiVersion":"1"}`))
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "ignored", "test", nil, time.Second)
	_, err := api.CreateApplication(context.Background(), azclient.ApplicationCreate{Registration: &azclient.RegistrationConfiguration{}})
	if err == nil || posted {
		t.Fatal("old server silently accepted unsupported request", err)
	}
}
func TestIdentifierTemplateConvergesAfterApproval(t *testing.T) {
	id := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	desired := stringSet(t, "api://{applicationId}")
	actual := identifiersWithTemplates(desired, []string{"api://" + id}, id)
	if configuredSetDiffers(desired, actual) {
		t.Fatal("generated client ID causes perpetual drift")
	}
}

func TestVersionTwoStateUpgradePreservesIdentityAndLeavesScopesUnmanaged(t *testing.T) {
	ctx := context.Background()
	for _, isRequest := range []bool{false, true} {
		priorSchema := managedApplicationSchemaV2(!isRequest)
		currentSchema := managedApplicationSchema(!isRequest)
		prior := tfsdk.State{Schema: priorSchema, Raw: tftypes.NewValue(priorSchema.Type().TerraformType(ctx), nil)}
		for k, v := range map[string]any{"id": "11111111-2222-4333-8444-555555555555", "status": "PendingApproval", "display_name": "Existing request", "app_roles": appRoleSet(t)} {
			if d := prior.SetAttribute(ctx, path.Root(k), v); d.HasError() {
				t.Fatal(d)
			}
		}
		result := resource.UpgradeStateResponse{State: tfsdk.State{Schema: currentSchema, Raw: tftypes.NewValue(currentSchema.Type().TerraformType(ctx), nil)}}
		upgraders := (&applicationResource{}).UpgradeState(ctx)
		if isRequest {
			upgraders = (&applicationRequestResource{}).UpgradeState(ctx)
		}
		upgraders[2].StateUpgrader(ctx, resource.UpgradeStateRequest{State: &prior}, &result)
		if result.Diagnostics.HasError() {
			t.Fatal(result.Diagnostics)
		}
		var id types.String
		var scopes types.Set
		if d := result.State.GetAttribute(ctx, path.Root("id"), &id); d.HasError() {
			t.Fatal(d)
		}
		if d := result.State.GetAttribute(ctx, path.Root("exposed_scopes"), &scopes); d.HasError() {
			t.Fatal(d)
		}
		if id.ValueString() != "11111111-2222-4333-8444-555555555555" || !scopes.IsNull() {
			t.Fatal("upgrade changed identity or took ownership of scopes")
		}
	}
}

func TestAdminScopesRejectExplicitEmptyConsentText(t *testing.T) {
	ctx := context.Background()
	var models []scopeModel
	if d := requestedScopeSet(t).ElementsAs(ctx, &models, false); d.HasError() {
		t.Fatal(d)
	}
	models[0].UserConsentDisplayName = types.StringValue("")
	value, d := types.SetValueFrom(ctx, scopeObjectType(), models)
	if d.HasError() {
		t.Fatal(d)
	}
	if _, err := scopesFromModel(ctx, value, nil); err == nil {
		t.Fatal("empty consent text would be normalized to null and break Terraform state consistency")
	}
}

func TestPreAuthorizedClientsRejectCaseInsensitiveDuplicateScopeIds(t *testing.T) {
	ctx := context.Background()
	id := "abcdefab-1234-4567-89ab-abcdefabcdef"
	value, d := types.SetValueFrom(ctx, clientObjectType(), []authorizedClientModel{{
		AppID:    types.StringValue("11111111-2222-4333-8444-555555555555"),
		ScopeIDs: stringSet(t, id, strings.ToUpper(id)),
	}})
	if d.HasError() {
		t.Fatal(d)
	}
	if _, err := clientsFromModel(ctx, value, nil); err == nil {
		t.Fatal("duplicate UUIDs would be collapsed by the API and break Terraform state consistency")
	}
}
