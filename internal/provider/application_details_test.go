package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestDetailsUpdatePayloadAndOldServerProtection(t *testing.T) {
	ctx := context.Background()
	old := "Original description"
	current := &azclient.Application{DisplayName: "Original", Description: &old}
	model := applicationResourceModel{DisplayName: types.StringValue("Original"), Description: types.StringValue(old)}
	update, err := updateRequestFromModel(ctx, model, current)
	if err != nil || update.Details != nil {
		t.Fatalf("unchanged details should be omitted: %#v %v", update, err)
	}
	model.Description = types.StringNull()
	update, err = updateRequestFromModel(ctx, model, current)
	if err != nil || update.Details == nil || !update.Details.UpdateDescription || update.Details.Description != nil || update.Details.DisplayName != nil {
		t.Fatalf("description removal was lost: %#v %v", update.Details, err)
	}
	var mutations int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			mutations++
			t.Errorf("old server received mutation: %s", r.Method)
		}
		fmt.Fprint(w, `{ "enabled": true }`)
	}))
	defer server.Close()
	client, _ := azclient.New(server.URL, "scope", "test-token", nil, time.Second)
	_, err = client.UpdateApplication(ctx, "existing-resource", update)
	if err == nil || !strings.Contains(err.Error(), "upgrade AZExecute") || mutations != 0 {
		t.Fatalf("unsupported update not blocked: %v", err)
	}
}

func TestDetailsPlanRequiresCapabilityOnlyForExistingChanges(t *testing.T) {
	ctx := context.Background()
	for _, syncResource := range []bool{false, true} {
		schema := managedApplicationSchema(syncResource)
		state := tfsdk.State{Schema: schema, Raw: tftypes.NewValue(schema.Type().TerraformType(ctx), nil)}
		if d := state.SetAttribute(ctx, path.Root("display_name"), "Original"); d.HasError() {
			t.Fatal(d)
		}
		plan := tfsdk.Plan{Schema: schema, Raw: state.Raw}
		if d := plan.SetAttribute(ctx, path.Root("description"), "New"); d.HasError() {
			t.Fatal(d)
		}
		response := resource.ModifyPlanResponse{Plan: plan}
		validateApplicationDetailsPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan}, &response, &azclient.Capabilities{})
		if !response.Diagnostics.HasError() {
			t.Fatal("old API must be rejected during plan")
		}
		response = resource.ModifyPlanResponse{Plan: plan}
		validateApplicationDetailsPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan}, &response, &azclient.Capabilities{SupportsApplicationDetailsUpdates: true})
		if response.Diagnostics.HasError() || len(response.RequiresReplace) != 0 {
			t.Fatal(response)
		}
	}
}

