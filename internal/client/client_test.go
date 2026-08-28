package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestCapabilitiesUsesBearerTokenAndVersionedPath(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/terraform/v1/capabilities" {
			t.Errorf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected authorization header")
		}
		response.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(response).Encode(Capabilities{APIVersion: "1", Enabled: true, AllowApplicationCreation: true})
	}))
	defer server.Close()

	api, err := New(server.URL, "ignored", "test-token", nil, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	result, err := api.Capabilities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Enabled || !result.AllowApplicationCreation || result.APIVersion != "1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRetriesTransientResponsesWithReusablePostBody(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var payload ApplicationCreate
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("attempt body could not be decoded: %v", err)
		}
		if payload.ResourceID != "retry-id" {
			t.Errorf("request body was not preserved: %#v", payload)
		}
		if attempts.Add(1) < 3 {
			response.Header().Set("Retry-After", "0")
			response.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(response).Encode(Application{ResourceID: payload.ResourceID, DisplayName: "retry", Status: "Ready"})
	}))
	defer server.Close()
	api, _ := New(server.URL, "ignored", "token", nil, time.Second)
	result, err := api.CreateApplication(context.Background(), ApplicationCreate{ResourceID: "retry-id", DisplayName: "retry"})
	if err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 3 || result.Status != "Ready" {
		t.Fatalf("unexpected retry result after %d attempts: %#v", attempts.Load(), result)
	}
}

func TestCreateApplicationAcceptsAsyncResponse(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", request.Method)
		}
		var payload ApplicationCreate
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.ResourceID != "resource-id" || payload.DisplayName != "example" {
			t.Errorf("unexpected payload: %#v", payload)
		}
		response.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(response).Encode(Application{ResourceID: payload.ResourceID, DisplayName: payload.DisplayName, Status: "PendingApproval"})
	}))
	defer server.Close()
	api, _ := New(server.URL, "ignored", "token", nil, time.Second)
	result, err := api.CreateApplication(context.Background(), ApplicationCreate{ResourceID: "resource-id", DisplayName: "example"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "PendingApproval" {
		t.Fatalf("unexpected status: %s", result.Status)
	}
}

func TestProblemDetailsAreReturnedAsTypedError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/problem+json")
		response.WriteHeader(http.StatusNotFound)
		_, _ = response.Write([]byte(`{"title":"terraform_resource_not_found","detail":"missing","traceId":"abc"}`))
	}))
	defer server.Close()
	api, _ := New(server.URL, "ignored", "token", nil, time.Second)
	_, err := api.GetApplication(context.Background(), "missing")
	if !IsNotFound(err) {
		t.Fatalf("expected typed not-found error, got %v", err)
	}
}

func TestRejectsInsecureRemoteEndpoint(t *testing.T) {
	t.Parallel()
	if _, err := New("http://example.com", "scope", "token", nil, time.Second); err == nil {
		t.Fatal("expected insecure endpoint to be rejected")
	}
}
