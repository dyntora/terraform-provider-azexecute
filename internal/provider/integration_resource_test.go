package provider

import (
	"context"
	"encoding/json"
	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func integrationTestObject(kind string) types.Object {
	m := emptyIntegrationObject(context.Background(), kind)
	a := m.Attributes()
	a["id"] = types.StringUnknown()
	a["integration_id"] = types.Int64Unknown()
	a["name"] = types.StringValue("Production")
	a["enabled"] = types.BoolValue(true)
	a["global"] = types.BoolValue(false)
	for k, v := range map[string]map[string]string{
		"aws":    {"service": "secrets_manager", "account_id": "123456789012", "role_arn": "arn:aws:iam::123456789012:role/azexecute", "external_id": "original-secret"},
		"vercel": {"team_id": "team_123", "access_token": "original-secret"},
		"github": {"app_id": "123", "installation_id": "456", "private_key_pem": "original-secret", "webhook_secret": "webhook"},
	}[kind] {
		a[k] = types.StringValue(v)
	}
	return types.ObjectValueMust(m.AttributeTypes(context.Background()), a)
}

func TestIntegrationLifecycleAndImport(t *testing.T) {
	for _, kind := range []string{"aws", "vercel", "github"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			var stored map[string]any
			var id string
			status := 200
			deleted := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if status != 200 {
					w.WriteHeader(status)
					_, _ = w.Write([]byte(`{"code":"access_denied"}`))
					return
				}
				if req.Method == "DELETE" {
					deleted = true
					w.WriteHeader(204)
					return
				}
				if deleted {
					w.WriteHeader(404)
					_, _ = w.Write([]byte(`{"code":"terraform_integration_not_found"}`))
					return
				}
				if req.Method == "POST" && !strings.Contains(req.URL.Path, "/import/") {
					id = req.URL.Path[strings.LastIndex(req.URL.Path, "/")+1:]
				}
				if req.Method == "POST" && !strings.Contains(req.URL.Path, "/import/") || req.Method == "PUT" {
					var input map[string]any
					_ = json.NewDecoder(req.Body).Decode(&input)
					stored = input
				}
				safe := map[string]any{}
				for k, v := range stored {
					safe[k] = v
				}
				for _, key := range []string{"externalId", "accessToken", "privateKeyPem", "webhookSecret"} {
					delete(safe, key)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"resourceId": id, "integrationId": 42, "kind": kind, "configuration": safe})
			}))
			defer server.Close()
			api, _ := azclient.New(server.URL, "ignored", "token", nil, time.Second)
			r := &integrationResource{kind: kind, client: api}
			sr := resource.SchemaResponse{}
			r.Schema(ctx, resource.SchemaRequest{}, &sr)
			if d := sr.Schema.ValidateImplementation(ctx); d.HasError() {
				t.Fatal(d)
			}
			state := tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
			m := integrationTestObject(kind)
			if d := state.Set(ctx, m); d.HasError() {
				t.Fatal(d)
			}
			plan := tfsdk.Plan{Schema: sr.Schema, Raw: state.Raw}
			create := resource.CreateResponse{State: state}
			r.Create(ctx, resource.CreateRequest{Plan: plan}, &create)
			if create.Diagnostics.HasError() {
				t.Fatal(create.Diagnostics)
			}
			state = create.State
			read := resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, &read)
			if read.Diagnostics.HasError() {
				t.Fatal(read.Diagnostics)
			}
			var actual types.Object
			if d := read.State.Get(ctx, &actual); d.HasError() {
				t.Fatal(d)
			}
			secret := map[string]string{"aws": "external_id", "vercel": "access_token", "github": "private_key_pem"}[kind]
			if integrationString(actual.Attributes(), secret).ValueString() != "original-secret" {
				t.Fatal("read erased credential")
			}
			a := actual.Attributes()
			a["name"] = types.StringValue("Renamed")
			a[secret] = types.StringValue("rotated-secret")
			actual = types.ObjectValueMust(actual.AttributeTypes(ctx), a)
			if d := state.Set(ctx, actual); d.HasError() {
				t.Fatal(d)
			}
			update := resource.UpdateResponse{State: read.State}
			r.Update(ctx, resource.UpdateRequest{Plan: tfsdk.Plan{Schema: sr.Schema, Raw: state.Raw}, State: read.State}, &update)
			if update.Diagnostics.HasError() {
				t.Fatal(update.Diagnostics)
			}
			state = update.State
			imported := resource.ImportStateResponse{State: tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}}
			r.ImportState(ctx, resource.ImportStateRequest{ID: "42"}, &imported)
			if imported.Diagnostics.HasError() {
				t.Fatal(imported.Diagnostics)
			}
			if d := imported.State.Get(ctx, &actual); d.HasError() {
				t.Fatal(d)
			}
			if !integrationString(actual.Attributes(), secret).IsNull() {
				t.Fatal("import exposed credential")
			}
			if integrationString(actual.Attributes(), "name").ValueString() != "Renamed" {
				t.Fatal("import lost metadata")
			}
			for _, code := range []int{403, 404, 502} {
				status = code
				read = resource.ReadResponse{State: state}
				r.Read(ctx, resource.ReadRequest{State: state}, &read)
				if !read.Diagnostics.HasError() || read.State.Raw.IsNull() {
					t.Fatal("ambiguous absence removed state")
				}
			}
			status = 200
			destroy := resource.DeleteResponse{State: state}
			r.Delete(ctx, resource.DeleteRequest{State: state}, &destroy)
			if destroy.Diagnostics.HasError() {
				t.Fatal(destroy.Diagnostics)
			}
			read = resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, &read)
			if read.Diagnostics.HasError() || !read.State.Raw.IsNull() {
				t.Fatal("confirmed absence retained state")
			}
		})
	}
}

