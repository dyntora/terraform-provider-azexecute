package client

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type GroupConfiguration struct {
	Name                 string  `json:"name"`
	Description          *string `json:"description"`
	Source               string  `json:"source"`
	GroupType            *string `json:"groupType"`
	IsActive             bool    `json:"isActive"`
	ColorHex             *string `json:"colorHex"`
	Icon                 *string `json:"icon"`
	DeleteEntraOnDestroy bool    `json:"deleteEntraOnDestroy"`
}
type Group struct {
	ResourceID    string             `json:"resourceId"`
	GroupID       int64              `json:"groupId"`
	DisplayName   string             `json:"displayName"`
	ActualSource  string             `json:"actualSource"`
	EntraObjectID *string            `json:"entraObjectId"`
	Status        string             `json:"status"`
	Configuration GroupConfiguration `json:"configuration"`
}
type GroupCapabilities struct {
	Enabled                bool    `json:"enabled"`
	CanCreateGroups        bool    `json:"canCreateGroups"`
	CanCreateLocalGroups   bool    `json:"canCreateLocalGroups"`
	CreateGroupsInEntra    bool    `json:"createGroupsInEntra"`
	EntraUnavailableReason *string `json:"entraUnavailableReason"`
}
type GroupMembership struct {
	GroupID  int64   `json:"groupId"`
	TargetID string  `json:"targetId"`
	Role     *string `json:"role"`
}

func (c *Client) GroupCapabilities(ctx context.Context) (*GroupCapabilities, error) {
	var result GroupCapabilities
	return &result, c.do(ctx, http.MethodGet, "api/terraform/v1/groups/capabilities", nil, &result)
}
func groupPath(id string) string { return "api/terraform/v1/groups/" + url.PathEscape(id) }
func (c *Client) CreateGroup(ctx context.Context, id string, input GroupConfiguration) (*Group, error) {
	var result Group
	// The persisted UUID prevents duplicate creation; unconfirmed directory writes are never repeated by the server.
	err := c.do(ctx, http.MethodPost, groupPath(id), input, &result)
	return &result, validateGroupResponse(&result, id, 0, false, err)
}
func (c *Client) GetGroup(ctx context.Context, id string) (*Group, error) {
	var result Group
	err := c.do(ctx, http.MethodGet, groupPath(id), nil, &result)
	return &result, validateGroupResponse(&result, id, 0, false, err)
}
func (c *Client) UpdateGroup(ctx context.Context, id string, input GroupConfiguration) (*Group, error) {
	var result Group
	err := c.do(ctx, http.MethodPut, groupPath(id), input, &result)
	return &result, validateGroupResponse(&result, id, 0, false, err)
}
func (c *Client) DeleteGroup(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, groupPath(id), nil, nil)
}
func (c *Client) ImportGroup(ctx context.Context, id int64) (*Group, error) {
	var result Group
	err := c.do(ctx, http.MethodPost, "api/terraform/v1/groups/import/"+strconv.FormatInt(id, 10), struct{}{}, &result)
	return &result, validateGroupResponse(&result, "", id, false, err)
}
func (c *Client) LookupGroup(ctx context.Context, id int64) (*Group, error) {
	var result Group
	err := c.do(ctx, http.MethodGet, "api/terraform/v1/groups/lookup/"+strconv.FormatInt(id, 10), nil, &result)
	return &result, validateGroupResponse(&result, "", id, true, err)
}
func membershipPath(group int64, kind, target string) string {
	return "api/terraform/v1/group-memberships/" + strconv.FormatInt(group, 10) + "/" + url.PathEscape(kind) + "/" + url.PathEscape(target)
}
func (c *Client) GetGroupMembership(ctx context.Context, group int64, kind, target string) (*GroupMembership, error) {
	var result GroupMembership
	err := c.do(ctx, http.MethodGet, membershipPath(group, kind, target), nil, &result)
	return &result, validateMembershipResponse(&result, group, target, err)
}
func (c *Client) PutGroupMembership(ctx context.Context, group int64, kind, target string, role *string) (*GroupMembership, error) {
	var result GroupMembership
	err := c.do(ctx, http.MethodPut, membershipPath(group, kind, target), struct {
		Role *string `json:"role"`
	}{role}, &result)
	return &result, validateMembershipResponse(&result, group, target, err)
}

func validateGroupResponse(result *Group, resourceID string, groupID int64, lookup bool, err error) error {
	if err != nil {
		return err
	}
	// This value drives a replacement-only Terraform field. An omitted source
	// must be a read error, never drift that schedules a replacement group.
	if result.Configuration.Source != "Auto" && result.Configuration.Source != "Local" && result.Configuration.Source != "Entra" {
		return fmt.Errorf("AZExecute returned an incomplete group source; Terraform state is retained")
	}
	id, parseErr := uuid.Parse(result.ResourceID)
	if parseErr != nil || (!lookup && id == uuid.Nil) || (resourceID != "" && !strings.EqualFold(result.ResourceID, resourceID)) || result.GroupID <= 0 || (groupID > 0 && result.GroupID != groupID) || result.Configuration.Name == "" || (result.ActualSource != "Local" && result.ActualSource != "Entra") || (result.Status != "Ready" && result.Status != "Pending" && result.Status != "Deleting") {
		return fmt.Errorf("AZExecute returned an incomplete or mismatched group response; Terraform state is retained")
	}
	return nil
}
func validateMembershipResponse(result *GroupMembership, group int64, target string, err error) error {
	if err != nil {
		return err
	}
	if result.GroupID != group || !strings.EqualFold(result.TargetID, target) {
		return fmt.Errorf("AZExecute did not confirm the requested group membership; Terraform state is retained")
	}
	return nil
}
func (c *Client) DeleteGroupMembership(ctx context.Context, group int64, kind, target string) error {
	return c.do(ctx, http.MethodDelete, membershipPath(group, kind, target), nil, nil)
}
