package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSecretRotationRetainsStateOnUnconfirmedResponse(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	for _, tc := range []struct {
		name, method string
		status       int
		body         string
		absent       bool
	}{
		{"absent", "GET", 404, `{"code":"terraform_secret_rotation_not_found"}`, true},
		{"proxy", "GET", 404, "Not found", false},
		{"wrong app", "GET", 200, `{"applicationEntityId":"aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee","lifetimeDays":30,"renewBeforeHours":12}`, false},
		{"sparse", "GET", 200, `{}`, false},
		{"different schedule", "PUT", 200, `{"applicationEntityId":"11111111-2222-4333-8444-555555555555","lifetimeDays":30,"renewBeforeHours":1}`, false},
		{"incomplete disable", "DELETE", 202, `{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.Path != "/api/terraform/v1/managed-applications/"+id+"/secret-rotation" {
					t.Errorf("unexpected rotation operation: %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c, err := New(server.URL, "", "test", nil, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			switch tc.method {
			case "DELETE":
				err = c.DisableSecretRotation(context.Background(), id)
			case "PUT":
				_, err = c.SecretRotation(context.Background(), id, &SecretRotationSettings{LifetimeDays: 30, RenewBeforeHours: 12})
			default:
				_, err = c.SecretRotation(context.Background(), id, nil)
			}
			if err == nil || IsSecretRotationNotFound(err) != tc.absent {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}
}
