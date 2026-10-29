package provider

import (
	"fmt"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Once an identity is in state it may only disappear through a confirmed delete.
// A partial response or a remapped request is an error, never new Terraform state.
func validateApplicationIdentity(previous applicationResourceModel, current *azclient.Application) error {
	if current == nil {
		return fmt.Errorf("AZExecute returned no application; existing identity is retained")
	}
	for _, field := range []struct {
		name   string
		old    types.String
		actual string
	}{
		{"resource ID", previous.ID, current.ResourceID},
		{"application entity ID", previous.ApplicationEntityID, pointerText(current.ApplicationEntityID)},
		{"client ID", previous.ApplicationID, pointerText(current.ApplicationID)},
		{"application object ID", previous.ApplicationObjectID, pointerText(current.ApplicationObjectID)},
	} {
		if !field.old.IsNull() && !field.old.IsUnknown() && field.old.ValueString() != "" && !strings.EqualFold(field.old.ValueString(), field.actual) {
			return fmt.Errorf("AZExecute returned a missing or changed %s for an existing application. Existing Terraform identity is retained; reconcile the API record before retrying. Do not replace the application to resolve this error", field.name)
		}
	}
	if !previous.RequestID.IsNull() && !previous.RequestID.IsUnknown() && previous.RequestID.ValueInt64() > 0 && previous.RequestID.ValueInt64() != current.RequestID {
		return fmt.Errorf("AZExecute returned a missing or changed creation request ID. Existing Terraform identity is retained; reconcile the API record before retrying")
	}
	return nil
}

func pointerText(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func identityValue(previous types.String, actual *string) types.String {
	if actual != nil && !previous.IsNull() && !previous.IsUnknown() && strings.EqualFold(previous.ValueString(), *actual) {
		return previous
	}
	return types.StringPointerValue(actual)
}

func validateApplicationResponseIdentity(previous, current *azclient.Application) error {
	if previous == nil {
		return fmt.Errorf("the previous application response is unavailable")
	}
	return validateApplicationIdentity(applicationResourceModel{ID: types.StringValue(previous.ResourceID), RequestID: types.Int64Value(previous.RequestID),
		ApplicationID: types.StringPointerValue(previous.ApplicationID), ApplicationObjectID: types.StringPointerValue(previous.ApplicationObjectID),
		ApplicationEntityID: types.StringPointerValue(previous.ApplicationEntityID)}, current)
}
