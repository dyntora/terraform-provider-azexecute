package provider

import (
	"context"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Capability validation must never fall back to replacing an existing app.
func validateApplicationDetailsPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse, capabilities *azclient.Capabilities) {
	if request.State.Raw.IsNull() || capabilities.SupportsApplicationDetailsUpdates {
		return
	}
	for _, name := range []string{"display_name", "description"} {
		var before, after types.String
		response.Diagnostics.Append(request.State.GetAttribute(ctx, path.Root(name), &before)...)
		response.Diagnostics.Append(request.Plan.GetAttribute(ctx, path.Root(name), &after)...)
		if !after.IsUnknown() && !before.Equal(after) {
			response.Diagnostics.AddAttributeError(path.Root(name), "AZExecute API upgrade required", "This API does not support in-place display_name or description updates. Upgrade AZExecute before applying this change. The existing application will not be replaced.")
		}
	}
}
