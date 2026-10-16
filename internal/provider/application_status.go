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
		action = "A tenant administrator can review the request flow and retry provisioning. To intentionally start over, remove this resource from configuration and apply, or run terraform destroy, subject to tenant deletion policy; this deletes any application created by this request. Then restore the corrected configuration and apply. Clean up and retire is also available to tenant administrators."
	case "Rejected":
		action = "The request cannot be updated in its current state. Ask a tenant administrator to review the decision. To withdraw it, remove this resource from configuration and apply, or run terraform destroy. Any partial application cleanup requires tenant deletion permission. Then restore the corrected configuration and apply. Clean up and retire is also available to tenant administrators."
	}
	return fmt.Sprintf("AZExecute request %d reports status %q. Reason: %s\n\n%s", application.RequestID, application.Status, reason, action)
}
