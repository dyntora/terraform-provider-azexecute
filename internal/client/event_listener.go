package client

import (
	"context"
	"net/http"
	"strconv"
)

// EventListener contains configuration and authorization metadata, never runtime credential values.
type EventListener struct {
	ID                     int64                      `json:"id,omitempty"`
	Name                   string                     `json:"name"`
	Description            *string                    `json:"description"`
	IsEnabled              bool                       `json:"isEnabled"`
	ExecutionOrder         int64                      `json:"executionOrder"`
	EventType              string                     `json:"eventType"`
	ActionType             string                     `json:"actionType"`
	ApplicationEntityID    *string                    `json:"applicationEntityId"`
	CredentialType         *string                    `json:"credentialType"`
	AutomationTaskSettings *EventListenerTaskSettings `json:"automationTaskSettings"`
	TopdeskSettings        map[string]any             `json:"topdeskSettings"`
	AuthorizedByUserID     *string                    `json:"authorizedByUserId,omitempty"`
}

type EventListenerTaskSettings struct {
	AutomationTaskID int64                    `json:"automationTaskId"`
	Parameters       []EventListenerParameter `json:"parameters"`
}

type EventListenerParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (c *Client) CreateEventListener(ctx context.Context, request EventListener) (*EventListener, error) {
	// POST has no idempotency key. A retry after an ambiguous response could create
	// duplicate automation, so leave reconciliation to the caller.
	request.ID = 0
	request.AuthorizedByUserID = nil
	var result EventListener
	return &result, c.doWithRetry(ctx, http.MethodPost, "api/v1/EventListeners", request, &result, false)
}

func (c *Client) GetEventListener(ctx context.Context, id int64) (*EventListener, error) {
	var result EventListener
	return &result, c.do(ctx, http.MethodGet, eventListenerPath(id), nil, &result)
}

func (c *Client) UpdateEventListener(ctx context.Context, id int64, request EventListener) (*EventListener, error) {
	request.ID = 0
	request.AuthorizedByUserID = nil
	var result EventListener
	return &result, c.do(ctx, http.MethodPut, eventListenerPath(id), request, &result)
}

func (c *Client) DeleteEventListener(ctx context.Context, id int64) error {
	return c.do(ctx, http.MethodDelete, eventListenerPath(id), nil, nil)
}

func eventListenerPath(id int64) string { return "api/v1/EventListeners/" + strconv.FormatInt(id, 10) }
