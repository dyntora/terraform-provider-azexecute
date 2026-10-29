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
	"sort"
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
	Title      string              `json:"title"`
	Detail     string              `json:"detail"`
	TraceID    string              `json:"traceId"`
	Code       string              `json:"code"`
	Service    string              `json:"service"`
	Errors     map[string][]string `json:"errors"`
}

func (e *APIError) Error() string {
	message := e.Detail
	if message == "" {
		message = e.Title
	}
	if message == "" {
		message = defaultErrorDetail(e.StatusCode)
	}
	fields := make([]string, 0, len(e.Errors))
	for field := range e.Errors {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	for _, field := range fields {
		seen := make(map[string]bool)
		for _, detail := range e.Errors[field] {
			detail = strings.TrimSpace(detail)
			if detail != "" && !seen[detail] {
				message += "\n- " + terraformFieldName(field) + ": " + detail
				seen[detail] = true
			}
		}
	}
	if e.Code != "" && e.Code != e.Title && !strings.Contains(message, e.Code) {
		message += "\nError code: " + e.Code
	}
	if e.TraceID != "" && !strings.Contains(message, e.TraceID) {
		message += " (trace " + e.TraceID + ")"
	}
	return fmt.Sprintf("AZExecute API returned HTTP %d: %s", e.StatusCode, message)
}

func isResourceNotFound(err error, code string) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound &&
		apiErr.Service == "" && apiErr.Code == code
}

func IsGroupNotFound(err error) bool { return isResourceNotFound(err, "terraform_group_not_found") }
func IsGroupMembershipNotFound(err error) bool {
	return isResourceNotFound(err, "terraform_group_membership_not_found")
}
func IsEventListenerNotFound(err error) bool {
	return isResourceNotFound(err, "event_listener_not_found")
}

// Only an explicit Terraform API code proves absence. Bare routing/proxy 404s
// must retain state so that retries cannot create duplicate applications.
func IsApplicationNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound &&
		apiErr.Service == "" && apiErr.Code == "terraform_resource_not_found"
}

func IsDeleteNotFound(err error) bool { return IsApplicationNotFound(err) }

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
	if err := c.validateMetadataBeforeWrite(ctx, request.Metadata); err != nil {
		return nil, err
	}
	if request.Registration != nil {
		capabilities, err := c.Capabilities(ctx)
		if err != nil {
			return nil, err
		}
		if !capabilities.SupportsRegistrationRequests {
			return nil, fmt.Errorf("AZExecute API does not support registration configuration in requests; upgrade the API before submitting this request")
		}
	}
	var result Application
	err := c.do(ctx, http.MethodPost, "api/terraform/v1/applications", request, &result)
	return &result, validateApplicationResponse(result, "", err)
}

func (c *Client) GetApplication(ctx context.Context, resourceID string) (*Application, error) {
	var result Application
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID)
	err := c.do(ctx, http.MethodGet, path, nil, &result)
	return &result, validateApplicationResponse(result, resourceID, err)
}

func (c *Client) UpdateApplication(ctx context.Context, resourceID string, request ApplicationUpdate) (*Application, error) {
	if request.APIPermissionRequests != nil {
		capabilities, err := c.Capabilities(ctx)
		if err != nil {
			return nil, err
		}
		if !capabilities.SupportsPermissionRequestUpdates {
			return nil, fmt.Errorf("this API does not support in-place api_permission_request updates; upgrade AZExecute before applying this change. The application must not be replaced")
		}
	}
	if err := c.validateMetadataBeforeWrite(ctx, request.Metadata); err != nil {
		return nil, err
	}
	if request.Details != nil {
		capabilities, err := c.Capabilities(ctx)
		if err != nil {
			return nil, err
		}
		if !capabilities.SupportsApplicationDetailsUpdates {
			return nil, fmt.Errorf("this API does not support in-place display_name or description updates; upgrade AZExecute before applying this change")
		}
	}
	var result Application
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID)
	err := c.do(ctx, http.MethodPut, path, request, &result)
	return &result, validateApplicationResponse(result, resourceID, err)
}

