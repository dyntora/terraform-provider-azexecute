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
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestUnreadyUpdatesExplainRecoveryWithoutMutatingOrRemovingState(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		for _, status := range []string{"Rejected", "NeedsAttention", "PendingApproval", "Provisioning"} {
			ctx := context.Background()
			reason := "Recorded reason for request"
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				calls++
				if req.Method != http.MethodGet {
					t.Errorf("unexpected mutation: %s", req.Method)
				}
				_ = json.NewEncoder(w).Encode(azclient.Application{ResourceID: "existing", RequestID: 42, Status: status, StatusReason: &reason})
			}))
			api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
			s := managedApplicationSchema(synchronous)
			state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
			if d := state.SetAttribute(ctx, path.Root("id"), "existing"); d.HasError() {
				t.Fatal(d)
			}
			plan := tfsdk.Plan{Schema: s, Raw: state.Raw}
			response := resource.UpdateResponse{State: state}
			if synchronous {
				(&applicationResource{client: api}).Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &response)
			} else {
				(&applicationRequestResource{client: api}).Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &response)
			}
			server.Close()
			if !response.Diagnostics.HasError() || response.State.Raw.IsNull() || calls != 1 {
				t.Fatalf("status %s mishandled: %v", status, response.Diagnostics)
			}
			detail := response.Diagnostics[0].Detail()
			if !strings.Contains(detail, reason) || !strings.Contains(detail, "42") {
				t.Fatal(detail)
			}
			if (status == "NeedsAttention" || status == "Rejected") && !strings.Contains(detail, "Clean up and retire") {
				t.Fatal(detail)
			}
		}
	}
}

func TestSynchronousCreateStopsForRecoveryAndPreservesIdentity(t *testing.T) {
	ctx := context.Background()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls++
		if req.URL.Path == "/api/terraform/v1/capabilities" {
			_ = json.NewEncoder(w).Encode(azclient.Capabilities{Enabled: true, AllowApplicationCreation: true})
			return
		}
		if req.Method != http.MethodPost {
			t.Errorf("polled a paused request: %s", req.Method)
		}
		var input azclient.ApplicationCreate
		if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		_ = json.NewEncoder(w).Encode(azclient.Application{ResourceID: input.ResourceID, RequestID: 42, Status: "NeedsAttention", DisplayName: input.DisplayName})
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
	s := managedApplicationSchema(true)
	plan := tfsdk.Plan{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if d := plan.SetAttribute(ctx, path.Root("display_name"), "Failed API"); d.HasError() {
		t.Fatal(d)
	}
	response := resource.CreateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}}
	(&applicationResource{client: api}).Create(ctx, resource.CreateRequest{Plan: plan}, &response)
	if !response.Diagnostics.HasError() || response.State.Raw.IsNull() || calls != 2 {
		t.Fatalf("lost request or kept polling: %v", response.Diagnostics)
	}
	if !strings.Contains(response.Diagnostics[0].Detail(), "Clean up and retire") {
		t.Fatal(response.Diagnostics)
	}
}