// Runs the actual Terraform plan/apply protocol against a local API, without an
// Azure account. Set AZEXECUTE_TERRAFORM_EXECUTABLE to enable this integration test.
func TestApplicationDetailsTerraformLifecycle(t *testing.T) {
	terraform := os.Getenv("AZEXECUTE_TERRAFORM_EXECUTABLE")
	if terraform == "" {
		t.Skip("set AZEXECUTE_TERRAFORM_EXECUTABLE to run Terraform CLI lifecycle coverage")
	}
	pluginDir := t.TempDir()
	executable := "terraform-provider-azexecute"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}
	build := exec.Command("go", "build", "-o", filepath.Join(pluginDir, executable), ".")
	build.Dir = "../.."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build provider: %v\n%s", err, output)
	}
	for _, scenario := range []struct {
		name, resourceType string
		waitLater          bool
	}{
		{"request", "azexecute_application_request", false},
		{"application", "azexecute_application", false},
		{"request_wait_later", "azexecute_application_request", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			resourceType := scenario.resourceType
			waitEnabled := !scenario.waitLater
			var mu sync.Mutex
			clientID, objectID := "11111111-2222-4333-8444-555555555555", "22222222-2222-4333-8444-555555555555"
			app := azclient.Application{}
			completeOnRead := false
			readsBeforeCompletion := 0
			creates, deletes, updates := 0, 0, 0
			supportsUpdates := true
			preservesMetadata := true
			discardMetadata := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/terraform/v1/capabilities" {
					_ = json.NewEncoder(w).Encode(azclient.Capabilities{Enabled: true, AllowApplicationCreation: true, AllowApplicationDeletion: true,
						SupportsRegistrationRequests: true, AllowRegistrationConfiguration: true, AllowAPIPermissionRequests: true,
						RequiredMetadataFields: []string{"business_justification"}, SupportsApplicationDetailsUpdates: supportsUpdates, PreservesHiddenMetadata: preservesMetadata, IncludedMetadataFields: []string{}})
					return
				}
				switch r.Method {
				case "POST":
					creates++
					var create azclient.ApplicationCreate
					if err := json.NewDecoder(r.Body).Decode(&create); err != nil {
						t.Error(err)
					}
					app = azclient.Application{ResourceID: create.ResourceID, DisplayName: create.DisplayName, Description: create.Description,
						Metadata: create.Metadata, Status: "Provisioning", RequestID: 42, RequestedRegistration: create.Registration}
					app.OwnerObjectIDs = *create.OwnerObjectIDs
					app.RequestedAPIPermissions = create.APIPermissionRequests
					for i := range app.OwnerObjectIDs {
						app.OwnerObjectIDs[i] = strings.ToLower(app.OwnerObjectIDs[i])
					}
					if resourceType == "azexecute_application_request" {
						app.Status = "PendingApproval"
					}
					completeOnRead = true
				case "PUT":
					updates++
					var update azclient.ApplicationUpdate
					if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
						t.Error(err)
					}
					app.Metadata = update.Metadata
					if update.OwnerObjectIDs != nil {
						app.OwnerObjectIDs = *update.OwnerObjectIDs
					}
					if update.Registration != nil {
						app.Registration = update.Registration
					}
					if discardMetadata {
						app.Metadata.RequiresElevatedPermissions = false
						app.Metadata.ElevatedPermissionsJustification = nil
					}
					if update.Details != nil {
						if update.Details.DisplayName != nil {
							app.DisplayName = *update.Details.DisplayName
						}
						if update.Details.UpdateDescription {
							app.Description = update.Details.Description
							if app.Description != nil && *app.Description == "" {
								app.Description = nil
							}
						}
					}
				case "DELETE":
					deletes++
					if deletes == 1 {
						w.WriteHeader(http.StatusConflict)
						fmt.Fprint(w, `{"code":"directory_deletion_unconfirmed","detail":"Entra deletion is not yet confirmed."}`)
						return
					}
					app = azclient.Application{}
					w.WriteHeader(http.StatusNoContent)
					return
				case "GET":
					if app.ResourceID == "" {
						w.WriteHeader(http.StatusNotFound)
						fmt.Fprint(w, `{"code":"terraform_resource_not_found"}`)
						return
					}
					if completeOnRead {
						if readsBeforeCompletion > 0 {
							readsBeforeCompletion--
						}
					}
					if completeOnRead && readsBeforeCompletion == 0 {
						completeOnRead = false
						app.Status, app.ApplicationID, app.ApplicationObjectID = "Ready", &clientID, &objectID
						registration := *app.RequestedRegistration
						app.Registration = &registration
						app.Registration.ApplicationID = clientID
						// Match ApplicationRegistrationRequest.ResolveIdentifiers on the API:
						// exposed scopes receive a generated identifier when none was supplied.
						if len(app.Registration.IdentifierUris) == 0 && len(app.Registration.API.Scopes) > 0 {
							app.Registration.IdentifierUris = []azclient.URIValue{{Value: "api://" + clientID}}
						}
					}
				default:
					t.Errorf("unexpected method: %s", r.Method)
				}
				app.DisplayName = strings.TrimSpace(app.DisplayName)
				app.Metadata.BusinessJustification = strings.TrimSpace(app.Metadata.BusinessJustification)
				if app.Registration != nil {
					app.Registration.Web.HomePageURL, app.Registration.Web.LogoutURL = nil, nil
				}
				_ = json.NewEncoder(w).Encode(app)
			}))
			defer server.Close()
			dir := t.TempDir()
			configPath := filepath.Join(dir, "terraform.rc")
			write := func(name, content string) {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("terraform.rc", fmt.Sprintf("provider_installation {\n dev_overrides {\n  \"dyntora/azexecute\" = %q\n }\n direct {}\n}\n", filepath.ToSlash(pluginDir)))
			command := func(args ...string) ([]byte, error) {
				cmd := exec.Command(terraform, args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+configPath, "TF_IN_AUTOMATION=1", "CHECKPOINT_DISABLE=1")
				return cmd.CombinedOutput()
			}
			run := func(args ...string) []byte {
				output, err := command(args...)
				if err != nil {
					t.Fatalf("terraform %v: %v\n%s", args, err, output)
				}
				return output
			}
			config := func(name, metadata string) {
				wait := ""
				if resourceType == "azexecute_application_request" {
					wait = fmt.Sprintf("wait_for_completion = %t", waitEnabled)
				}
				write("main.tf", fmt.Sprintf(`terraform {
  required_providers {
    azexecute = { source = "dyntora/azexecute" }
  }
}
provider "azexecute" {
  endpoint = %q
  access_token = "local-test-token"
}
resource "terraform_data" "metadata" {
  input = "  Test metadata updates  "
}
resource %q "test" {
  display_name = %q
  business_justification = terraform_data.metadata.output
  contact_phone = ""
  %s
  poll_interval_seconds = 1
  configure_registration = true
  sign_in_audience = "AzureADMyOrg"
  owner_object_ids = ["AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE"]
  web_home_page_url = ""
  web_logout_url = ""
  web_redirect_uris = ["https://example.com/callback"]
  app_roles = [{
    id = "AAAAAAAA-1111-4222-8333-EEEEEEEEEEEE"
    display_name = "Read"
    value = "Read.All"
    description = "Read application data"
    is_enabled = true
    allow_users_and_groups = false
    allow_applications = true
  }]
  exposed_scopes = [{
    id = "AAAAAAAA-2222-4333-8444-EEEEEEEEEEEE"
    value = "access_as_user"
    admin_consent_display_name = "Access application"
    admin_consent_description = "Access this API on behalf of a user"
    consent_type = "Admin"
    is_enabled = true
  }]
  pre_authorized_applications = [{
    application_id = "AAAAAAAA-3333-4444-8555-EEEEEEEEEEEE"
    delegated_permission_ids = ["AAAAAAAA-2222-4333-8444-EEEEEEEEEEEE"]
  }]
  api_permission_request {
    target_type = "ExternalApi"
    target_external_api_app_id = "00000003-0000-0000-c000-000000000000"
    grant_type = "AppRole"
    permission {
      id = "df021288-bdef-4463-88db-98f22de89214"
      requires_admin_consent = true
    }
  }
  requires_elevated_permissions = true
  elevated_permissions_justification = "Read employee profiles for roster eligibility."
  %s
}
`, server.URL, resourceType, name, wait, metadata))
			}
			config(" Original ", `description = "Original description"`)
			run("apply", "-auto-approve", "-input=false", "-no-color")
			if scenario.waitLater {
				mu.Lock()
				if app.Status != "PendingApproval" || app.ApplicationID != nil {
					t.Error("asynchronous creation did not retain pending request")
				}
				readsBeforeCompletion = 3 // refresh, update preflight, then the wait completes
				mu.Unlock()
				waitEnabled = true
				config(" Original ", `description = "Original description"`)
				run("apply", "-auto-approve", "-input=false", "-no-color")
				run("plan", "-detailed-exitcode", "-input=false", "-no-color")
			}
			mu.Lock()
			originalID := app.ResourceID
			if len(app.Registration.IdentifierUris) != 1 || app.Registration.IdentifierUris[0].Value != "api://"+clientID {
				t.Error("waiting erased the API-generated identifier URI")
			}
			mu.Unlock()
			for _, edit := range []struct{ name, metadata string }{
				{"Original", `description = "New description"`},
				{"Original", ""},                 // removal must explicitly clear description
				{"Original", `description = ""`}, // Graph may normalize empty to null
				{"Original", ""},
				{"Renamed", `project_name = "Project"`},
				{"Renamed", "project_name = \"Project\"\n department_owner = \"Engineering\""},
			} {
				config(edit.name, edit.metadata)
				run("plan", "-out=update.tfplan", "-input=false", "-no-color")
				output := run("show", "-json", "update.tfplan")
				var plan struct {
					ResourceChanges []struct {
						Address string                     `json:"address"`
						Change  struct{ Actions []string } `json:"change"`
					} `json:"resource_changes"`
				}
				if err := json.Unmarshal(output, &plan); err != nil {
					t.Fatalf("plan JSON: %v\n%s", err, output)
				}
				for _, change := range plan.ResourceChanges {
					if change.Address == resourceType+".test" && !reflect.DeepEqual(change.Change.Actions, []string{"update"}) {
						t.Fatalf("expected in-place update: %s", output)
					}
				}
				run("apply", "-auto-approve", "-input=false", "-no-color", "update.tfplan")
				run("plan", "-detailed-exitcode", "-input=false", "-no-color") // exit 0 = clean
				mu.Lock()
				if app.ResourceID != originalID || *app.ApplicationID != clientID || *app.ApplicationObjectID != objectID || creates != 1 || deletes != 0 {
					t.Error("application identity or lifecycle changed")
				}
				mu.Unlock()
			}
			// Reproduce the old API's erased metadata, then repair the same application.
			mu.Lock()
			app.Registration.AppRoles[0].Value = "Unexpected.Role"
			app.OwnerObjectIDs = []string{}
			mu.Unlock()
			run("apply", "-auto-approve", "-input=false", "-no-color")
			run("plan", "-detailed-exitcode", "-input=false", "-no-color")
			mu.Lock()
			if app.Registration.AppRoles[0].Value != "Read.All" || len(app.OwnerObjectIDs) != 1 {
				t.Error("owner/registration drift was not repaired")
			}
			app.Metadata.RequiresElevatedPermissions = false
			app.Metadata.ElevatedPermissionsJustification = nil
			discardMetadata = true
			mu.Unlock()
			failedOutput, failedErr := command("apply", "-auto-approve", "-input=false", "-no-color")
			if failedErr == nil || !strings.Contains(string(failedOutput), "AZExecute did not retain the requested metadata") || strings.Contains(string(failedOutput), "Provider produced inconsistent result") {
				t.Fatalf("missing useful metadata diagnostic: %v\n%s", failedErr, failedOutput)
			}
			mu.Lock()
			discardMetadata = false
			mu.Unlock()
			run("apply", "-auto-approve", "-input=false", "-no-color")
			run("plan", "-detailed-exitcode", "-input=false", "-no-color")
			mu.Lock()
			if !app.Metadata.RequiresElevatedPermissions || app.Metadata.ElevatedPermissionsJustification == nil || app.ResourceID != originalID || *app.ApplicationID != clientID || *app.ApplicationObjectID != objectID || creates != 1 || deletes != 0 {
				t.Error("metadata repair lost values or replaced the application")
			}
			preservesMetadata = false
			mu.Unlock()
			legacyOutput, legacyErr := command("plan", "-input=false", "-no-color")
			if legacyErr == nil || !strings.Contains(string(legacyOutput), "Upgrade AZExecute") {
				t.Fatalf("legacy metadata loss not blocked during planning: %v\n%s", legacyErr, legacyOutput)
			}
			mu.Lock()
			preservesMetadata = true
			supportsUpdates = false
			mu.Unlock()
			config("Renamed", `description = "Unsupported update"`)
			output, err := command("plan", "-input=false", "-no-color")
			if err == nil || !strings.Contains(string(output), "API upgrade required") {
				t.Fatalf("old API not blocked: %v\n%s", err, output)
			}
			mu.Lock()
			totalUpdates := updates
			mu.Unlock()
			if totalUpdates != 10 { // initial reconciliation, edits, ownership/role drift, metadata recovery
				t.Fatalf("unexpected update count: %d", totalUpdates)
			}
			mu.Lock()
			supportsUpdates = true
			mu.Unlock()
			config("Renamed", "project_name = \"Project\"\n department_owner = \"Engineering\"")
			run("state", "rm", resourceType+".test")
			run("import", "-input=false", "-no-color", resourceType+".test", originalID)
			run("apply", "-auto-approve", "-input=false", "-no-color")
			run("plan", "-detailed-exitcode", "-input=false", "-no-color")
			mu.Lock()
			if creates != 1 || deletes != 0 || app.ResourceID != originalID {
				t.Error("import replaced application")
			}
			mu.Unlock()
			// A prior failed creation can leave Terraform's taint mark. Its
			// replacement must finish delayed cleanup before creating a new ID.
			run("taint", "-no-color", resourceType+".test")
			run("apply", "-auto-approve", "-input=false", "-no-color")
			run("plan", "-detailed-exitcode", "-input=false", "-no-color")
			mu.Lock()
			if creates != 2 || deletes != 2 || app.ResourceID == originalID {
				t.Errorf("tainted replacement did not resume cleanup: creates=%d deletes=%d id=%s", creates, deletes, app.ResourceID)
			}
			mu.Unlock()
			run("destroy", "-auto-approve", "-input=false", "-no-color")
			run("destroy", "-auto-approve", "-input=false", "-no-color")
			mu.Lock()
			if deletes != 3 {
				t.Errorf("expected two replacement cleanup attempts and one final deletion, got %d attempts", deletes)
			}
			mu.Unlock()
		})
	}
}
