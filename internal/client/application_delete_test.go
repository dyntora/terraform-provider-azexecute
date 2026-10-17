package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestApplicationDeleteWaitsForConfirmedCleanup(t *testing.T) {
	for _, terminalStatus := range []int{204, 404} {
		t.Run(fmt.Sprint(terminalStatus), func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/api/terraform/v1/applications/stable" {
					t.Errorf("unexpected cleanup request: %s %s", r.Method, r.URL.Path)
				}
				switch attempts.Add(1) {
				case 1:
					w.WriteHeader(409)
					fmt.Fprint(w, `{"code":"directory_deletion_unconfirmed"}`)
				case 2:
					w.WriteHeader(409)
					fmt.Fprint(w, `{"code":"terraform_application_operation_in_progress"}`)
				default:
					w.WriteHeader(terminalStatus)
					if terminalStatus == 404 {
						fmt.Fprint(w, `{"code":"terraform_resource_not_found"}`)
					}
				}
			}))
			defer server.Close()
			api, _ := New(server.URL, "scope", "token", nil, time.Second)
			err := api.deleteApplication(context.Background(), "stable", time.Second, time.Millisecond)
			if (err != nil && !IsDeleteNotFound(err)) || attempts.Load() != 3 {
				t.Fatalf("cleanup did not converge: attempts=%d error=%v", attempts.Load(), err)
			}
		})
	}
}

func TestApplicationDeleteDoesNotRetryUnrelatedErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{
		{409, `{"code":"application_request_changed"}`},
		{409, `{"code":"terraform_resource_conflict"}`},
		{403, `{"code":"terraform_application_deletion_disabled"}`},
		{403, `{"code":"terraform_resource_access_denied"}`},
		{404, `{"service":"Microsoft Graph","code":"Request_ResourceNotFound"}`},
		{409, `{"service":"Microsoft Graph","code":"directory_deletion_unconfirmed"}`},
		{400, `{"code":"directory_deletion_unconfirmed"}`},
		{202, `{}`},
	} {
		t.Run(tc.body+fmt.Sprint(tc.status), func(t *testing.T) {
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			api, _ := New(server.URL, "scope", "token", nil, time.Second)
			err := api.deleteApplication(context.Background(), "stable", time.Second, time.Millisecond)
			if err == nil || IsDeleteNotFound(err) || attempts.Load() != 1 {
				t.Fatalf("unexpected retry/success: attempts=%d error=%v", attempts.Load(), err)
			}
		})
	}
}

func TestApplicationDeleteTimeoutAndCancellationExplainRecovery(t *testing.T) {
	for _, cancelRequest := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelRequest), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var attempts atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts.Add(1)
				w.WriteHeader(409)
				fmt.Fprint(w, `{"code":"directory_deletion_unconfirmed","traceId":"cleanup-reference"}`)
			}))
			defer server.Close()
			api, _ := New(server.URL, "scope", "token", nil, time.Second)
			if cancelRequest {
				cancel()
			}
			err := api.deleteApplication(ctx, "stable", 40*time.Millisecond, time.Millisecond)
			want := context.DeadlineExceeded
			if cancelRequest {
				want = context.Canceled
			}
			if !errors.Is(err, want) || IsDeleteNotFound(err) || !strings.Contains(err.Error(), "Terraform state is retained") || !strings.Contains(err.Error(), "stable") {
				t.Fatalf("missing safe recovery guidance: %v", err)
			}
			if !cancelRequest && (attempts.Load() < 1 || !strings.Contains(err.Error(), "cleanup-reference")) {
				t.Fatalf("missing last cleanup response: attempts=%d error=%v", attempts.Load(), err)
			}
		})
	}
}
