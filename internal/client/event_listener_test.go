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
