package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestCapabilitiesExposeMetadataPreservationAndLegacyDefault(t *testing.T) {
	for _, body := range []string{`{"preservesHiddenMetadata":true}`, `{}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
		ctx := context.Background()
		d := &capabilitiesDataSource{client: api}
		schema := datasource.SchemaResponse{}
		d.Schema(ctx, datasource.SchemaRequest{}, &schema)
		response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
		d.Read(ctx, datasource.ReadRequest{}, &response)
		server.Close()
		if response.Diagnostics.HasError() {
			t.Fatal(response.Diagnostics)
		}
		var preserved types.Bool
		if diagnostics := response.State.GetAttribute(ctx, path.Root("preserves_hidden_metadata"), &preserved); diagnostics.HasError() {
			t.Fatal(diagnostics)
		}
		if preserved.IsNull() || preserved.IsUnknown() || preserved.ValueBool() != (body != `{}`) {
			t.Fatalf("unexpected capability: %s", preserved)
		}
	}
}

func TestCapabilitiesExposeIndependentDelegatedPolicies(t *testing.T) {
	for _, body := range []string{`{"autoApproveDelegatedPermissions":true,"grantAdminConsentForDelegatedPermissions":false}`, `{}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
		ctx := context.Background()
		d := &capabilitiesDataSource{client: api}
		schema := datasource.SchemaResponse{}
		d.Schema(ctx, datasource.SchemaRequest{}, &schema)
		response := datasource.ReadResponse{State: tfsdk.State{Schema: schema.Schema}}
		d.Read(ctx, datasource.ReadRequest{}, &response)
		server.Close()
		if response.Diagnostics.HasError() {
			t.Fatal(response.Diagnostics)
		}
		for name, expected := range map[string]bool{"auto_approve_delegated_permissions": true, "grant_admin_consent_for_delegated_permissions": false} {
			var value types.Bool
			if diagnostics := response.State.GetAttribute(ctx, path.Root(name), &value); diagnostics.HasError() {
				t.Fatal(diagnostics)
			}
			if body == `{}` {
				if !value.IsNull() {
					t.Fatalf("legacy %s should be unknown to this API, got %s", name, value)
				}
			} else if value.IsNull() || value.IsUnknown() || value.ValueBool() != expected {
				t.Fatalf("unexpected %s: %s", name, value)
			}
		}
	}
}
