package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Retrying DELETE resumes the same authorized, leased cleanup. Never retry all
// conflicts: ownership, policy and concurrent edits require user intervention.
func (c *Client) deleteApplication(ctx context.Context, resourceID string, timeout, interval time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	path := "api/terraform/v1/applications/" + url.PathEscape(resourceID)
	var lastPending error
	for {
		err := c.do(waitCtx, http.MethodDelete, path, nil, nil)
		if err == nil || IsDeleteNotFound(err) {
			return err
		}
		if waitCtx.Err() == nil && !isApplicationDeletionPending(err) {
			return err
		}
		if isApplicationDeletionPending(err) {
			lastPending = err
		}
		if waitErr := waitForRetry(waitCtx, interval); waitErr != nil {
			message := fmt.Sprintf("Deletion of AZExecute resource %s was not confirmed before waiting stopped. Terraform state is retained. Review a fresh plan and retry the intended apply or destroy to resume cleanup; do not remove the resource from state or untaint it while deletion is incomplete", resourceID)
			if lastPending != nil {
				return fmt.Errorf("%s. Last response: %v. Waiting stopped: %w", message, lastPending, waitErr)
			}
			return fmt.Errorf("%s: %w", message, waitErr)
		}
	}
}

func isApplicationDeletionPending(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict && apiErr.Service == "" &&
		(apiErr.Code == "directory_deletion_unconfirmed" || apiErr.Code == "terraform_application_operation_in_progress")
}
