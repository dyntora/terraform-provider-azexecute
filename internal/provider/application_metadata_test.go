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
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestMetadataPlanCompatibilityDefersUnknownValues(t *testing.T) {
	caps := &azclient.Capabilities{Enabled: true, AllowApplicationCreation: true, IncludedMetadataFields: []string{}}
	model := applicationResourceModel{DisplayName: types.StringValue("API"), RequiresElevatedPermissions: types.BoolUnknown(), ElevatedPermissionsJustification: types.StringUnknown()}
	if errors := validateApplicationPlan(model, caps, true); len(errors) != 0 {
		t.Fatal(errors)
	}
	model.RequiresElevatedPermissions = types.BoolValue(true)
	if errors := validateApplicationPlan(model, caps, true); len(errors) != 1 || !strings.Contains(errors[0], "Upgrade AZExecute") {
		t.Fatal(errors)
	}
	caps.PreservesHiddenMetadata = true
	if errors := validateApplicationPlan(model, caps, true); len(errors) != 0 {
		t.Fatal(errors)
	}
}

func TestCreateRepairsMetadataOnceOrReportsDurableIdentity(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		for _, repairSucceeds := range []bool{false, true} {
			ctx := context.Background()
			writes := 0
			app := azclient.Application{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/terraform/v1/capabilities" {
					_ = json.NewEncoder(w).Encode(azclient.Capabilities{Enabled: true, AllowApplicationCreation: true, PreservesHiddenMetadata: true})
					return
				}
				switch r.Method {
				case http.MethodPost:
					var create azclient.ApplicationCreate
					_ = json.NewDecoder(r.Body).Decode(&create)
					app = azclient.Application{ResourceID: create.ResourceID, RequestID: 42, DisplayName: create.DisplayName, Metadata: create.Metadata, Status: "Ready"}
					app.Metadata.RequiresElevatedPermissions = false
					app.Metadata.ElevatedPermissionsJustification = nil
				case http.MethodPut:
					writes++
					var update azclient.ApplicationUpdate
					_ = json.NewDecoder(r.Body).Decode(&update)
					if repairSucceeds {
						app.Metadata = update.Metadata
					}
				default:
					t.Errorf("unexpected request: %s", r.Method)
				}
				_ = json.NewEncoder(w).Encode(app)
			}))
			api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
			schema := managedApplicationSchema(synchronous)
			plan := tfsdk.Plan{Schema: schema, Raw: tftypes.NewValue(schema.Type().TerraformType(ctx), nil)}
			justification := "Private business reason for profile access"
			for name, value := range map[string]any{"display_name": "API", "business_justification": "Provision roster API", "requires_elevated_permissions": true, "elevated_permissions_justification": justification} {
				if d := plan.SetAttribute(ctx, path.Root(name), value); d.HasError() {
					t.Fatal(d)
				}
			}
			response := resource.CreateResponse{State: tfsdk.State{Schema: schema, Raw: tftypes.NewValue(schema.Type().TerraformType(ctx), nil)}}
			if synchronous {
				(&applicationResource{client: api}).Create(ctx, resource.CreateRequest{Plan: plan}, &response)
			} else {
				(&applicationRequestResource{client: api}).Create(ctx, resource.CreateRequest{Plan: plan}, &response)
			}
			server.Close()
			if response.Diagnostics.HasError() == repairSucceeds || writes != 1 {
				t.Fatalf("repair=%v writes=%d diagnostics=%v", repairSucceeds, writes, response.Diagnostics)
			}
			var id types.String
			if d := response.State.GetAttribute(ctx, path.Root("id"), &id); d.HasError() {
				t.Fatal(d)
			}
			if id.ValueString() != app.ResourceID || id.ValueString() == "" {
				t.Fatal("lost durable identity")
			}
			if !repairSucceeds {
				message := response.Diagnostics[0].Detail()
				if !strings.Contains(message, "42") || !strings.Contains(message, "elevated_permissions_justification") || strings.Contains(message, justification) {
					t.Fatal(message)
				}
			}
		}
	}
}
