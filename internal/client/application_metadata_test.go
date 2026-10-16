package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetadataCompatibilityBeforeCreateAndUpdate(t *testing.T) {
	for _, modern := range []bool{false, true} {
		for _, visible := range []bool{false, true} {
			for _, update := range []bool{false, true} {
				mutations := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/api/terraform/v1/capabilities" {
						fields := []string{}
						if visible {
							fields = []string{"requires_elevated_permissions", "elevated_permissions_justification"}
						}
						_ = json.NewEncoder(w).Encode(Capabilities{PreservesHiddenMetadata: modern, IncludedMetadataFields: fields})
						return
					}
					mutations++
					_ = json.NewEncoder(w).Encode(Application{ResourceID: "existing", Status: "Ready"})
				}))
				api, _ := New(server.URL, "scope", "token", nil, time.Second)
				justification := "Sensitive metadata must not appear in diagnostics"
				metadata := ApplicationMetadata{RequiresElevatedPermissions: true, ElevatedPermissionsJustification: &justification}
				var err error
				if update {
					_, err = api.UpdateApplication(context.Background(), "existing", ApplicationUpdate{Metadata: metadata})
				} else {
					_, err = api.CreateApplication(context.Background(), ApplicationCreate{Metadata: metadata})
				}
				server.Close()
				if modern || visible {
					if err != nil || mutations != 1 {
						t.Fatalf("compatible write blocked: %v, writes=%d", err, mutations)
					}
				} else {
					if err == nil || mutations != 0 {
						t.Fatalf("unsafe write was allowed: %v, writes=%d", err, mutations)
					}
					if !strings.Contains(err.Error(), "Upgrade AZExecute") || !strings.Contains(err.Error(), "elevated_permissions_justification") || strings.Contains(err.Error(), justification) {
						t.Fatalf("missing or unsafe feedback: %v", err)
					}
				}
			}
		}
	}
}

func TestLegacyMetadataCompatibilityHandlesIndependentControlsAndDefaults(t *testing.T) {
	for _, tc := range []struct {
		elevated, justification bool
		fields                  []string
		blocked                 bool
	}{
		{false, false, nil, false},
		{true, false, nil, true},
		{true, false, []string{"requires_elevated_permissions"}, false},
		{false, true, []string{"requires_elevated_permissions"}, true},
		{false, true, []string{"elevated_permissions_justification"}, true},
		{true, true, []string{"requires_elevated_permissions", "elevated_permissions_justification"}, false},
	} {
		err := ValidateMetadataPreservation(&Capabilities{IncludedMetadataFields: tc.fields}, tc.elevated, tc.justification)
		if (err != nil) != tc.blocked {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}
