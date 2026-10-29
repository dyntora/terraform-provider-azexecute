package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Integration struct {
	ResourceID    *string                    `json:"resourceId"`
	IntegrationID int64                      `json:"integrationId"`
	Kind          string                     `json:"kind"`
	Configuration map[string]json.RawMessage `json:"configuration"`
}

type AWSFederationSetup struct {
	ProviderURL string `json:"providerUrl"`
	Audience    string `json:"audience"`
}

func (c *Client) GetAWSFederationSetup(ctx context.Context) (*AWSFederationSetup, error) {
	var result AWSFederationSetup
	if err := c.do(ctx, http.MethodGet, "api/v1/Integrations/AWS/FederationSetup", nil, &result); err != nil {
		return nil, err
	}
	u, err := url.Parse(result.ProviderURL)
	if err != nil || u.Scheme != "https" || u.Host != "login.microsoftonline.com" || result.Audience == "" {
		return nil, fmt.Errorf("AZExecute returned incomplete AWS federation settings")
	}
	return &result, nil
}

func integrationPath(kind, id string) string {
	return "api/terraform/v1/integrations/" + url.PathEscape(kind) + "/" + url.PathEscape(id)
}
func (c *Client) integration(ctx context.Context, method, path, kind, id string, numeric int64, lookup bool, input any) (*Integration, error) {
	var result Integration
	if err := c.do(ctx, method, path, input, &result); err != nil {
		return nil, err
	}
	if result.Kind != kind || result.IntegrationID <= 0 || (numeric > 0 && result.IntegrationID != numeric) || result.Configuration == nil {
		return nil, fmt.Errorf("AZExecute returned an incomplete or mismatched integration; Terraform state is retained")
	}
	if result.ResourceID == nil {
		if !lookup {
			return nil, fmt.Errorf("AZExecute did not confirm the integration resource identity")
		}
	} else {
		parsed, err := uuid.Parse(*result.ResourceID)
		if err != nil || parsed == uuid.Nil || (id != "" && !strings.EqualFold(id, *result.ResourceID)) {
			return nil, fmt.Errorf("AZExecute returned a mismatched integration identity; Terraform state is retained")
		}
	}
	required := []string{"name", "enabled", "global"}
	switch kind {
	case "aws":
		required = append(required, "accountId", "service", "roleArn")
	case "vercel":
		required = append(required, "teamId")
	case "github":
		required = append(required, "appId", "installationId")
	}
	for _, key := range required {
		raw, ok := result.Configuration[key]
		if !ok || string(raw) == "null" {
			return nil, fmt.Errorf("AZExecute returned an incomplete integration configuration; Terraform state is retained")
		}
		if key == "enabled" || key == "global" {
			var v bool
			if json.Unmarshal(raw, &v) != nil {
				return nil, fmt.Errorf("invalid integration boolean")
			}
		} else {
			var v string
			if json.Unmarshal(raw, &v) != nil || v == "" {
				return nil, fmt.Errorf("invalid integration configuration")
			}
		}
	}
	return &result, nil
}
func (c *Client) CreateIntegration(ctx context.Context, kind, id string, input any) (*Integration, error) {
	return c.integration(ctx, http.MethodPost, integrationPath(kind, id), kind, id, 0, false, input)
}
func (c *Client) UpdateIntegration(ctx context.Context, kind, id string, input any) (*Integration, error) {
	return c.integration(ctx, http.MethodPut, integrationPath(kind, id), kind, id, 0, false, input)
}
func (c *Client) GetIntegration(ctx context.Context, kind, id string) (*Integration, error) {
	return c.integration(ctx, http.MethodGet, integrationPath(kind, id), kind, id, 0, false, nil)
}
func (c *Client) LookupIntegration(ctx context.Context, kind string, id int64, adopt bool) (*Integration, error) {
	method, action := http.MethodGet, "lookup"
	if adopt {
		method, action = http.MethodPost, "import"
	}
	return c.integration(ctx, method, integrationPath(kind, action)+"/"+strconv.FormatInt(id, 10), kind, "", id, !adopt, nil)
}
func (c *Client) DeleteIntegration(ctx context.Context, kind, id string) error {
	// A successful DELETE must be followed by an explicit, typed absence response.
	if err := c.do(ctx, http.MethodDelete, integrationPath(kind, id), nil, nil); err != nil {
		return err
	}
	_, err := c.GetIntegration(ctx, kind, id)
	if IsIntegrationNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("AZExecute has not confirmed integration deletion; Terraform state is retained")
}
func IsIntegrationNotFound(err error) bool {
	var e *APIError
	return errors.As(err, &e) && e.StatusCode == 404 && e.Code == "terraform_integration_not_found" && e.Service == ""
}
