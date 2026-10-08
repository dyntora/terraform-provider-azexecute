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
	for _, resourceType := range []string{"azexecute_application_request", "azexecute_application"} {
		t.Run(resourceType, func(t *testing.T) {
			var mu sync.Mutex
			clientID, objectID := "11111111-2222-4333-8444-555555555555", "22222222-2222-4333-8444-555555555555"
			app := azclient.Application{}
			creates, deletes, updates := 0, 0, 0
			supportsUpdates := true
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/terraform/v1/capabilities" {
					_ = json.NewEncoder(w).Encode(azclient.Capabilities{Enabled: true, AllowApplicationCreation: true, SupportsApplicationDetailsUpdates: supportsUpdates})
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
						Metadata: create.Metadata, Status: "Ready", RequestID: 42, ApplicationID: &clientID, ApplicationObjectID: &objectID}
				case "PUT":
					updates++
					var update azclient.ApplicationUpdate
					if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
						t.Error(err)
					}
					app.Metadata = update.Metadata
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
					t.Error("metadata edit deleted the application")
				case "GET":
				default:
					t.Errorf("unexpected method: %s", r.Method)
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
				write("main.tf", fmt.Sprintf(`terraform {
  required_providers {
    azexecute = { source = "dyntora/azexecute" }
  }
}
provider "azexecute" {
  endpoint = %q
  access_token = "local-test-token"
}
resource %q "test" {
  display_name = %q
  business_justification = "Test metadata updates"
  %s
}
`, server.URL, resourceType, name, metadata))
			}
			config("Original", `description = "Original description"`)
			run("apply", "-auto-approve", "-input=false", "-no-color")
			mu.Lock()
			originalID := app.ResourceID
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
						Change struct{ Actions []string } `json:"change"`
					} `json:"resource_changes"`
				}
				if err := json.Unmarshal(output, &plan); err != nil {
					t.Fatalf("plan JSON: %v\n%s", err, output)
				}
				if len(plan.ResourceChanges) != 1 || !reflect.DeepEqual(plan.ResourceChanges[0].Change.Actions, []string{"update"}) {
					t.Fatalf("expected in-place update: %s", output)
				}
				run("apply", "-auto-approve", "-input=false", "-no-color", "update.tfplan")
				run("plan", "-detailed-exitcode", "-input=false", "-no-color") // exit 0 = clean
				mu.Lock()
				if app.ResourceID != originalID || *app.ApplicationID != clientID || *app.ApplicationObjectID != objectID || creates != 1 || deletes != 0 {
					t.Error("application identity or lifecycle changed")
				}
				mu.Unlock()
			}
			mu.Lock()
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
			if totalUpdates != 6 {
				t.Fatalf("unexpected update count: %d", totalUpdates)
			}
		})
	}
}
