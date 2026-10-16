package client

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// Older APIs erased elevated-permission metadata when the tenant hid its form
// controls. Visibility is not a storage policy. Check before sending a mutation,
// including when applying a saved plan against a changed server or tenant.
func ValidateMetadataPreservation(capabilities *Capabilities, elevated, justification bool) error {
	if capabilities.PreservesHiddenMetadata {
		return nil
	}
	var fields []string
	elevatedVisible := slices.Contains(capabilities.IncludedMetadataFields, "requires_elevated_permissions")
	if elevated && !elevatedVisible {
		fields = append(fields, "requires_elevated_permissions")
	}
	if justification && (!elevatedVisible || !slices.Contains(capabilities.IncludedMetadataFields, "elevated_permissions_justification")) {
		fields = append(fields, "elevated_permissions_justification")
	}
	if len(fields) == 0 {
		return nil
	}
	return fmt.Errorf("this AZExecute API cannot preserve %s while their metadata controls are hidden (or visibility was not advertised).\n\nUpgrade AZExecute to a version that preserves hidden metadata, or enable Requires Elevated Permissions and Elevated Permissions Justification in the tenant Application settings. This operation was stopped before sending its application write. Keep the intended metadata; replacing the application will not fix this server behavior", strings.Join(fields, ", "))
}

func (c *Client) validateMetadataBeforeWrite(ctx context.Context, metadata ApplicationMetadata) error {
	if !metadata.RequiresElevatedPermissions && metadata.ElevatedPermissionsJustification == nil {
		return nil
	}
	capabilities, err := c.Capabilities(ctx)
	if err != nil {
		return fmt.Errorf("unable to check metadata compatibility before writing the application: %w", err)
	}
	return ValidateMetadataPreservation(capabilities, metadata.RequiresElevatedPermissions, metadata.ElevatedPermissionsJustification != nil)
}
