package client

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
)

type VercelStepAction struct {
	IntegrationID       int64    `json:"integrationId"`
	Operation           string   `json:"operation"`
	TargetIntegrationID int64    `json:"targetIntegrationId,omitempty"`
	TeamID              string   `json:"teamId,omitempty"`
	ProjectID           string   `json:"projectId,omitempty"`
	Environments        []string `json:"environments,omitempty"`
	VariableName        string   `json:"variableName,omitempty"`
	ValueReference      string   `json:"valueReference,omitempty"`
}

type VercelStepInput struct {
	Enabled       bool             `json:"enabled"`
	StopOnFailure bool             `json:"stopOnFailure"`
	Action        VercelStepAction `json:"action"`
}

type VercelStep struct {
	TaskID int64 `json:"taskId"`
	StepID int64 `json:"stepId"`
	VercelStepInput
}

func (c *Client) VercelStep(ctx context.Context, method string, task, step int64, input *VercelStepInput) (*VercelStep, error) {
	if task < 1 || (step < 1 && method != http.MethodPost) {
		return nil, fmt.Errorf("a positive task and step ID are required")
	}
	path := fmt.Sprintf("api/terraform/v1/tasks/%d/vercel-steps", task)
	if step > 0 {
		path += fmt.Sprintf("/%d", step)
	}
	if method == http.MethodDelete {
		return nil, c.do(ctx, method, path, nil, nil)
	}
	var result VercelStep
	var body any
	if input != nil {
		body = input
	}
	if err := c.doWithRetry(ctx, method, path, body, &result, method != http.MethodPost); err != nil {
		return nil, err
	}
	if result.TaskID != task || result.StepID < 1 || (step > 0 && result.StepID != step) || result.Action.IntegrationID < 1 || !slices.Contains([]string{"update_variable", "rotate_access_token", "redeploy"}, result.Action.Operation) {
		return nil, fmt.Errorf("AZExecute did not confirm the requested Vercel step")
	}
	if input != nil && (input.Enabled != result.Enabled || input.StopOnFailure != result.StopOnFailure || !equalAction(input.Action, result.Action)) {
		return nil, fmt.Errorf("AZExecute returned different settings for step %d/%d; refresh or import before retrying", task, result.StepID)
	}
	return &result, nil
}

func equalAction(a, b VercelStepAction) bool {
	return a.Operation == b.Operation && a.IntegrationID == b.IntegrationID && a.TargetIntegrationID == b.TargetIntegrationID && a.TeamID == b.TeamID && a.ProjectID == b.ProjectID && a.VariableName == b.VariableName && a.ValueReference == b.ValueReference && slices.Equal(a.Environments, b.Environments)
}

type VercelDestination struct {
	IntegrationID     int64  `json:"integrationId"`
	ProjectID         string `json:"projectId"`
	VariableName      string `json:"variableName"`
	Environments      string `json:"environments"`
	TriggerRedeploy   bool   `json:"triggerRedeploy"`
	DeploymentTargets string `json:"deploymentTargets,omitempty"`
}

type VercelDelivery struct {
	ApplicationID string              `json:"applicationEntityId"`
	Destinations  []VercelDestination `json:"destinations"`
}

func (c *Client) VercelDelivery(ctx context.Context, method, id string, destinations []VercelDestination) (*VercelDelivery, error) {
	rotation, err := secretRotationPath(id)
	if err != nil {
		return nil, err
	}
	path := strings.TrimSuffix(rotation, "secret-rotation") + "vercel-delivery"
	if method == http.MethodDelete {
		return nil, c.do(ctx, method, path, nil, nil)
	}
	var result VercelDelivery
	var body any
	if destinations != nil {
		body = destinations
	}
	if err = c.do(ctx, method, path, body, &result); err != nil {
		return nil, err
	}
	if !strings.EqualFold(result.ApplicationID, id) || len(result.Destinations) == 0 || (destinations != nil && !slices.Equal(destinations, result.Destinations)) {
		return nil, fmt.Errorf("AZExecute did not confirm the requested delivery configuration")
	}
	for _, d := range result.Destinations {
		if d.IntegrationID < 1 || d.ProjectID == "" || d.Environments == "" || d.VariableName == "" {
			return nil, fmt.Errorf("AZExecute returned an incomplete Vercel destination")
		}
	}
	return &result, nil
}

func IsVercelStepNotFound(err error) bool {
	return isResourceNotFound(err, "terraform_vercel_step_not_found")
}
func IsVercelDeliveryNotFound(err error) bool {
	return isResourceNotFound(err, "terraform_vercel_delivery_not_found")
}