func validateApplicationResponse(result Application, expectedID string, err error) error {
	if err != nil {
		return err
	}
	if strings.TrimSpace(result.ResourceID) == "" || strings.TrimSpace(result.Status) == "" ||
		(expectedID != "" && !strings.EqualFold(result.ResourceID, expectedID)) {
		return fmt.Errorf("AZExecute returned an incomplete or mismatched application response; existing Terraform state is retained. Check the API deployment and proxy routing, then retry. A write may have completed; check AZExecute before creating another application")
	}
	return nil
}

func (c *Client) GetApplicationOwner(ctx context.Context, resourceID, ownerObjectID string) (*ApplicationOwner, error) {
	var result ApplicationOwner
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID) + "/owners/" + url.PathEscape(ownerObjectID)
	err := c.do(ctx, http.MethodGet, path, nil, &result)
	return &result, validateOwnerResponse(result, resourceID, ownerObjectID, err)
}

func (c *Client) AddApplicationOwner(ctx context.Context, resourceID, ownerObjectID string) (*ApplicationOwner, error) {
	var result ApplicationOwner
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID) + "/owners/" + url.PathEscape(ownerObjectID)
	err := c.do(ctx, http.MethodPut, path, nil, &result)
	return &result, validateOwnerResponse(result, resourceID, ownerObjectID, err)
}

func validateOwnerResponse(result ApplicationOwner, resourceID, ownerObjectID string, err error) error {
	if err != nil {
		return err
	}
	if result.ResourceID == "" || result.OwnerObjectID == "" || !strings.EqualFold(result.ResourceID, resourceID) || !strings.EqualFold(result.OwnerObjectID, ownerObjectID) {
		return fmt.Errorf("AZExecute returned an incomplete or mismatched application-owner response; check the API deployment and proxy routing, then retry")
	}
	return nil
}

func (c *Client) RemoveApplicationOwner(ctx context.Context, resourceID, ownerObjectID string) error {
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID) + "/owners/" + url.PathEscape(ownerObjectID)
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

func (c *Client) DeleteApplication(ctx context.Context, resourceID string) error {
	return c.deleteApplication(ctx, resourceID, 5*time.Minute, 5*time.Second)
}

func (c *Client) do(ctx context.Context, method, path string, input, output any) error {
	return c.doWithRetry(ctx, method, path, input, output, true)
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, input, output any, allowRetry bool) error {
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
			if allowRetry && attempt < 3 && ctx.Err() == nil {
				if waitErr := waitForRetry(ctx, retryDelay(attempt, "")); waitErr != nil {
					return waitErr
				}
				continue
			}
			return fmt.Errorf("call AZExecute API: %w", requestErr)
		}

		if allowRetry && isTransientStatus(response.StatusCode) && attempt < 3 {
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
			payload, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
			if readErr != nil {
				apiErr.Detail = "unable to read error response"
			} else if len(payload) > 0 {
				if decodeErr := json.Unmarshal(payload, apiErr); decodeErr != nil {
					// Shared public endpoints may return a JSON validation string instead of ProblemDetails.
					var message string
					if json.Unmarshal(payload, &message) == nil && len(message) <= 2000 {
						apiErr.Detail = message
					} else {
						apiErr.Detail = "response was not valid problem JSON"
					}
				}
			}
			response.Body.Close()
			if apiErr.TraceID == "" {
				apiErr.TraceID = response.Header.Get("X-Correlation-ID")
			}
			return apiErr
		}
		if method == http.MethodDelete && response.StatusCode != http.StatusNoContent {
			response.Body.Close()
			return fmt.Errorf("AZExecute did not confirm deletion (HTTP %d; expected 204). Terraform state is retained; check the API/proxy and retry after cleanup completes", response.StatusCode)
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
