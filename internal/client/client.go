package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type Client struct {
	endpoint    *url.URL
	scope       string
	credential  azcore.TokenCredential
	staticToken string
	httpClient  *http.Client
}

type APIError struct {
	StatusCode int
	Title      string `json:"title"`
	Detail     string `json:"detail"`
	TraceID    string `json:"traceId"`
}

func (e *APIError) Error() string {
	message := e.Detail
	if message == "" {
		message = e.Title
	}
	if message == "" {
		message = http.StatusText(e.StatusCode)
	}
	if e.TraceID != "" {
		message += " (trace " + e.TraceID + ")"
	}
	return fmt.Sprintf("AZExecute API returned HTTP %d: %s", e.StatusCode, message)
}

func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

func New(endpoint, scope, staticToken string, credential azcore.TokenCredential, timeout time.Duration) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(endpoint, "/"))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("endpoint must be an absolute HTTP or HTTPS URL")
	}
	if parsed.Scheme != "https" && parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1" {
		return nil, fmt.Errorf("endpoint must use HTTPS except for localhost development")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/"
	if staticToken == "" && credential == nil {
		return nil, fmt.Errorf("an access token or Azure credential is required")
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{endpoint: parsed, scope: scope, credential: credential, staticToken: staticToken, httpClient: &http.Client{Timeout: timeout}}, nil
}

func (c *Client) Capabilities(ctx context.Context) (*Capabilities, error) {
	var result Capabilities
	return &result, c.do(ctx, http.MethodGet, "api/terraform/v1/capabilities", nil, &result)
}

func (c *Client) CreateApplication(ctx context.Context, request ApplicationCreate) (*Application, error) {
	var result Application
	return &result, c.do(ctx, http.MethodPost, "api/terraform/v1/applications", request, &result)
}

func (c *Client) GetApplication(ctx context.Context, resourceID string) (*Application, error) {
	var result Application
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID)
	return &result, c.do(ctx, http.MethodGet, path, nil, &result)
}

func (c *Client) UpdateApplication(ctx context.Context, resourceID string, request ApplicationUpdate) (*Application, error) {
	var result Application
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID)
	return &result, c.do(ctx, http.MethodPut, path, request, &result)
}

func (c *Client) GetApplicationOwner(ctx context.Context, resourceID, ownerObjectID string) (*ApplicationOwner, error) {
	var result ApplicationOwner
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID) + "/owners/" + url.PathEscape(ownerObjectID)
	return &result, c.do(ctx, http.MethodGet, path, nil, &result)
}

func (c *Client) AddApplicationOwner(ctx context.Context, resourceID, ownerObjectID string) (*ApplicationOwner, error) {
	var result ApplicationOwner
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID) + "/owners/" + url.PathEscape(ownerObjectID)
	return &result, c.do(ctx, http.MethodPut, path, nil, &result)
}

func (c *Client) RemoveApplicationOwner(ctx context.Context, resourceID, ownerObjectID string) error {
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID) + "/owners/" + url.PathEscape(ownerObjectID)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) DeleteApplication(ctx context.Context, resourceID string) error {
	return c.do(ctx, http.MethodDelete, "api/terraform/v1/applications/"+url.PathEscape(resourceID), nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, input, output any) error {
	var payload []byte
	if input != nil {
		var err error
		payload, err = json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode AZExecute request: %w", err)
		}
	}

	target := c.endpoint.ResolveReference(&url.URL{Path: strings.TrimLeft(path, "/")})
	token := c.staticToken
	if token == "" {
		accessToken, tokenErr := c.credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{c.scope}})
		if tokenErr != nil {
			return fmt.Errorf("acquire AZExecute access token: %w", tokenErr)
		}
		token = accessToken.Token
	}

	for attempt := 0; attempt < 4; attempt++ {
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
		if err != nil {
			return fmt.Errorf("create AZExecute request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "terraform-provider-azexecute")
		req.Header.Set("Authorization", "Bearer "+token)
		if input != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		response, requestErr := c.httpClient.Do(req)
		if requestErr != nil {
			if attempt < 3 && ctx.Err() == nil {
				if waitErr := waitForRetry(ctx, retryDelay(attempt, "")); waitErr != nil {
					return waitErr
				}
				continue
			}
			return fmt.Errorf("call AZExecute API: %w", requestErr)
		}

		if isTransientStatus(response.StatusCode) && attempt < 3 {
			retryAfter := response.Header.Get("Retry-After")
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
			response.Body.Close()
			if waitErr := waitForRetry(ctx, retryDelay(attempt, retryAfter)); waitErr != nil {
				return waitErr
			}
			continue
		}

		if response.StatusCode < 200 || response.StatusCode >= 300 {
			apiErr := &APIError{StatusCode: response.StatusCode}
			limited := io.LimitReader(response.Body, 1<<20)
			if decodeErr := json.NewDecoder(limited).Decode(apiErr); decodeErr != nil && decodeErr != io.EOF {
				apiErr.Detail = "response was not valid problem JSON"
			}
			response.Body.Close()
			return apiErr
		}
		if output == nil || response.StatusCode == http.StatusNoContent {
			response.Body.Close()
			return nil
		}
		decodeErr := json.NewDecoder(io.LimitReader(response.Body, 10<<20)).Decode(output)
		response.Body.Close()
		if decodeErr != nil {
			return fmt.Errorf("decode AZExecute response: %w", decodeErr)
		}
		return nil
	}
	return fmt.Errorf("call AZExecute API: retry limit exhausted")
}

func isTransientStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout ||
		statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusInternalServerError ||
		statusCode == http.StatusBadGateway ||
		statusCode == http.StatusServiceUnavailable ||
		statusCode == http.StatusGatewayTimeout
}

func retryDelay(attempt int, retryAfter string) time.Duration {
	if retryAfter != "" {
		if seconds, err := time.ParseDuration(strings.TrimSpace(retryAfter) + "s"); err == nil && seconds >= 0 {
			if seconds > 30*time.Second {
				return 30 * time.Second
			}
			return seconds
		}
		if when, err := http.ParseTime(retryAfter); err == nil {
			delay := time.Until(when)
			if delay > 0 && delay < 30*time.Second {
				return delay
			}
		}
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
