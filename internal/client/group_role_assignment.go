package client

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"net/http"
	"strings"
)

type GroupRoleAssignment struct {
	ResourceID string `json:"resourceId"`
	RoleID     string `json:"roleId"`
	GroupID    string `json:"groupId"`
}

func IsGroupRoleAssignmentNotFound(err error) bool {
	return isResourceNotFound(err, "terraform_group_role_assignment_not_found")
}

func (c *Client) GroupRoleAssignment(ctx context.Context, method string, assignment GroupRoleAssignment) error {
	for _, id := range []string{assignment.ResourceID, assignment.RoleID, assignment.GroupID} {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil || !strings.EqualFold(parsed.String(), id) {
			return fmt.Errorf("invalid group role assignment UUID")
		}
	}
	path := "api/terraform/v1/applications/" + assignment.ResourceID + "/app-roles/" + assignment.RoleID + "/groups/" + assignment.GroupID
	if method == http.MethodDelete {
		return c.do(ctx, method, path, nil, nil)
	}
	var result GroupRoleAssignment
	if err := c.do(ctx, method, path, nil, &result); err != nil {
		return err
	}
	if !strings.EqualFold(result.ResourceID, assignment.ResourceID) || !strings.EqualFold(result.RoleID, assignment.RoleID) || !strings.EqualFold(result.GroupID, assignment.GroupID) {
		return fmt.Errorf("AZExecute did not confirm the requested group role assignment")
	}
	return nil
}
