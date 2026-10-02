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
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func listenerTestModel() eventListenerModel {
	return eventListenerModel{
		ID: types.StringUnknown(), Name: types.StringValue("Deploy renewed secret"), Description: types.StringNull(),
		EventType: types.StringValue("ApplicationCredentialRenewed"), ActionType: types.StringUnknown(),
		IsEnabled: types.BoolValue(true), ExecutionOrder: types.Int64Value(100),
		ApplicationEntityID: types.StringValue("11111111-2222-4333-8444-555555555555"), BroadListener: types.BoolValue(false),
		CredentialType: types.StringValue("Secret"), AutomationTaskID: types.Int64Value(42),
		Parameters:      types.MapValueMust(types.StringType, map[string]attr.Value{"DeploymentInput": types.StringValue("{{ renewed_secret }}")}),
		TopdeskSettings: types.ObjectNull(topdeskTypes()), AuthorizedByUserID: types.StringUnknown(),
	}
}

func TestEventListenerLifecycleAndImport(t *testing.T) {
	ctx := context.Background()
	var saved azclient.EventListener
	calls := []string{}
	missing := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls = append(calls, req.Method+" "+req.URL.Path)
		if req.Header.Get("Authorization") != "Bearer token" {
			t.Error("missing bearer token")
		}
		if missing {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "POST", "PUT":
			if err := json.NewDecoder(req.Body).Decode(&saved); err != nil {
				t.Error(err)
			}
			if saved.AuthorizedByUserID != nil {
				t.Error("client supplied authorizer")
			}
			saved.ID = 7
		case "DELETE":
			w.WriteHeader(204)
			return
		}
		_ = json.NewEncoder(w).Encode(saved)
	}))
	defer server.Close()
	api, err := azclient.New(server.URL, "ignored", "token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	r := &eventListenerResource{client: api}
	sr := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	if d := sr.Schema.ValidateImplementation(ctx); d.HasError() {
		t.Fatal(d)
	}
	emptyState := func() tfsdk.State {
		return tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
	}
	planFor := func(m eventListenerModel) tfsdk.Plan {
		plan := tfsdk.Plan{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
		if d := plan.Set(ctx, m); d.HasError() {
			t.Fatal(d)
		}
		return plan
	}
	m := listenerTestModel()
	created := resource.CreateResponse{State: emptyState()}
	r.Create(ctx, resource.CreateRequest{Plan: planFor(m)}, &created)
	if created.Diagnostics.HasError() {
		t.Fatal(created.Diagnostics)
	}
	if saved.AutomationTaskSettings.Parameters[0].Value != "{{ renewed_secret }}" || saved.ActionType != "AutomationTask" {
		t.Fatal("lost protected mapping")
	}
	if d := created.State.Get(ctx, &m); d.HasError() {
		t.Fatal(d)
	}
	if m.ID.ValueString() != "7" {
		t.Fatal("missing ID")
	}
	m.IsEnabled = types.BoolValue(false)
	updated := resource.UpdateResponse{State: created.State}
	r.Update(ctx, resource.UpdateRequest{Plan: planFor(m), State: created.State}, &updated)
	if updated.Diagnostics.HasError() || saved.IsEnabled {
		t.Fatal("update failed", updated.Diagnostics)
	}
	imported := resource.ImportStateResponse{State: emptyState()}
	r.ImportState(ctx, resource.ImportStateRequest{ID: "7"}, &imported)
	if imported.Diagnostics.HasError() {
		t.Fatal(imported.Diagnostics)
	}
	read := resource.ReadResponse{State: imported.State}
	r.Read(ctx, resource.ReadRequest{State: imported.State}, &read)
	if read.Diagnostics.HasError() {
		t.Fatal(read.Diagnostics)
	}
	var adopted eventListenerModel
	if d := read.State.Get(ctx, &adopted); d.HasError() {
		t.Fatal(d)
	}
	if !adopted.Parameters.Equal(m.Parameters) || adopted.IsEnabled.ValueBool() {
		t.Fatal("import did not adopt configuration")
	}
	deleted := resource.DeleteResponse{State: read.State}
	r.Delete(ctx, resource.DeleteRequest{State: read.State}, &deleted)
	if deleted.Diagnostics.HasError() {
		t.Fatal(deleted.Diagnostics)
	}
	missing = true
	gone := resource.ReadResponse{State: read.State}
	r.Read(ctx, resource.ReadRequest{State: read.State}, &gone)
	if gone.Diagnostics.HasError() || !gone.State.Raw.IsNull() {
		t.Fatal("404 did not remove state", gone.Diagnostics)
	}
	if strings.Join(calls, ",") != "POST /api/v1/EventListeners,PUT /api/v1/EventListeners/7,GET /api/v1/EventListeners/7,DELETE /api/v1/EventListeners/7,GET /api/v1/EventListeners/7" {
		t.Fatal(calls)
	}
}

func TestEventListenerValidationAndTopdeskRoundTrip(t *testing.T) {
	ctx := context.Background()
	m := listenerTestModel()
	if d := validateEventListener(m); d.HasError() {
		t.Fatal(d)
	}
	m.ApplicationEntityID = types.StringNull()
	if d := validateEventListener(m); !d.HasError() {
		t.Fatal("silent broad scope was allowed")
	}
	m.BroadListener = types.BoolValue(true)
	m.AutomationTaskID = types.Int64Null()
	m.Parameters = types.MapValueMust(types.StringType, map[string]attr.Value{})
	attrs := map[string]attr.Value{}
	for key := range topdeskFields {
		attrs[key] = types.StringNull()
	}
	attrs["integration_id"] = types.Int64Value(4)
	attrs["request_text"] = types.StringValue("Renewed {{ application_name }}")
	m.TopdeskSettings = types.ObjectValueMust(topdeskTypes(), attrs)
	input, d := eventListenerInput(ctx, m)
	if d.HasError() {
		t.Fatal(d)
	}
	if input.ActionType != "TopdeskCreateIncident" || input.AutomationTaskSettings != nil || input.TopdeskSettings["topdeskIntegrationId"] != int64(4) {
		t.Fatal(input)
	}
	// Use wire JSON so number decoding matches real API responses.
	raw, _ := json.Marshal(input)
	var decoded azclient.EventListener
	_ = json.Unmarshal(raw, &decoded)
	if d := setEventListenerModel(ctx, &m, &decoded); d.HasError() {
		t.Fatal(d)
	}
	if m.TopdeskSettings.Attributes()["request_text"].(types.String).ValueString() != "Renewed {{ application_name }}" {
		t.Fatal("TOPdesk settings lost")
	}
	for _, id := range []string{"0", "-1", "abc", "2147483648"} {
		if _, err := eventListenerID(id); err == nil {
			t.Fatal("invalid ID accepted", id)
		}
	}
}

func TestEventListenerRejectsCredentialEnvelopes(t *testing.T) {
	m := listenerTestModel()
	m.Parameters = types.MapValueMust(types.StringType, map[string]attr.Value{"DeploymentInput": types.StringValue("azx-credential:v1:payload")})
	if d := validateEventListener(m); !d.HasError() {
		t.Fatal("encrypted credential envelope allowed")
	}
}

func TestEventListenerForbiddenReadKeepsExistingState(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(403) }))
	defer server.Close()
	api, err := azclient.New(server.URL, "ignored", "token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	r := &eventListenerResource{client: api}
	sr := resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	state := tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
	m := listenerTestModel()
	m.ID = types.StringValue("7")
	m.ActionType = types.StringValue("AutomationTask")
	m.AuthorizedByUserID = types.StringNull()
	if d := state.Set(ctx, m); d.HasError() {
		t.Fatal(d)
	}
	result := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &result)
	if !result.Diagnostics.HasError() || !result.State.Raw.Equal(state.Raw) {
		t.Fatal("permission error must preserve existing state", result.Diagnostics)
	}
}
