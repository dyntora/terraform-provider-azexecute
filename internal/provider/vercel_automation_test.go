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
	"testing"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestVercelStepAndDeliveryLifecyclePreservesIndependentResources(t *testing.T) {
	const app = "11111111-2222-4333-8444-555555555555"
	const collection = "/api/terraform/v1/tasks/7/vercel-steps"
	const delivery = "/api/terraform/v1/managed-applications/" + app + "/vercel-delivery"
	var step azclient.VercelStep
	var targets []azclient.VercelDestination
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case collection, collection + "/9":
			switch r.Method {
			case "POST", "PUT":
				if (r.Method == "POST") != (r.URL.Path == collection) {
					t.Error("wrong step mutation path")
				}
				var input azclient.VercelStepInput
				if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
					t.Error(err)
				}
				step = azclient.VercelStep{TaskID: 7, StepID: 9, VercelStepInput: input}
			case "DELETE":
				step = azclient.VercelStep{}
				w.WriteHeader(204)
				return
			}
			if step.StepID == 0 {
				w.WriteHeader(404)
				fmt.Fprint(w, `{"code":"terraform_vercel_step_not_found"}`)
				return
			}
			json.NewEncoder(w).Encode(step)
		case delivery:
			switch r.Method {
			case "PUT":
				if err := json.NewDecoder(r.Body).Decode(&targets); err != nil {
					t.Error(err)
				}
			case "DELETE":
				targets = nil
				w.WriteHeader(204)
				return
			}
			json.NewEncoder(w).Encode(azclient.VercelDelivery{ApplicationID: app, Destinations: targets})
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	api, err := azclient.New(server.URL, "", "test", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	envs := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("env_uat"), types.StringValue("env_stage")})
	stepResource := &vercelStepResource{client: api}
	deliveryResource := &vercelDeliveryResource{client: api}
	var stepSchema, deliverySchema resource.SchemaResponse
	stepResource.Schema(ctx, resource.SchemaRequest{}, &stepSchema)
	deliveryResource.Schema(ctx, resource.SchemaRequest{}, &deliverySchema)
	stepState := tfsdk.State{Schema: stepSchema.Schema, Raw: tftypes.NewValue(stepSchema.Schema.Type().TerraformType(ctx), nil)}
	deliveryState := tfsdk.State{Schema: deliverySchema.Schema, Raw: tftypes.NewValue(deliverySchema.Schema.Type().TerraformType(ctx), nil)}
	m := vercelStepModel{TaskID: types.Int64Value(7), StepID: types.Int64Unknown(), Integration: types.Int64Value(12), Operation: types.StringValue("update_variable"), Project: types.StringValue("prj_1"), Environments: envs, Variable: types.StringValue("CLIENT_SECRET"), Reference: types.StringValue("{{ NewSecret }}"), Enabled: types.BoolValue(true), Stop: types.BoolValue(true)}
	if d := stepState.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	model := destinationModel{Integration: m.Integration, Project: m.Project, Variable: m.Variable, Environments: envs, Redeploy: types.SetValueMust(types.StringType, []attr.Value{types.StringValue("env_uat")})}
	element := deliverySchema.Schema.Attributes["destinations"].GetType().(types.ListType).ElemType
	list, d := types.ListValueFrom(ctx, element, []destinationModel{model})
	if d.HasError() {
		t.Fatal(d)
	}
	if d := deliveryState.Set(ctx, &vercelDeliveryModel{Application: types.StringValue(app), Destinations: list}); d.HasError() {
		t.Fatal(d)
	}
	created := resource.CreateResponse{State: stepState}
	stepResource.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: stepState.Schema, Raw: stepState.Raw}}, &created)
	if created.Diagnostics.HasError() || step.Action.ValueReference != "{{ NewSecret }}" {
		t.Fatalf("create step: %v", created.Diagnostics)
	}
	configured := resource.CreateResponse{State: deliveryState}
	deliveryResource.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan{Schema: deliveryState.Schema, Raw: deliveryState.Raw}}, &configured)
	if configured.Diagnostics.HasError() || len(targets) != 1 || targets[0].DeploymentTargets != "env_uat" {
		t.Fatalf("configure delivery: %v", configured.Diagnostics)
	}
	imported := resource.ImportStateResponse{State: tfsdk.State{Schema: stepState.Schema, Raw: tftypes.NewValue(stepState.Schema.Type().TerraformType(ctx), nil)}}
	stepResource.ImportState(ctx, resource.ImportStateRequest{ID: "7/9"}, &imported)
	if imported.Diagnostics.HasError() {
		t.Fatal(imported.Diagnostics)
	}
	refreshed := resource.ReadResponse{State: imported.State}
	stepResource.Read(ctx, resource.ReadRequest{State: imported.State}, &refreshed)
	if refreshed.Diagnostics.HasError() || !refreshed.State.Raw.Equal(created.State.Raw) {
		t.Fatalf("import lost step settings: %v", refreshed.Diagnostics)
	}
	deliveryRead := resource.ReadResponse{State: configured.State}
	deliveryResource.Read(ctx, resource.ReadRequest{State: configured.State}, &deliveryRead)
	if deliveryRead.Diagnostics.HasError() || !deliveryRead.State.Raw.Equal(configured.State.Raw) {
		t.Fatalf("delivery drift on refresh: %v", deliveryRead.Diagnostics)
	}
	m.StepID = types.Int64Value(9)
	m.Project = types.StringNull()
	m.Environments = types.SetNull(types.StringType)
	m.Variable = types.StringNull()
	m.Reference = types.StringNull()
	m.Operation = types.StringValue("rotate_access_token")
	m.Target = types.Int64Value(19)
	if d := stepState.Set(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	updated := resource.UpdateResponse{State: refreshed.State}
	stepResource.Update(ctx, resource.UpdateRequest{State: refreshed.State, Plan: tfsdk.Plan{Schema: stepState.Schema, Raw: stepState.Raw}}, &updated)
	if updated.Diagnostics.HasError() || step.Action.TargetIntegrationID != 19 || step.Action.ValueReference != "" {
		t.Fatalf("operation switch: %v", updated.Diagnostics)
	}
	removed := resource.DeleteResponse{}
	deliveryResource.Delete(ctx, resource.DeleteRequest{State: configured.State}, &removed)
	if removed.Diagnostics.HasError() || targets != nil || step.StepID != 9 {
		t.Fatalf("remove delivery: %v", removed.Diagnostics)
	}
	stepResource.Delete(ctx, resource.DeleteRequest{State: updated.State}, &removed)
	if removed.Diagnostics.HasError() || step.StepID != 0 {
		t.Fatalf("remove step: %v", removed.Diagnostics)
	}
	t.Run("terraform_cli", func(t *testing.T) {
		cli, plugins := os.Getenv("AZEXECUTE_TERRAFORM_EXECUTABLE"), os.Getenv("AZEXECUTE_TERRAFORM_PLUGIN_DIR")
		if cli == "" || plugins == "" {
			t.Skip("set Terraform executable and plugin directory to run the CLI lifecycle")
		}
		dir := t.TempDir()
		write := func(name, content string) {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
		}
		write("terraform.rc", fmt.Sprintf("provider_installation {\n dev_overrides {\n \"dyntora/azexecute\" = %q\n }\n direct {}\n}\n", filepath.ToSlash(plugins)))
		write("main.tf", fmt.Sprintf(`terraform {
  required_providers { azexecute = { source = "dyntora/azexecute" } }
}
provider "azexecute" {
  endpoint = %q
  access_token = "test"
}
resource "azexecute_vercel_task_step" "test" {
  task_id = 7
  integration_id = 12
  operation = "update_variable"
  project_id = "prj_1"
  environments = ["env_uat", "env_stage"]
  variable_name = "CLIENT_SECRET"
  value_reference = "{{ NewSecret }}"
}
resource "azexecute_application_vercel_delivery" "test" {
  application_entity_id = %q
  destinations = [{integration_id = 12, project_id = "prj_1", variable_name = "CLIENT_SECRET", environments = ["env_uat","env_stage"], redeploy_environments = ["env_uat"]}]
}
`, server.URL, app))
		run := func(args ...string) {
			cmd := exec.Command(cli, args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(), "TF_CLI_CONFIG_FILE="+filepath.Join(dir, "terraform.rc"), "CHECKPOINT_DISABLE=1", "TF_IN_AUTOMATION=1")
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("terraform %v: %v\n%s", args, err, output)
			}
		}
		run("apply", "-auto-approve", "-input=false", "-no-color")
		run("plan", "-detailed-exitcode", "-input=false", "-no-color")
		run("state", "rm", "azexecute_vercel_task_step.test", "azexecute_application_vercel_delivery.test")
		run("import", "-input=false", "azexecute_vercel_task_step.test", "7/9")
		run("import", "-input=false", "azexecute_application_vercel_delivery.test", app)
		run("plan", "-detailed-exitcode", "-input=false", "-no-color")
		run("destroy", "-auto-approve", "-input=false", "-no-color")
	})
}
