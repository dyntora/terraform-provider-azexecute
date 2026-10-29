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

func TestApplicationIdentityMismatchPreservesStateAndBlocksUpdates(t *testing.T) {
	ctx := context.Background()
	clientID, objectID, entityID := "aaaaaaaa-client", "bbbbbbbb-object", "cccccccc-entity"
	for _, synchronous := range []bool{false, true} {
		for _, field := range []string{"client", "object", "entity", "request"} {
			for _, missing := range []bool{false, true} {
				current := azclient.Application{ResourceID: "existing", RequestID: 42, Status: "Ready", ApplicationID: &clientID, ApplicationObjectID: &objectID, ApplicationEntityID: &entityID}
				changed := "changed"
				pointer := &changed
				if missing {
					pointer = nil
				}
				switch field {
				case "client":
					current.ApplicationID = pointer
				case "object":
					current.ApplicationObjectID = pointer
				case "entity":
					current.ApplicationEntityID = pointer
				case "request":
					current.RequestID = 43
					if missing {
						current.RequestID = 0
					}
				}
				mutations := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodGet {
						mutations++
					}
					_ = json.NewEncoder(w).Encode(current)
				}))
				api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
				s := managedApplicationSchema(synchronous)
				state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
				for name, value := range map[string]any{"id": "existing", "request_id": int64(42), "application_id": clientID, "application_object_id": objectID, "application_entity_id": entityID} {
					if d := state.SetAttribute(ctx, path.Root(name), value); d.HasError() {
						t.Fatal(d)
					}
				}
				read := resource.ReadResponse{State: state}
				update := resource.UpdateResponse{State: state}
				request := resource.UpdateRequest{State: state, Plan: tfsdk.Plan{Schema: s, Raw: state.Raw}}
				if synchronous {
					r := &applicationResource{client: api}
					r.Read(ctx, resource.ReadRequest{State: state}, &read)
					r.Update(ctx, request, &update)
				} else {
					r := &applicationRequestResource{client: api}
					r.Read(ctx, resource.ReadRequest{State: state}, &read)
					r.Update(ctx, request, &update)
				}
				server.Close()
				if !read.Diagnostics.HasError() || !update.Diagnostics.HasError() || !read.State.Raw.Equal(state.Raw) || !update.State.Raw.Equal(state.Raw) || mutations != 0 {
					t.Fatalf("identity %s (missing=%t, sync=%t) not protected: read=%v update=%v mutations=%d", field, missing, synchronous, read.Diagnostics, update.Diagnostics, mutations)
				}
			}
		}
	}
}

func TestIdentityCanBeAssignedOnceAndIsCaseInsensitive(t *testing.T) {
	clientID := "ABCDEF"
	current := &azclient.Application{ResourceID: "existing", RequestID: 42, ApplicationID: &clientID}
	previous := applicationResourceModel{ID: types.StringValue("existing"), RequestID: types.Int64Value(42)}
	if err := validateApplicationIdentity(previous, current); err != nil {
		t.Fatal(err)
	}
	previous.ApplicationID = types.StringValue(strings.ToLower(clientID))
	if err := validateApplicationIdentity(previous, current); err != nil {
		t.Fatal(err)
	}
	if err := validateApplicationIdentity(previous, nil); err == nil {
		t.Fatal("empty response accepted")
	}
	if actual := identityValue(previous.ApplicationID, &clientID); !actual.Equal(previous.ApplicationID) {
		t.Fatal("equivalent identity changed state spelling")
	}
}

func TestUpdateRejectsChangedIdentityOrUnfinishedPermissionsInSuccessfulResponse(t *testing.T) {
	ctx := context.Background()
	for _, synchronous := range []bool{false, true} {
		for _, unfinished := range []bool{false, true} {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				clientID := "original"
				current := azclient.Application{ResourceID: "existing", RequestID: 42, Status: "Ready", DisplayName: "Stable", ApplicationID: &clientID}
				if r.Method == http.MethodPut {
					writes++
					if unfinished {
						current.PermissionRequestsPending = true
					} else {
						clientID = "changed"
					}
				}
				_ = json.NewEncoder(w).Encode(current)
			}))
			api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
			s := managedApplicationSchema(synchronous)
			state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
			for name, value := range map[string]any{"id": "existing", "request_id": int64(42), "application_id": "original", "display_name": "Stable", "configure_registration": false} {
				if d := state.SetAttribute(ctx, path.Root(name), value); d.HasError() {
					t.Fatal(d)
				}
			}
			plan := tfsdk.Plan{Schema: s, Raw: state.Raw}
			if d := plan.SetAttribute(ctx, path.Root("application_id"), types.StringUnknown()); d.HasError() {
				t.Fatal(d)
			}
			response := resource.UpdateResponse{State: state}
			request := resource.UpdateRequest{State: state, Plan: plan}
			if synchronous {
				(&applicationResource{client: api}).Update(ctx, request, &response)
			} else {
				(&applicationRequestResource{client: api}).Update(ctx, request, &response)
			}
			server.Close()
			if writes != 1 || !response.Diagnostics.HasError() || !response.State.Raw.Equal(state.Raw) {
				t.Fatalf("unsafe update response accepted (sync=%t, unfinished=%t): writes=%d diagnostics=%v", synchronous, unfinished, writes, response.Diagnostics)
			}
		}
	}
}

func TestRequestPollingRejectsChangedIdentityAndKeepsLatestGoodResponse(t *testing.T) {
	clientID := "original"
	initial := &azclient.Application{ResourceID: "existing", RequestID: 42, Status: "Provisioning", ApplicationID: &clientID}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		changed := "changed"
		_ = json.NewEncoder(w).Encode(azclient.Application{ResourceID: "existing", RequestID: 42, Status: "Ready", ApplicationID: &changed})
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
	result, err := waitForApplicationRequest(context.Background(), api, initial, time.Millisecond, time.Second)
	if err == nil || result != initial {
		t.Fatalf("polling replaced identity: %+v %v", result, err)
	}
}
