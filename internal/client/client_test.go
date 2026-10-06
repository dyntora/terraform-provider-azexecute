package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCapabilitiesUsesBearerTokenAndVersionedPath(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/terraform/v1/capabilities" {
			t.Errorf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected authorization header")
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(Capabilities{APIVersion: "1", Enabled: true, AllowApplicationCreation: true})
	}))
	defer server.Close()

	api, err := New(server.URL, "ignored", "test-token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := api.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Enabled || !result.AllowApplicationCreation || result.APIVersion != "1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRetriesTransientResponsesWithReusablePostBody(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var payload ApplicationCreate
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("attempt body could not be decoded: %v", err)
		}
		if payload.ResourceID != "retry-id" {
			t.Errorf("request body was not preserved: %#v", payload)
		}
		if attempts.Add(1) < 3 {
			response.Header().Set("Retry-After", "0")
			response.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(response).Encode(Application{ResourceID: payload.ResourceID, DisplayName: "retry", Status: "Ready"})
	}))
	defer server.Close()
	api, _ := New(server.URL, "ignored", "token", nil, time.Second)
	result, err := api.CreateApplication(context.Background(), ApplicationCreate{ResourceID: "retry-id", DisplayName: "retry"})
	if err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 3 || result.Status != "Ready" {
		t.Fatalf("unexpected retry result after %d attempts: %#v", attempts.Load(), result)
	}
}

func TestCreateApplicationAcceptsAsyncResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", request.Method)
		}
		var payload ApplicationCreate
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.ResourceID != "resource-id" || payload.DisplayName != "example" {
			t.Errorf("unexpected payload: %#v", payload)
		}
		response.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(response).Encode(Application{ResourceID: payload.ResourceID, DisplayName: payload.DisplayName, Status: "PendingApproval"})
	}))
	defer server.Close()
	api, _ := New(server.URL, "ignored", "token", nil, time.Second)
	result, err := api.CreateApplication(context.Background(), ApplicationCreate{ResourceID: "resource-id", DisplayName: "example"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "PendingApproval" {
		t.Fatalf("unexpected status: %s", result.Status)
	}
}

func TestProblemDetailsAreReturnedAsTypedError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/problem+json")
		response.WriteHeader(http.StatusNotFound)
		_, _ = response.Write([]byte(`{"title":"terraform_resource_not_found","detail":"missing","traceId":"abc"}`))
	}))
	defer server.Close()
	api, _ := New(server.URL, "ignored", "token", nil, time.Second)
	_, err := api.GetApplication(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Fatalf("expected typed not-found error, got %v", err)
	}
}

