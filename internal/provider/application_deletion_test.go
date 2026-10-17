package provider

import (
	"context"
	"fmt"
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

func TestApplicationDeletionCancelledWhilePendingRetainsState(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprint(legacy), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.WriteHeader(http.StatusConflict)
				fmt.Fprint(w, `{"code":"directory_deletion_unconfirmed"}`)
				cancel()
			}))
			defer server.Close()
			api, _ := azclient.New(server.URL, "scope", "token", nil, time.Second)
			s := managedApplicationSchema(legacy)
			state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
			if d := state.SetAttribute(ctx, path.Root("id"), "stable"); d.HasError() {
				t.Fatal(d)
			}
			response := resource.DeleteResponse{State: state}
			if legacy {
				(&applicationResource{client: api}).Delete(ctx, resource.DeleteRequest{State: state}, &response)
			} else {
				(&applicationRequestResource{client: api}).Delete(ctx, resource.DeleteRequest{State: state}, &response)
			}
			if !response.Diagnostics.HasError() || !response.State.Raw.Equal(state.Raw) {
				t.Fatalf("cancelled cleanup must retain its original identity and state: %v", response.Diagnostics)
			}
		})
	}
}

func TestApplicationDeletionAndRefreshPreserveGraphFailures(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, read := range []bool{false, true} {
			for _, tc := range []struct {
				name      string
				status    int
				body      string
				wantError bool
			}{
				{"graph404", 404, `{"service":"Microsoft Graph","code":"Request_ResourceNotFound","title":"Microsoft Graph could not complete the request"}`, true},
				{"graph404WithoutCode", 404, `{"service":"Microsoft Graph"}`, true},
				{"oldGraph404", 404, `{"code":"Request_ResourceNotFound"}`, true},
				{"absent", 404, `{"code":"terraform_resource_not_found"}`, false},
				{"unqualified404", 404, ``, true},
				{"emptySuccess", 200, `{}`, true},
				{"wrongResource", 200, `{"resourceId":"different","status":"Ready"}`, true},
				{"unconfirmedCleanup", 202, `{}`, true},
				{"confirmedDeletion", 204, ``, false},
				{"denied", 403, `{"code":"access_denied"}`, true},
				{"propagating", 409, `{"code":"cleanup_pending"}`, true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					if read && tc.status == 204 {
						tc.wantError = true
					}
					ctx := context.Background()
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						w.Header().Set("Content-Type", "application/problem+json")
						w.WriteHeader(tc.status)
						_, _ = w.Write([]byte(tc.body))
					}))
					defer server.Close()
					api, err := azclient.New(server.URL, "scope", "token", nil, time.Second)
					if err != nil {
						t.Fatal(err)
					}
					s := managedApplicationSchema(legacy)
					state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
					if d := state.SetAttribute(ctx, path.Root("id"), "stable"); d.HasError() {
						t.Fatal(d)
					}
					if read {
						response := resource.ReadResponse{State: state}
						if legacy {
							(&applicationResource{client: api}).Read(ctx, resource.ReadRequest{State: state}, &response)
						} else {
							(&applicationRequestResource{client: api}).Read(ctx, resource.ReadRequest{State: state}, &response)
						}
						if response.Diagnostics.HasError() != tc.wantError {
							t.Fatalf("read: %v", response.Diagnostics)
						}
						if tc.wantError && response.State.Raw.IsNull() {
							t.Fatal("lost resource after dependency failure")
						}
					} else {
						response := resource.DeleteResponse{State: state}
						if legacy {
							(&applicationResource{client: api}).Delete(ctx, resource.DeleteRequest{State: state}, &response)
						} else {
							(&applicationRequestResource{client: api}).Delete(ctx, resource.DeleteRequest{State: state}, &response)
						}
						if response.Diagnostics.HasError() != tc.wantError {
							t.Fatalf("delete: %v", response.Diagnostics)
						}
					}
				})
			}
		}
	}
}
