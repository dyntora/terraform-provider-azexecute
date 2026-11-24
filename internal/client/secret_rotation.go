package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type SecretRotationSettings struct {
	LifetimeDays     int64 `json:"lifetimeDays"`
	RenewBeforeHours int64 `json:"renewBeforeHours"`
}

type SecretRotation struct {
	ApplicationEntityID string `json:"applicationEntityId"`
	SecretRotationSettings
}

func IsSecretRotationNotFound(err error) bool {
	return isResourceNotFound(err, "terraform_secret_rotation_not_found")
}

func secretRotationPath(id string) (string, error) {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed == uuid.Nil || !strings.EqualFold(parsed.String(), id) {
		return "", fmt.Errorf("invalid managed application UUID")
	}
	return "api/terraform/v1/managed-applications/" + parsed.String() + "/secret-rotation", nil
}

func (c *Client) SecretRotation(ctx context.Context, id string, settings *SecretRotationSettings) (*SecretRotation, error) {
	var result SecretRotation
	path, err := secretRotationPath(id)
	if err != nil {
		return nil, err
	}
	if settings == nil {
		err = c.do(ctx, http.MethodGet, path, nil, &result)
	} else {
		err = c.do(ctx, http.MethodPut, path, settings, &result)
	}
	if err != nil {
		return nil, err
	}
	if result.ApplicationEntityID == "" || !strings.EqualFold(result.ApplicationEntityID, id) || result.LifetimeDays < 1 || result.RenewBeforeHours < 1 {
		return nil, fmt.Errorf("AZExecute did not confirm the application's secret rotation schedule")
	}
	if settings != nil && result.SecretRotationSettings != *settings {
		return nil, fmt.Errorf("AZExecute returned a different rotation schedule; refresh and retry")
	}
	return &result, nil
}

func (c *Client) DisableSecretRotation(ctx context.Context, id string) error {
	path, err := secretRotationPath(id)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}
