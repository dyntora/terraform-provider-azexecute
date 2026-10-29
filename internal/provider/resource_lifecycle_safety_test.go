package provider

import (
	"context"
	"encoding/json"
	"fmt"
	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLifecycleBare404MustRetainState(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, "proxy route not found")
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "scope", "test-token", nil, time.Second)
	for name, r := range map[string]resource.Resource{"group": &azgroupResource{client: api}, "membership": &azgroupMembershipResource{client: api}, "listener": &eventListenerResource{client: api}} {
		t.Run(name, func(t *testing.T) {
			var sr resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &sr)
			state := tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
			state.SetAttribute(ctx, path.Root("id"), "42")
			if name == "membership" {
				state.SetAttribute(ctx, path.Root("group_id"), int64(7))
				state.SetAttribute(ctx, path.Root("type"), "integration")
				state.SetAttribute(ctx, path.Root("target_id"), "9")
			}
			out := resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, &out)
			if out.State.Raw.IsNull() || !out.Diagnostics.HasError() {
				t.Fatalf("bare proxy 404 discarded existing state without error: removed=%t diagnostics=%v", out.State.Raw.IsNull(), out.Diagnostics)
			}
		})
	}
}

func TestLifecycleUpdateChecksStoredIdentityBeforeAndAfterWrite(t *testing.T) {
	ctx := context.Background()
	const id = "11111111-2222-4333-8444-555555555555"
	for _, kind := range []string{"group", "aws", "vercel", "github"} {
		for _, phase := range []string{"preflight", "response"} {
			t.Run(kind+"/"+phase, func(t *testing.T) {
				writes := 0
				var r resource.Resource = &azgroupResource{}
				if kind != "group" {
					r = &integrationResource{kind: kind}
				}
				var sr resource.SchemaResponse
				r.Schema(ctx, resource.SchemaRequest{}, &sr)
				state := tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
				if kind != "group" {
					if d := state.Set(ctx, integrationTestObject(kind)); d.HasError() {
						t.Fatal(d)
					}
				}
				for name, value := range map[string]any{"id": id, "name": "Existing"} {
					if d := state.SetAttribute(ctx, path.Root(name), value); d.HasError() {
						t.Fatal(d)
					}
				}
				attribute := "integration_id"
				if kind == "group" {
					attribute = "group_id"
					state.SetAttribute(ctx, path.Root("source"), "local")
					state.SetAttribute(ctx, path.Root("active"), true)
				}
				state.SetAttribute(ctx, path.Root(attribute), int64(7))
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					numeric := 7
					if req.Method == "PUT" {
						writes++
					}
					if phase == "preflight" || req.Method == "PUT" {
						numeric = 99
					}
					if kind == "group" {
						_ = json.NewEncoder(w).Encode(map[string]any{"resourceId": id, "groupId": numeric, "actualSource": "Local", "status": "Ready", "configuration": map[string]any{"name": "Existing", "source": "Local", "isActive": true}})
					} else {
						_ = json.NewEncoder(w).Encode(map[string]any{"resourceId": id, "integrationId": numeric, "kind": kind, "configuration": integrationInput(kind, integrationTestObject(kind).Attributes())})
					}
				}))
				defer server.Close()
				api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
				if kind == "group" {
					r.(*azgroupResource).client = api
				} else {
					r.(*integrationResource).client = api
				}
				plan := tfsdk.Plan{Schema: sr.Schema, Raw: state.Raw}
				plan.SetAttribute(ctx, path.Root(attribute), types.Int64Unknown())
				result := resource.UpdateResponse{State: state}
				r.Update(ctx, resource.UpdateRequest{Plan: plan, State: state}, &result)
				expectedWrites := 0
				if phase == "response" {
					expectedWrites = 1
				}
				if !result.Diagnostics.HasError() || !result.State.Raw.Equal(state.Raw) || writes != expectedWrites {
					t.Fatalf("identity not protected: writes=%d diagnostics=%v", writes, result.Diagnostics)
				}
			})
		}
	}
}

func TestLifecycleUnconfirmedDeleteMustFail(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(202)
		fmt.Fprint(w, `{"status":"pending"}`)
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "scope", "test-token", nil, time.Second)
	for name, del := range map[string]func() error{"group": func() error { return api.DeleteGroup(ctx, "existing") }, "membership": func() error { return api.DeleteGroupMembership(ctx, 7, "integrations", "9") }, "listener": func() error { return api.DeleteEventListener(ctx, 42) }} {
		t.Run(name, func(t *testing.T) {
			if err := del(); err == nil {
				t.Fatal("HTTP 202 pending was accepted as completed deletion")
			}
		})
	}
}

func TestLifecycleListenerReadMustRejectDifferentIdentity(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":99,"name":"another listener","eventType":"ApplicationCreated","actionType":"AutomationTask"}`)
	}))
	defer server.Close()
	api, _ := azclient.New(server.URL, "scope", "test-token", nil, time.Second)
	r := &eventListenerResource{client: api}
	var sr resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &sr)
	state := tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
	state.SetAttribute(ctx, path.Root("id"), "42")
	out := resource.ReadResponse{State: state}
	r.Read(ctx, resource.ReadRequest{State: state}, &out)
	var actual string
	out.State.GetAttribute(ctx, path.Root("id"), &actual)
	if !out.Diagnostics.HasError() || actual != "42" {
		t.Fatalf("read of listener 42 adopted returned ID %s, diagnostics=%v", actual, out.Diagnostics)
	}
}

func TestLifecycleStableUUIDMustNotRebindNumericIdentity(t *testing.T) {
	ctx := context.Background()
	const id = "11111111-2222-4333-8444-555555555555"
	for _, kind := range []string{"group", "integration"} {
		t.Run(kind, func(t *testing.T) {
			body := `{"resourceId":"` + id + `","groupId":99,"displayName":"Existing","actualSource":"Local","status":"Ready","configuration":{"name":"Existing","source":"Local","isActive":true}}`
			if kind == "integration" {
				body = `{"resourceId":"` + id + `","integrationId":99,"kind":"aws","configuration":{"name":"Existing","enabled":true,"global":false,"accountId":"123456789012","service":"route53","roleArn":"arn:aws:iam::123456789012:role/test"}}`
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			api, _ := azclient.New(server.URL, "scope", "test-token", nil, time.Second)
			var r resource.Resource = &azgroupResource{client: api}
			attribute := "group_id"
			if kind == "integration" {
				r = &integrationResource{client: api, kind: "aws"}
				attribute = "integration_id"
			}
			var sr resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &sr)
			state := tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
			state.SetAttribute(ctx, path.Root("id"), id)
			state.SetAttribute(ctx, path.Root(attribute), int64(7))
			out := resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, &out)
			var actual int64
			out.State.GetAttribute(ctx, path.Root(attribute), &actual)
			if !out.Diagnostics.HasError() || actual != 7 {
				t.Fatalf("stable UUID rebound %s from 7 to %d without error: %v", attribute, actual, out.Diagnostics)
			}
		})
	}
}
