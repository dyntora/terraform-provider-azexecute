package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPermissionUpdatesPreserveRequestsAndRejectOldAPI(t *testing.T) {
	ctx := context.Background()
	target := "00000003-0000-0000-c000-000000000000"
	current := &azclient.Application{ResourceID: "existing", RequestedAPIPermissions: []azclient.APIPermissionRequest{{TargetType: "ExternalApi", TargetExternalAPIAppID: &target, GrantType: "AppRole", Permissions: []azclient.APIPermission{{ID: "aaaaaaaa-1111-4222-8333-eeeeeeeeeeee"}}}}}
	value := types.SetNull(permissionRequestObjectType())
	var diagnostics diag.Diagnostics
	mapPermissionRequests(ctx, current.RequestedAPIPermissions, &value, &diagnostics)
	if diagnostics.HasError() {
		t.Fatal(diagnostics)
	}
	update := azclient.ApplicationUpdate{}
	if err := permissionUpdateFromModel(ctx, value, current, &update); err != nil || update.APIPermissionRequests != nil {
		t.Fatalf("unchanged request must be omitted: %v %#v", err, update)
	}
	empty := types.SetValueMust(permissionRequestObjectType(), nil)
	if err := validatePermissionSnapshot(ctx, empty, value, current); err != nil {
		t.Fatal(err)
	}
	if err := validatePermissionSnapshot(ctx, empty, empty, current); err == nil {
		t.Fatal("concurrent server change must not be overwritten")
	}
	if err := validatePermissionSnapshot(ctx, value, empty, current); err != nil {
		t.Fatal("lost-response retry must remain idempotent")
	}
	legacy := &azclient.Application{ResourceID: "existing"}
	if err := validatePermissionSnapshot(ctx, value, value, legacy); err != nil {
		t.Fatal("legacy API must still support unrelated edits")
	}
	if err := validatePermissionSnapshot(ctx, empty, value, legacy); err == nil {
		t.Fatal("legacy API without a snapshot must reject permission edits")
	}
	if err := permissionUpdateFromModel(ctx, empty, current, &update); err != nil || update.APIPermissionRequests == nil || len(*update.APIPermissionRequests) != 0 || len(*update.ExpectedAPIPermissionRequests) != 1 {
		t.Fatalf("removal payload: %v %#v", err, update)
	}
	data, _ := json.Marshal(update)
	if !strings.Contains(string(data), `"apiPermissionRequests":[]`) {
		t.Fatalf("empty removal must survive serialization: %s", data)
	}
	warnPermissionRemoval(update, &diagnostics)
	if diagnostics.WarningsCount() != 1 {
		t.Fatal("removal must disclose retained grants")
	}
	mutations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			mutations++
			t.Errorf("old API received mutation")
		}
		fmt.Fprint(w, `{"enabled":true}`)
	}))
	defer server.Close()
	client, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
	if _, err := client.UpdateApplication(ctx, "existing", update); err == nil || !strings.Contains(err.Error(), "upgrade AZExecute") || mutations != 0 {
		t.Fatalf("saved-plan apply must fail closed: %v", err)
	}

	for _, sync := range []bool{false, true} {
		s := managedApplicationSchema(sync)
		state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
		if d := state.SetAttribute(ctx, path.Root("api_permission_request"), value); d.HasError() {
			t.Fatal(d)
		}
		plan := tfsdk.Plan{Schema: s, Raw: state.Raw}
		if d := plan.SetAttribute(ctx, path.Root("api_permission_request"), empty); d.HasError() {
			t.Fatal(d)
		}
		response := resource.ModifyPlanResponse{Plan: plan}
		validatePermissionRequestsPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan}, &response, &azclient.Capabilities{})
		if !response.Diagnostics.HasError() {
			t.Fatal("old API must fail during plan")
		}
		response = resource.ModifyPlanResponse{Plan: plan}
		validatePermissionRequestsPlan(ctx, resource.ModifyPlanRequest{State: state, Plan: plan}, &response, &azclient.Capabilities{SupportsPermissionRequestUpdates: true})
		if response.Diagnostics.HasError() || len(response.RequiresReplace) != 0 {
			t.Fatal(response)
		}
		block := s.Blocks["api_permission_request"].(schema.SetNestedBlock)
		for _, modifier := range block.PlanModifiers {
			result := planmodifier.SetResponse{PlanValue: empty}
			modifier.PlanModifySet(ctx, planmodifier.SetRequest{StateValue: value, PlanValue: empty, ConfigValue: empty, State: state, Plan: plan}, &result)
			if result.RequiresReplace {
				t.Fatal("permission removal must never replace the app")
			}
		}
	}
}

func TestPermissionRefreshAdoptsActualConfigurationWithoutLosingOptionalValues(t *testing.T) {
	ctx := context.Background()
	target := "00000003-0000-0000-c000-000000000000"
	source := []azclient.APIPermissionRequest{{TargetType: "ExternalApi", TargetExternalAPIAppID: &target, GrantType: "AppRole", Permissions: []azclient.APIPermission{{ID: "aaaaaaaa-1111-4222-8333-eeeeeeeeeeee"}}}}
	value := types.SetNull(permissionRequestObjectType())
	var diagnostics diag.Diagnostics
	mapPermissionRequests(ctx, source, &value, &diagnostics)
	var blocks []permissionRequestModel
	diagnostics.Append(value.ElementsAs(ctx, &blocks, false)...)
	var permissions []permissionModel
	diagnostics.Append(blocks[0].Permissions.ElementsAs(ctx, &permissions, false)...)
	permissions[0].RequiresAdminConsent = types.BoolNull()
	blocks[0].Permissions, _ = types.SetValueFrom(ctx, permissionRequestObjectType().AttrTypes["permission"].(types.SetType).ElemType, permissions)
	value, _ = types.SetValueFrom(ctx, permissionRequestObjectType(), blocks)
	before := value
	mapPermissionRequests(ctx, source, &value, &diagnostics)
	if diagnostics.HasError() || !before.Equal(value) {
		t.Fatal("refresh must retain omitted false values")
	}
	mapPermissionRequests(ctx, []azclient.APIPermissionRequest{}, &value, &diagnostics)
	if len(value.Elements()) != 0 {
		t.Fatal("server-side removal must be visible")
	}
}
