package provider

import (
	"context"
	"fmt"
	"time"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func validateApplicationWaitSettings(pollSeconds, timeoutMinutes int64) error {
	if pollSeconds < 1 || pollSeconds > 300 || timeoutMinutes < 1 || timeoutMinutes > 1440 {
		return fmt.Errorf("poll_interval_seconds must be 1-300 and create_timeout_minutes must be 1-1440")
	}
	return nil
}

// Always return the latest known request so failures cannot discard its identity.
func waitForApplicationRequest(ctx context.Context, client *azclient.Client, current *azclient.Application, interval, timeout time.Duration) (*azclient.Application, error) {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for current.Status != "Ready" {
		if current.Status == "NeedsAttention" || current.Status == "Rejected" {
			return current, fmt.Errorf("%s", applicationNotReadyMessage(current))
		}
		tflog.Info(ctx, "Waiting for AZExecute application request", map[string]any{"resource_id": current.ResourceID, "status": current.Status})
		timer := time.NewTimer(interval)
		select {
		case <-waitCtx.Done():
			timer.Stop()
			return current, fmt.Errorf("Stopped waiting for AZExecute request %d (status %s): %w. The request is retained. Refresh Terraform to read its status; waiting does not approve or retry a paused request.", current.RequestID, current.Status, waitCtx.Err())
		case <-timer.C:
		}
		next, err := client.GetApplication(waitCtx, current.ResourceID)
		if err != nil {
			return current, fmt.Errorf("Unable to read provisioning status for AZExecute request %d: %w", current.RequestID, err)
		}
		if err := validateApplicationResponseIdentity(current, next); err != nil {
			return current, err
		}
		current = next
	}
	return current, nil
}
