package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestListenerResponsesRejectMissingAndChangedIdentity(t *testing.T) {
	for _, body := range []string{`{}`, `{"id":0,"name":"Rule","eventType":"ApplicationCreated","actionType":"AutomationTask"}`, `{"id":99,"name":"Rule","eventType":"ApplicationCreated","actionType":"AutomationTask"}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		api, _ := New(server.URL, "scope", "token", nil, time.Second)
		if _, err := api.GetEventListener(context.Background(), 7); err == nil {
			t.Fatal("invalid read response accepted")
		}
		if _, err := api.UpdateEventListener(context.Background(), 7, EventListener{}); err == nil {
			t.Fatal("invalid update response accepted")
		}
		server.Close()
	}
}

func TestResourceAbsenceRequiresMatchingCodeAndNoDependencyService(t *testing.T) {
	for code, check := range map[string]func(error) bool{
		"terraform_group_not_found": IsGroupNotFound, "terraform_group_membership_not_found": IsGroupMembershipNotFound, "event_listener_not_found": IsEventListenerNotFound,
	} {
		for _, actual := range []string{code, "", "terraform_resource_not_found", "access_denied"} {
			for _, service := range []string{"", "graph"} {
				err := &APIError{StatusCode: 404, Code: actual, Service: service}
				if check(err) != (actual == code && service == "") {
					t.Fatalf("incorrect absence classification: %+v", err)
				}
			}
		}
	}
}

func TestListenerCreateDoesNotRetryAmbiguousServerFailure(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	api, err := New(server.URL, "ignored", "token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := api.CreateEventListener(context.Background(), EventListener{Name: "renewal"}); err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 1 {
		t.Fatal("non-idempotent creation was retried", calls.Load())
	}
}

func TestListenerValidationErrorPreservesPublicApiMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`"Choose Secret, Certificate, or all credentials for a credential event."`))
	}))
	defer server.Close()
	api, err := New(server.URL, "ignored", "token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = api.CreateEventListener(context.Background(), EventListener{Name: "renewal"})
	if err == nil || !strings.Contains(err.Error(), "Choose Secret, Certificate") {
		t.Fatal("validation detail lost", err)
	}
}
