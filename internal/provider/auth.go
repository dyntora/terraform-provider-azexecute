package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

type credentialConfig struct {
	TenantID             string
	ClientID             string
	ClientSecret         string
	CertificatePath      string
	CertificatePassword  string
	SendCertificateChain bool
	UseOIDC              bool
	OIDCToken            string
	OIDCTokenFilePath    string
	OIDCAudience         string
	UseManagedIdentity   bool
}

func buildCredential(ctx context.Context, config credentialConfig) (azcore.TokenCredential, error) {
	githubRequestURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	githubRequestToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	useOIDC := config.UseOIDC || config.OIDCToken != "" || config.OIDCTokenFilePath != "" || (githubRequestURL != "" && githubRequestToken != "")

	if useOIDC {
		if err := requireTenantAndClient(config, "OIDC"); err != nil {
			return nil, err
		}
		assertionSource, err := buildAssertionSource(config, githubRequestURL, githubRequestToken)
		if err != nil {
			return nil, err
		}
		return azidentity.NewClientAssertionCredential(config.TenantID, config.ClientID, assertionSource, nil)
	}

	if config.CertificatePath != "" {
		if err := requireTenantAndClient(config, "client-certificate"); err != nil {
			return nil, err
		}
		certificateData, err := os.ReadFile(config.CertificatePath)
		if err != nil {
			return nil, fmt.Errorf("read client certificate: %w", err)
		}
		certificates, privateKey, err := azidentity.ParseCertificates(certificateData, []byte(config.CertificatePassword))
		if err != nil {
			return nil, fmt.Errorf("parse client certificate: %w", err)
		}
		return azidentity.NewClientCertificateCredential(
			config.TenantID,
			config.ClientID,
			certificates,
			privateKey,
			&azidentity.ClientCertificateCredentialOptions{SendCertificateChain: config.SendCertificateChain})
	}

	if config.ClientSecret != "" {
		if err := requireTenantAndClient(config, "client-secret"); err != nil {
			return nil, err
		}
		return azidentity.NewClientSecretCredential(config.TenantID, config.ClientID, config.ClientSecret, nil)
	}

	if config.UseManagedIdentity {
		options := &azidentity.ManagedIdentityCredentialOptions{}
		if config.ClientID != "" {
			options.ID = azidentity.ClientID(config.ClientID)
		}
		return azidentity.NewManagedIdentityCredential(options)
	}

	// DefaultAzureCredential covers Azure workload identity, system/user-assigned
	// managed identity (AZURE_CLIENT_ID), Azure CLI, Azure Developer CLI, and
	// Azure PowerShell. TenantID constrains tenant-aware developer credentials.
	return azidentity.NewDefaultAzureCredential(&azidentity.DefaultAzureCredentialOptions{TenantID: config.TenantID})
}

func requireTenantAndClient(config credentialConfig, method string) error {
	if config.TenantID == "" || config.ClientID == "" {
		return fmt.Errorf("tenant_id and client_id are required for %s authentication", method)
	}
	return nil
}

func buildAssertionSource(config credentialConfig, githubRequestURL, githubRequestToken string) (func(context.Context) (string, error), error) {
	if config.OIDCToken != "" {
		token := strings.TrimSpace(config.OIDCToken)
		if token == "" {
			return nil, fmt.Errorf("oidc_token cannot be empty")
		}
		return func(context.Context) (string, error) { return token, nil }, nil
	}
	if config.OIDCTokenFilePath != "" {
		path := config.OIDCTokenFilePath
		return func(context.Context) (string, error) {
			payload, err := os.ReadFile(path)
			if err != nil {
				return "", fmt.Errorf("read OIDC token file: %w", err)
			}
			if len(payload) > 1<<20 {
				return "", fmt.Errorf("OIDC token file exceeds 1 MiB")
			}
			token := strings.TrimSpace(string(payload))
			if token == "" {
				return "", fmt.Errorf("OIDC token file is empty")
			}
			return token, nil
		}, nil
	}
	if githubRequestURL == "" || githubRequestToken == "" {
		return nil, fmt.Errorf("OIDC authentication requires oidc_token, oidc_token_file_path, or the GitHub Actions OIDC environment")
	}
	audience := config.OIDCAudience
	if audience == "" {
		audience = "api://AzureADTokenExchange"
	}
	return func(ctx context.Context) (string, error) {
		return requestGitHubAssertion(ctx, githubRequestURL, githubRequestToken, audience)
	}, nil
}

func requestGitHubAssertion(ctx context.Context, requestURL, requestToken, audience string) (string, error) {
	return requestGitHubAssertionWithClient(ctx, requestURL, requestToken, audience, &http.Client{Timeout: 15 * time.Second})
}

func requestGitHubAssertionWithClient(ctx context.Context, requestURL, requestToken, audience string, client *http.Client) (string, error) {
	parsed, err := url.Parse(requestURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", fmt.Errorf("GitHub OIDC request URL must be an absolute HTTPS URL")
	}
	query := parsed.Query()
	query.Set("audience", audience)
	parsed.RawQuery = query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", fmt.Errorf("create GitHub OIDC request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+requestToken)
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return "", fmt.Errorf("request GitHub OIDC assertion: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return "", fmt.Errorf("GitHub OIDC endpoint returned HTTP %d", response.StatusCode)
	}
	var payload struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode GitHub OIDC response: %w", err)
	}
	if strings.TrimSpace(payload.Value) == "" {
		return "", fmt.Errorf("GitHub OIDC response contained no assertion")
	}
	return strings.TrimSpace(payload.Value), nil
}
