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

func TestRequestWaitTracksApprovalAndProvisioningToTerminalStatus(t *testing.T) {
	for _, terminal := range []string{"Ready", "NeedsAttention", "Rejected"} {
		t.Run(terminal, func(t *testing.T) {
			reads := 0
			reason := "Registration settings: missing pre-authorized scope. Attempt: test-attempt."
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method != "GET" || !strings.HasSuffix(req.URL.Path, "/stable") {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
				}
				reads++
				status := "Provisioning"
				if reads == 2 {
					status = terminal
				}
				_ = json.NewEncoder(w).Encode(azclient.Application{ResourceID: "stable", RequestID: 1291, Status: status, StatusReason: &reason})
			}))
			defer server.Close()
			api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
			result, err := waitForApplicationRequest(context.Background(), api, &azclient.Application{ResourceID: "stable", Status: "PendingApproval"}, time.Millisecond, time.Second)
			if reads != 2 || result.Status != terminal || result.ResourceID != "stable" {
				t.Fatalf("unexpected result: %+v, %d reads", result, reads)
			}
			if terminal == "Ready" && err != nil {
				t.Fatal(err)
			}
			if terminal != "Ready" && (err == nil || !strings.Contains(err.Error(), reason)) {
				t.Fatalf("missing failure reason: %v", err)
			}
		})
	}
}

func TestRequestWaitTimeoutAndCancellationRetainIdentity(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			cancel()
		}
		current := &azclient.Application{ResourceID: "stable", RequestID: 1291, Status: "PendingApproval"}
		result, err := waitForApplicationRequest(ctx, nil, current, time.Hour, time.Millisecond)
		cancel()
		if result != current || err == nil || !strings.Contains(err.Error(), "1291") {
			t.Fatalf("lost identity or timeout: %+v %v", result, err)
		}
	}
}

func TestRequestCreateWaitIsOptInAndPreservesFailedRequest(t *testing.T) {
	for _, wait := range []bool{false, true} {
		ctx := context.Background()
		reads := 0
		reason := "Registration settings: missing scope. Attempt: test-attempt."
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/api/terraform/v1/capabilities" {
				_ = json.NewEncoder(w).Encode(azclient.Capabilities{Enabled: true, AllowApplicationCreation: true})
				return
			}
			status := "Provisioning"
			if req.Method == "GET" {
				reads++
				status = "NeedsAttention"
			}
			_ = json.NewEncoder(w).Encode(azclient.Application{ResourceID: "stable", RequestID: 1291, Status: status, StatusReason: &reason, DisplayName: "API"})
		}))
		api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
		s := managedApplicationSchema(false)
		plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
		for name, value := range map[string]any{"display_name": "API", "wait_for_completion": wait, "poll_interval_seconds": int64(1)} {
			if d := plan.SetAttribute(ctx, path.Root(name), value); d.HasError() {
				t.Fatal(d)
			}
		}
		response := resource.CreateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}}
		(&applicationRequestResource{client: api}).Create(ctx, resource.CreateRequest{Plan: plan}, &response)
		server.Close()
		var state applicationRequestResourceModel
		if d := response.State.Get(ctx, &state); d.HasError() {
			t.Fatal(d)
		}
		if response.Diagnostics.HasError() != wait || state.ID.ValueString() != "stable" {
			t.Fatalf("lost state or wrong outcome: %+v %v", state, response.Diagnostics)
		}
		if wait && (reads != 1 || state.Status.ValueString() != "NeedsAttention" || !strings.Contains(response.Diagnostics[0].Detail(), reason)) {
			t.Fatal(response.Diagnostics)
		}
		if !wait && reads != 0 {
			t.Fatal("async default polled")
		}
	}
}