func TestValidationErrorsPreserveAllFieldsAndOneReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"title":"Validation failed","detail":"Review the configuration. Reference: abc.","traceId":"abc","code":"terraform_validation_failed","errors":{"registration.appRoles[1].value":["Choose a unique role value."],"metadata.projectName":["Project Name is required.","Project Name is required."],"registration.appRoles[0].value":["Supply a role value."]}}`))
	}))
	defer server.Close()
	api, _ := New(server.URL, "ignored", "token", nil, time.Second)
	_, err := api.CreateApplication(context.Background(), ApplicationCreate{})
	if err == nil {
		t.Fatal("expected validation failure")
	}
	message := err.Error()
	for _, expected := range []string{"project_name: Project Name is required.", "app_roles[0].value: Supply a role value.", "app_roles[1].value: Choose a unique role value.", "terraform_validation_failed"} {
		if !strings.Contains(message, expected) {
			t.Errorf("missing %q in %s", expected, message)
		}
	}
	if strings.Count(message, "abc") != 1 || strings.Count(message, "Project Name is required.") != 1 {
		t.Fatalf("duplicate diagnostic: %s", message)
	}
	if strings.Index(message, "project_name") > strings.Index(message, "app_roles[0]") {
		t.Fatalf("unstable field order: %s", message)
	}
}

func TestErrorResponseShapesAndCorrelationFallback(t *testing.T) {
	for _, body := range []string{`{"errors":{"metadata.contactEmail":["Supply a valid email address."]}}`, `"Supply a valid email address."`, `<html>private proxy diagnostics</html>`, ""} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("X-Correlation-ID", "header-trace")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()
			api, _ := New(server.URL, "ignored", "token", nil, time.Second)
			_, err := api.Capabilities(context.Background())
			if err == nil || !strings.Contains(err.Error(), "header-trace") {
				t.Fatalf("missing correlation: %v", err)
			}
			if strings.Contains(body, "valid email") && !strings.Contains(err.Error(), "Supply a valid email address.") {
				t.Fatalf("lost guidance: %v", err)
			}
			if strings.Contains(err.Error(), "private proxy") {
				t.Fatalf("exposed raw response: %v", err)
			}
		})
	}
}

func TestApplicationRequestSerializesEmptyCollectionsAsArrays(t *testing.T) {
	payload, err := json.Marshal(ApplicationCreate{Registration: &RegistrationConfiguration{}})
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]any
	if err := json.Unmarshal(payload, &actual); err != nil {
		t.Fatal(err)
	}
	registration := actual["registration"].(map[string]any)
	collections := []any{actual["apiPermissionRequests"], registration["identifierUris"], registration["appRoles"]}
	for _, platform := range []string{"web", "spa", "publicClient"} {
		collections = append(collections, registration[platform].(map[string]any)["redirectUris"])
	}
	api := registration["api"].(map[string]any)
	collections = append(collections, api["scopes"], api["preAuthorizedApplications"])
	for _, collection := range collections {
		if values, ok := collection.([]any); !ok || len(values) != 0 {
			t.Fatalf("expected empty array, got %#v; JSON: %s", collection, payload)
		}
	}
	if _, exists := actual["ownerObjectIds"]; exists {
		t.Fatal("unmanaged ownership must remain omitted")
	}
}

func TestValidationFieldNamesMatchTerraformAttributes(t *testing.T) {
	for field, expected := range map[string]string{
		"$.Metadata.ContactEmail":                             "contact_email",
		"Registration.Api.Scopes[0].AdminConsentDisplayName":  "exposed_scopes[0].admin_consent_display_name",
		"registration.api.preAuthorizedApplications[1].appId": "pre_authorized_applications[1].app_id",
		"registration.web.redirectUris[0]":                    "web_redirect_uris[0]",
		"registration.spa.redirectUris[0]":                    "spa_redirect_uris[0]",
		"registration.publicClient.redirectUris[0]":           "public_client_redirect_uris[0]",
		"apiPermissionRequests[0].targetExternalApiAppId":     "api_permission_request[0].target_external_api_app_id",
		"OwnerObjectIds": "owner_object_ids",
	} {
		if actual := terraformFieldName(field); actual != expected {
			t.Errorf("%s: got %s, want %s", field, actual, expected)
		}
	}
}

func TestRejectsInsecureRemoteEndpoint(t *testing.T) {
	t.Parallel()
	if _, err := New("http://example.com", "scope", "token", nil, time.Second); err == nil {
		t.Fatal("expected insecure endpoint to be rejected")
	}
}

func TestApplicationOwnerUsesAtomicChildEndpoints(t *testing.T) {
	t.Parallel()
	const resourceID = "11111111-2222-4333-8444-555555555555"
	const ownerID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/terraform/v1/applications/"+resourceID+"/owners/"+ownerID {
			t.Errorf("unexpected owner path: %s", request.URL.Path)
		}
		methods = append(methods, request.Method)
		if request.Method == http.MethodDelete {
			response.WriteHeader(http.StatusNoContent)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(ApplicationOwner{ResourceID: resourceID, OwnerObjectID: ownerID})
	}))
	defer server.Close()

	api, _ := New(server.URL, "ignored", "token", nil, time.Second)
	if _, err := api.AddApplicationOwner(context.Background(), resourceID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := api.GetApplicationOwner(context.Background(), resourceID, ownerID); err != nil {
		t.Fatal(err)
	}
	if err := api.RemoveApplicationOwner(context.Background(), resourceID, ownerID); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(methods, ","); got != "PUT,GET,DELETE" {
		t.Fatalf("unexpected method sequence: %s", got)
	}
}
