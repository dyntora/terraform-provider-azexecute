package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// Exercise Terraform's protocol and state rules, including failure recovery.
// The local API intentionally injects routing failures and pending deletions.
func TestRelatedResourcesTerraformLifecycle(t *testing.T) {
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
	for _, kind := range []string{"group", "membership", "listener", "aws", "vercel", "github"} {
		t.Run(kind, func(t *testing.T) {
			var mu sync.Mutex
			var stored map[string]any
			id := ""
			creates, updates, deletes := 0, 0, 0
			readStatus, deleteStatus := 200, 204
			gone := false
			numericID := 7
			target := "11111111-2222-4333-8444-555555555555"
			absence := "terraform_integration_not_found"
			resourceType := "azexecute_" + kind + "_integration"
			fields := ""
			switch kind {
			case "group":
				resourceType = "azexecute_azgroup"
				absence = "terraform_group_not_found"
				fields = "source = \"local\""
			case "membership":
				resourceType = "azexecute_azgroup_membership"
				absence = "terraform_group_membership_not_found"
				fields = fmt.Sprintf("group_id = 7\n type = \"user\"\n target_id = %q", target)
			case "listener":
				resourceType = "azexecute_event_listener"
				absence = "event_listener_not_found"
				fields = fmt.Sprintf("event_type = \"ApplicationCredentialRenewed\"\n application_entity_id = %q\n automation_task_id = 42", target)
			case "aws":
				fields = "service = \"route53\"\n account_id = \"123456789012\"\n role_arn = \"arn:aws:iam::123456789012:role/terraform\""
			case "vercel":
				fields = "team_id = \"team_test\"\n access_token = \"test-token\""
			case "github":
				fields = "app_id = \"123\"\n installation_id = \"456\"\n private_key_pem = \"test-key\""
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if r.Method == "GET" && readStatus != 200 {
					w.WriteHeader(readStatus)
					fmt.Fprint(w, `{"title":"proxy error"}`)
					return
				}
				if r.Method == "DELETE" {
					if deleteStatus != 204 {
						w.WriteHeader(deleteStatus)
						fmt.Fprint(w, `{"status":"pending"}`)
						return
					}
					deletes++
					gone = true
					w.WriteHeader(204)
					return
				}
				if gone {
					w.WriteHeader(404)
					_ = json.NewEncoder(w).Encode(map[string]string{"code": absence})
					return
				}
				if r.Method == "POST" || r.Method == "PUT" {
					if stored == nil {
						creates++
					} else {
						updates++
					}
					if err := json.NewDecoder(r.Body).Decode(&stored); err != nil {
						t.Error(err)
					}
					if id == "" {
						id = r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
					}
				}
				var result any
				switch kind {
				case "group":
					result = map[string]any{"resourceId": id, "groupId": numericID, "actualSource": "Local", "status": "Ready", "displayName": stored["name"], "configuration": stored}
				case "membership":
					id = "7/user/" + target
					result = map[string]any{"groupId": 7, "targetId": target, "role": stored["role"]}
				case "listener":
					id = "7"
					copy := map[string]any{}
					for k, v := range stored {
						copy[k] = v
					}
					copy["id"] = numericID
					result = copy
				default:
					safe := map[string]any{}
					for k, v := range stored {
						if k != "accessToken" && k != "privateKeyPem" && k != "webhookSecret" && k != "externalId" {
							safe[k] = v
						}
					}
					result = map[string]any{"resourceId": id, "integrationId": numericID, "kind": kind, "configuration": safe}
				}
				_ = json.NewEncoder(w).Encode(result)
			}))
			defer server.Close()
			dir := t.TempDir()
			write := func(name, value string) {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write("terraform.rc", fmt.Sprintf("provider_installation {\n dev_overrides {\n \"dyntora/azexecute\" = %q\n }\n direct {}\n}\n", filepath.ToSlash(pluginDir)))
			command := func(args ...string) ([]byte, error) {
				cmd := exec.Command(terraform, args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+filepath.Join(dir, "terraform.rc"), "TF_IN_AUTOMATION=1", "CHECKPOINT_DISABLE=1")
				return cmd.CombinedOutput()
			}
			run := func(args ...string) []byte {
				output, err := command(args...)
				if err != nil {
					t.Fatalf("terraform %v: %v\n%s", args, err, output)
				}
				return output
			}
			config := func(edit bool) {
				mutable := `name = "Original"`
				if edit {
					mutable = `name = "Renamed"`
				}
				if kind == "membership" {
					mutable = `role = "Member"`
					if edit {
						mutable = `role = "Viewer"`
					}
				}
				write("main.tf", fmt.Sprintf("terraform {\n required_providers {\n azexecute = { source = \"dyntora/azexecute\" }\n }\n}\nprovider \"azexecute\" {\n endpoint = %q\n access_token = \"test-token\"\n}\nresource %q \"test\" {\n%s\n%s\n}\n", server.URL, resourceType, mutable, fields))
			}
			address := resourceType + ".test"
			config(false)
			run("apply", "-auto-approve", "-input=false", "-no-color")
			run("plan", "-detailed-exitcode", "-input=false", "-no-color")
			mu.Lock()
			originalID := id
			mu.Unlock()
			config(true)
			run("plan", "-out=update.tfplan", "-input=false", "-no-color")
			output := run("show", "-json", "update.tfplan")
			var plan struct {
				ResourceChanges []struct {
					Address string
					Change  struct{ Actions []string }
				} `json:"resource_changes"`
			}
			if err := json.Unmarshal(output, &plan); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, change := range plan.ResourceChanges {
				if change.Address == address {
					found = strings.Join(change.Change.Actions, ",") == "update"
				}
			}
			if !found {
				t.Fatalf("expected only in-place update: %s", output)
			}
			run("apply", "-auto-approve", "-input=false", "-no-color", "update.tfplan")
			run("plan", "-detailed-exitcode", "-input=false", "-no-color")
			for _, status := range []int{403, 404} {
				mu.Lock()
				readStatus = status
				mu.Unlock()
				output, err := command("apply", "-refresh-only", "-auto-approve", "-input=false", "-no-color")
				if err == nil {
					t.Fatalf("HTTP %d refresh must fail: %s", status, output)
				}
				if !strings.Contains(string(run("state", "list")), address) {
					t.Fatal("failed refresh discarded state")
				}
			}
			mu.Lock()
			readStatus = 200
			mu.Unlock()
			if kind != "membership" {
				mu.Lock()
				numericID = 99
				mu.Unlock()
				output, err := command("plan", "-input=false", "-no-color")
				if err == nil {
					t.Fatalf("changed identity accepted: %s", output)
				}
				mu.Lock()
				numericID = 7
				mu.Unlock()
			}
			run("plan", "-detailed-exitcode", "-input=false", "-no-color")
			run("state", "rm", address)
			run("import", "-input=false", "-no-color", address, originalID)
			// Imported integration secrets are intentionally absent from reads; configure
			// them once to resume management, without replacing the integration.
			run("apply", "-auto-approve", "-input=false", "-no-color")
			run("plan", "-detailed-exitcode", "-input=false", "-no-color")
			mu.Lock()
			deleteStatus = 202
			mu.Unlock()
			output, err := command("destroy", "-auto-approve", "-input=false", "-no-color")
			if err == nil || !strings.Contains(string(output), "did not confirm deletion") {
				t.Fatalf("unconfirmed destroy accepted: %v\n%s", err, output)
			}
			if !strings.Contains(string(run("state", "list")), address) {
				t.Fatal("pending deletion discarded state")
			}
			mu.Lock()
			deleteStatus = 204
			mu.Unlock()
			run("destroy", "-auto-approve", "-input=false", "-no-color")
			run("destroy", "-auto-approve", "-input=false", "-no-color")
			mu.Lock()
			defer mu.Unlock()
			if id != originalID || creates != 1 || updates < 1 || deletes != 1 {
				t.Fatalf("unexpected lifecycle: id=%s original=%s create=%d update=%d delete=%d", id, originalID, creates, updates, deletes)
			}
		})
	}
}