func TestRequestRefreshWarnsWithBackgroundFailureWithoutDroppingState(t *testing.T) {
	ctx := context.Background()
	reason := "Registration settings: missing scope. Attempt: test-attempt."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" {
			t.Errorf("unexpected mutation: %s", req.Method)
		}
		_ = json.NewEncoder(w).Encode(azclient.Application{ResourceID: "stable", RequestID: 1291, Status: "NeedsAttention", StatusReason: &reason})
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
	s := managedApplicationSchema(false)
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if d := state.SetAttribute(ctx, path.Root("id"), "stable"); d.HasError() {
		t.Fatal(d)
	}
	response := resource.ReadResponse{State: state}
	(&applicationRequestResource{client: api}).Read(ctx, resource.ReadRequest{State: state}, &response)
	if response.Diagnostics.HasError() || len(response.Diagnostics) != 1 || !strings.Contains(response.Diagnostics[0].Detail(), reason) || response.State.Raw.IsNull() {
		t.Fatal(response.Diagnostics)
	}
}

func TestRequestWaitRejectsInvalidPollingBeforeSubmission(t *testing.T) {
	for _, pair := range [][2]int64{{0, 60}, {301, 60}, {5, 0}, {5, 1441}} {
		if validateApplicationWaitSettings(pair[0], pair[1]) == nil {
			t.Fatalf("accepted %v", pair)
		}
	}
}

func TestEnablingWaitPlansGeneratedIdentifiersAsUnknown(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = json.NewEncoder(w).Encode(azclient.Capabilities{Enabled: true, AllowApplicationCreation: true, SupportsApplicationDetailsUpdates: true})
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
	s := managedApplicationSchema(false)
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	for name, value := range map[string]any{"id": "stable", "display_name": "API", "status": "Provisioning"} {
		if d := state.SetAttribute(ctx, path.Root(name), value); d.HasError() {
			t.Fatal(d)
		}
	}
	plan := tfsdk.Plan{Schema: s, Raw: state.Raw}
	if d := plan.SetAttribute(ctx, path.Root("wait_for_completion"), true); d.HasError() {
		t.Fatal(d)
	}
	response := resource.ModifyPlanResponse{Plan: plan}
	(&applicationRequestResource{client: api}).ModifyPlan(ctx, resource.ModifyPlanRequest{Plan: plan, State: state, Config: tfsdk.Config{Schema: s, Raw: plan.Raw}}, &response)
	if response.Diagnostics.HasError() {
		t.Fatal(response.Diagnostics)
	}
	for _, name := range []string{"status", "status_reason", "application_entity_id", "application_id", "application_object_id"} {
		var value types.String
		if d := response.Plan.GetAttribute(ctx, path.Root(name), &value); d.HasError() {
			t.Fatal(d)
		}
		if !value.IsUnknown() {
			t.Fatalf("%s incorrectly fixed to old value", name)
		}
	}
	var identifiers types.Set
	if d := response.Plan.GetAttribute(ctx, path.Root("identifier_uris"), &identifiers); d.HasError() {
		t.Fatal(d)
	}
	if !identifiers.IsUnknown() {
		t.Fatal("unconfigured registration must be resolved after approval")
	}
	configured := applicationResourceModel{IdentifierURIs: stringSet(t, "api://explicit")}
	if d := response.Plan.SetAttribute(ctx, path.Root("identifier_uris"), configured.IdentifierURIs); d.HasError() {
		t.Fatal(d)
	}
	planPendingRegistration(ctx, configured, &response)
	if d := response.Plan.GetAttribute(ctx, path.Root("identifier_uris"), &identifiers); d.HasError() {
		t.Fatal(d)
	}
	if !identifiers.Equal(configured.IdentifierURIs) {
		t.Fatal("explicit registration configuration must remain authoritative")
	}
}

func TestRequestWaitReadFailureRetainsLatestRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
	current := &azclient.Application{ResourceID: "stable", RequestID: 1291, Status: "Provisioning"}
	result, err := waitForApplicationRequest(context.Background(), api, current, time.Millisecond, time.Second)
	if result != current || err == nil || !strings.Contains(err.Error(), "1291") {
		t.Fatalf("lost request or error: %+v %v", result, err)
	}
}
