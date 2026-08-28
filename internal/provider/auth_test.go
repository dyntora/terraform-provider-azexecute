package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAssertionSourceReadsRotatingTokenFile(t *testing.T) {
	t.Parallel()
	path := t.TempDir() + "/token"
	if err := os.WriteFile(path, []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source, err := buildAssertionSource(credentialConfig{OIDCTokenFilePath: path}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := source(context.Background())
	if err != nil || first != "first" {
		t.Fatalf("unexpected first assertion: %q, %v", first, err)
	}
	if err := os.WriteFile(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := source(context.Background())
	if err != nil || second != "second" {
		t.Fatalf("rotated assertion was not re-read: %q, %v", second, err)
	}
}

func TestGitHubAssertionUsesBearerTokenAndExchangeAudience(t *testing.T) {
	t.Parallel()
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer request-token" {
			t.Errorf("missing request bearer token")
		}
		if request.URL.Query().Get("audience") != "api://AzureADTokenExchange" {
			t.Errorf("unexpected audience: %s", request.URL.Query().Get("audience"))
		}
		_, _ = fmt.Fprint(response, `{"value":"github-assertion"}`)
	}))
	defer server.Close()

	assertion, err := requestGitHubAssertionWithClient(
		context.Background(),
		server.URL+"?x=1",
		"request-token",
		"api://AzureADTokenExchange",
		server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if assertion != "github-assertion" {
		t.Fatalf("unexpected assertion: %s", assertion)
	}
}

func TestGitHubAssertionRejectsInsecureEndpoint(t *testing.T) {
	t.Parallel()
	_, err := requestGitHubAssertion(context.Background(), "http://example.com", "token", "audience")
	if err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Fatalf("expected HTTPS validation error, got %v", err)
	}
}
