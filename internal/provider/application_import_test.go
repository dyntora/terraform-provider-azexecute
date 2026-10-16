package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestImportRefusesUnknownPermissionHistory(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	for _, body := range []string{
		`{"resourceId":"` + id + `","status":"Ready"}`,
		`{"resourceId":"` + id + `","status":"Ready","requestedApiPermissions":[]}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Error("import mutated the application")
			}
			_, _ = w.Write([]byte(body))
		}))
		api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
		ctx := context.Background()
		for _, synchronous := range []bool{false, true} {
			s := managedApplicationSchema(synchronous)
			response := resource.ImportStateResponse{State: tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}}
			request := resource.ImportStateRequest{ID: id}
			if synchronous {
				(&applicationResource{client: api}).ImportState(ctx, request, &response)
			} else {
				(&applicationRequestResource{client: api}).ImportState(ctx, request, &response)
			}
			if response.Diagnostics.HasError() == strings.Contains(body, "requestedApiPermissions") {
				t.Fatal(response.Diagnostics)
			}
		}
		server.Close()
	}
}