func TestIntegrationDataSourcesNeverAdoptOrExposeCredentials(t *testing.T) {
	for _, kind := range []string{"aws", "vercel", "github"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || !strings.HasSuffix(r.URL.Path, "/lookup/42") {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"resourceId": nil, "integrationId": 42, "kind": kind, "configuration": integrationInput(kind, integrationTestObject(kind).Attributes())})
			}))
			defer server.Close()
			api, _ := azclient.New(server.URL, "ignored", "token", nil, time.Second)
			d := &integrationDataSource{kind: kind, client: api}
			sr := datasource.SchemaResponse{}
			d.Schema(ctx, datasource.SchemaRequest{}, &sr)
			for _, f := range integrationFields(kind) {
				if f.sensitive {
					if _, ok := sr.Schema.Attributes[f.name]; ok {
						t.Fatal("credential exposed by data source schema")
					}
				}
			}
			a := map[string]attr.Value{}
			for k := range sr.Schema.Attributes {
				switch k {
				case "integration_id":
					a[k] = types.Int64Value(42)
				case "enabled", "global":
					a[k] = types.BoolNull()
				default:
					a[k] = types.StringNull()
				}
			}
			state := tfsdk.State{Schema: sr.Schema, Raw: tftypes.NewValue(sr.Schema.Type().TerraformType(ctx), nil)}
			if diags := state.Set(ctx, types.ObjectValueMust(sr.Schema.Type().(types.ObjectType).AttrTypes, a)); diags.HasError() {
				t.Fatal(diags)
			}
			read := datasource.ReadResponse{State: state}
			d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: sr.Schema, Raw: state.Raw}}, &read)
			if read.Diagnostics.HasError() {
				t.Fatal(read.Diagnostics)
			}
			var m types.Object
			read.State.Get(ctx, &m)
			if !integrationString(m.Attributes(), "id").IsNull() {
				t.Fatal("lookup adopted existing integration")
			}
		})
	}
}

func TestIntegrationValidationDefersUnknownsAndChecksCredentials(t *testing.T) {
	for _, kind := range []string{"aws", "vercel", "github"} {
		m := integrationTestObject(kind)
		a := m.Attributes()
		if d := validateIntegration(kind, a, true); d.HasError() {
			t.Fatal(d)
		}
		for _, f := range integrationFields(kind) {
			a[f.name] = types.StringUnknown()
		}
		if d := validateIntegration(kind, a, true); d.HasError() {
			t.Fatal(d)
		}
	}
	for kind, secret := range map[string]string{"vercel": "access_token", "github": "private_key_pem"} {
		a := integrationTestObject(kind).Attributes()
		a[secret] = types.StringNull()
		if !validateIntegration(kind, a, true).HasError() {
			t.Fatal("missing create credential accepted")
		}
		if validateIntegration(kind, a, false).HasError() {
			t.Fatal("import requires credential")
		}
	}
	a := integrationTestObject("aws").Attributes()
	a["role_arn"] = types.StringValue("arn:aws:iam::999999999999:role/wrong")
	if !validateIntegration("aws", a, true).HasError() {
		t.Fatal("mismatched account accepted")
	}
}
