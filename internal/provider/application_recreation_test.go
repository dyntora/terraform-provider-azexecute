package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestDeletedApplicationRefreshAllowsCreationWithNewIdentity(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		t.Run(map[bool]string{false: "request", true: "application"}[synchronous], func(t *testing.T) {
			ctx := context.Background()
			const oldID = "11111111-2222-3333-4444-555555555555"
			var createdID string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodGet && req.URL.Path == "/api/terraform/v1/capabilities" {
					_ = json.NewEncoder(w).Encode(azclient.Capabilities{Enabled: true, AllowApplicationCreation: true})
					return
				}
				if req.Method == http.MethodGet && req.URL.Path == "/api/terraform/v1/applications/"+oldID {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				if req.Method != http.MethodPost || req.URL.Path != "/api/terraform/v1/applications" {
					t.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				var input azclient.ApplicationCreate
				if err := json.NewDecoder(req.Body).Decode(&input); err != nil {
					t.Error(err)
				}
				createdID = input.ResourceID
				if input.DisplayName != "Recreated API" {
					t.Errorf("name changed: %s", input.DisplayName)
				}
				status := "PendingApproval"
				if synchronous {
					status = "Ready"
				}
				_ = json.NewEncoder(w).Encode(azclient.Application{ResourceID: input.ResourceID, RequestID: 42, DisplayName: input.DisplayName, Status: status, Metadata: input.Metadata})
			}))
			defer server.Close()
			api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
			schema := managedApplicationSchema(synchronous)
			state := tfsdk.State{Raw: tftypes.NewValue(schema.Type().TerraformType(ctx), nil), Schema: schema}
			for name, value := range map[string]any{"id": oldID, "display_name": "Recreated API", "status": "Ready", "request_id": int64(12)} {
				if d := state.SetAttribute(ctx, path.Root(name), value); d.HasError() {
					t.Fatal(d)
				}
			}
			read := resource.ReadResponse{State: state}
			if synchronous {
				(&applicationResource{client: api}).Read(ctx, resource.ReadRequest{State: state}, &read)
			} else {
				(&applicationRequestResource{client: api}).Read(ctx, resource.ReadRequest{State: state}, &read)
			}
			if read.Diagnostics.HasError() || !read.State.Raw.IsNull() {
				t.Fatalf("deleted state retained: %v", read.Diagnostics)
			}
			plan := tfsdk.Plan{Raw: tftypes.NewValue(schema.Type().TerraformType(ctx), nil), Schema: schema}
			for name, value := range map[string]any{"display_name": "Recreated API", "business_justification": "Restore manually deleted application", "configure_registration": false} {
				if d := plan.SetAttribute(ctx, path.Root(name), value); d.HasError() {
					t.Fatal(d)
				}
			}
			create := resource.CreateResponse{State: tfsdk.State{Raw: tftypes.NewValue(schema.Type().TerraformType(ctx), nil), Schema: schema}}
			if synchronous {
				(&applicationResource{client: api}).Create(ctx, resource.CreateRequest{Plan: plan}, &create)
			} else {
				(&applicationRequestResource{client: api}).Create(ctx, resource.CreateRequest{Plan: plan}, &create)
			}
			if create.Diagnostics.HasError() {
				t.Fatal(create.Diagnostics)
			}
			var newID string
			var newRequestID int64
			if d := create.State.GetAttribute(ctx, path.Root("id"), &newID); d.HasError() {
				t.Fatal(d)
			}
			if d := create.State.GetAttribute(ctx, path.Root("request_id"), &newRequestID); d.HasError() {
				t.Fatal(d)
			}
			if newID == "" || newID == oldID || newID != createdID || newRequestID != 42 {
				t.Fatalf("reused deleted identity: id=%s request=%d", newID, newRequestID)
			}
		})
	}
}

func TestFailedApplicationRefreshDoesNotRemoveState(t *testing.T) {
	for _, synchronous := range []bool{false, true} {
		for _, status := range []int{http.StatusForbidden, http.StatusServiceUnavailable} {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
			}))
			api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
			ctx := context.Background()
			schema := managedApplicationSchema(synchronous)
			state := tfsdk.State{Raw: tftypes.NewValue(schema.Type().TerraformType(ctx), nil), Schema: schema}
			if d := state.SetAttribute(ctx, path.Root("id"), "existing-resource"); d.HasError() {
				t.Fatal(d)
			}
			response := resource.ReadResponse{State: state}
			if synchronous {
				(&applicationResource{client: api}).Read(ctx, resource.ReadRequest{State: state}, &response)
			} else {
				(&applicationRequestResource{client: api}).Read(ctx, resource.ReadRequest{State: state}, &response)
			}
			server.Close()
			if !response.Diagnostics.HasError() || response.State.Raw.IsNull() {
				t.Fatalf("status %d removed state or hid error", status)
			}
		}
	}
}
