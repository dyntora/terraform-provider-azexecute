package provider

import (
	"fmt"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
)

func applicationNotReadyMessage(application *azclient.Application) string {
	reason := stringPointerValue(application.StatusReason, "No additional reason was supplied.")
	action := "Wait for provisioning to finish, then run Terraform again."
	switch application.Status {
	case "PendingApproval":
		action = "A tenant administrator must review and approve this request before it can be provisioned."
	case "NeedsAttention":
		action = "A tenant administrator must review the request flow and retry provisioning, or use Clean up and retire. After retirement, run a fresh Terraform plan/apply to submit a replacement using the corrected configuration."
	case "Rejected":
		action = "The request cannot be updated in its current state. Ask a tenant administrator to review the decision and request flow. To start over, use Clean up and retire, correct the configuration, and run a fresh Terraform plan/apply."
	}
	return fmt.Sprintf("AZExecute request %d reports status %q. Reason: %s\n\n%s", application.RequestID, application.Status, reason, action)
}
