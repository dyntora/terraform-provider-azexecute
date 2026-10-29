package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIntegrationResponsesRequireMatchingIdentityAndCompleteSettings(t *testing.T) {
	const id = "11111111-2222-4333-8444-555555555555"
	for _, scenario := range []string{"valid", "wrong_kind", "wrong_id", "missing_name", "missing_boolean", "missing_identity", "wrong_numeric"} {
		t.Run(scenario, func(t *testing.T) {
			data := map[string]any{"resourceId": id, "integrationId": 42, "kind": "aws", "configuration": map[string]any{"name": "AWS", "enabled": true, "global": false, "accountId": "123456789012", "roleArn": "arn:aws:iam::123456789012:role/test", "service": "route53"}}
			switch scenario {
			case "wrong_kind":
				data["kind"] = "github"
			case "wrong_id":
				data["resourceId"] = "22222222-2222-4333-8444-555555555555"
			case "missing_identity":
				data["resourceId"] = nil
			case "missing_name":
				delete(data["configuration"].(map[string]any), "name")
			case "missing_boolean":
				delete(data["configuration"].(map[string]any), "global")
			case "wrong_numeric":
				data["integrationId"] = 43
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(data)
			}))
			defer server.Close()
			c, _ := New(server.URL, "ignored", "token", nil, time.Second)
			var err error
			if scenario == "wrong_numeric" {
				_, err = c.LookupIntegration(context.Background(), "aws", 42, false)
			} else {
				_, err = c.GetIntegration(context.Background(), "aws", id)
			}
			if (scenario == "valid") != (err == nil) {
				t.Fatalf("unexpected response validation: %v", err)
			}
		})
	}
}

func TestIntegrationDeleteRequiresConfirmedAbsence(t *testing.T) {
	for _, scenario := range []string{"confirmed", "proxy404", "forbidden", "html", "still_present"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "DELETE" {
					w.WriteHeader(204)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				switch scenario {
				case "confirmed":
					w.WriteHeader(404)
					_, _ = w.Write([]byte(`{"code":"terraform_integration_not_found"}`))
				case "proxy404":
					w.WriteHeader(404)
				case "forbidden":
					w.WriteHeader(403)
				case "html":
					w.Header().Set("Content-Type", "text/html")
					_, _ = w.Write([]byte("<html>login</html>"))
				case "still_present":
					_, _ = w.Write([]byte(`{"resourceId":"11111111-2222-4333-8444-555555555555","integrationId":42,"kind":"vercel","configuration":{"name":"Existing","enabled":true,"global":false,"teamId":"team"}}`))
				}
			}))
			defer server.Close()
			c, _ := New(server.URL, "ignored", "token", nil, time.Second)
			err := c.DeleteIntegration(context.Background(), "vercel", "11111111-2222-4333-8444-555555555555")
			if (scenario == "confirmed") != (err == nil) {
				t.Fatalf("unexpected delete result: %v", err)
			}
		})
	}
}
