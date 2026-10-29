package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func validatePermissionRequestsPlan(ctx context.Context, request resource.ModifyPlanRequest, response *resource.ModifyPlanResponse, capabilities *azclient.Capabilities) {
	if request.State.Raw.IsNull() {
		return
	}
	var pending types.Bool
	response.Diagnostics.Append(request.State.GetAttribute(ctx, path.Root("permission_requests_pending"), &pending)...)
	// Computed state describes a real incomplete operation, not an input switch.
	// Planning its completion ensures Terraform calls Update even after a revert.
	response.Diagnostics.Append(response.Plan.SetAttribute(ctx, path.Root("permission_requests_pending"), false)...)
	if pending.ValueBool() {
		response.Diagnostics.AddWarning("Incomplete permission update will be reconciled", "The previous permission update did not finish. Apply will reconcile the current configuration against the same application. Previously approved access is retained; no application replacement is required.")
		if !capabilities.SupportsPermissionRequestUpdates {
			response.Diagnostics.AddError("AZExecute API upgrade required", "Restore an API supporting permission request updates before resuming this operation.")
		}
	}
	var before, after types.Set
	response.Diagnostics.Append(request.State.GetAttribute(ctx, path.Root("api_permission_request"), &before)...)
	response.Diagnostics.Append(request.Plan.GetAttribute(ctx, path.Root("api_permission_request"), &after)...)
	if response.Diagnostics.HasError() || !fullyKnownSet(after) || before.Equal(after) {
		return
	}
	if !capabilities.SupportsPermissionRequestUpdates {
		response.Diagnostics.AddAttributeError(path.Root("api_permission_request"), "AZExecute API upgrade required", "This API does not support in-place permission request updates. Upgrade AZExecute before applying this change. The application will not be replaced.")
		return
	}
	oldRequests, oldErr := permissionRequestsFromModel(ctx, before)
	newRequests, newErr := permissionRequestsFromModel(ctx, after)
	if oldErr == nil && newErr == nil {
		warnPermissionRemoval(azclient.ApplicationUpdate{APIPermissionRequests: &newRequests, ExpectedAPIPermissionRequests: &oldRequests}, &response.Diagnostics)
	}
}

func canonicalPermissionRequests(requests []azclient.APIPermissionRequest) string {
	// Copy before sorting; API response slices may still be used to populate state.
	copyRequests := append([]azclient.APIPermissionRequest{}, requests...)
	lower := func(p *string) *string {
		if p == nil {
			return nil
		}
		v := strings.ToLower(*p)
		return &v
	}
	for i := range copyRequests {
		r := &copyRequests[i]
		r.TargetType, r.GrantType = strings.ToLower(r.TargetType), strings.ToLower(r.GrantType)
		r.TargetApplicationEntityID, r.TargetExternalAPIAppID = lower(r.TargetApplicationEntityID), lower(r.TargetExternalAPIAppID)
		r.Permissions = append([]azclient.APIPermission{}, r.Permissions...)
		for j := range r.Permissions {
			r.Permissions[j].ID = strings.ToLower(r.Permissions[j].ID)
		}
		sort.Slice(r.Permissions, func(a, b int) bool { return r.Permissions[a].ID < r.Permissions[b].ID })
	}
	sort.Slice(copyRequests, func(a, b int) bool {
		x, _ := json.Marshal(copyRequests[a])
		y, _ := json.Marshal(copyRequests[b])
		return string(x) < string(y)
	})
	data, _ := json.Marshal(copyRequests)
	return string(data)
}

func permissionUpdateFromModel(ctx context.Context, value types.Set, current *azclient.Application, update *azclient.ApplicationUpdate) error {
	if current.ResourceID == "" {
		return nil
	} // Initial registration construction, before creation.
	desired, err := permissionRequestsFromModel(ctx, value)
	if err != nil {
		return err
	}
	if current.RequestedAPIPermissions == nil {
		if current.PermissionRequestsPending {
			return fmt.Errorf("the API reports an unfinished permission update without its configuration; restore a compatible API before retrying")
		}
		// Legacy APIs can still handle unrelated edits and post-create metadata
		// reconciliation. Update validates permission edits against prior state first.
		return nil
	}
	if current.PermissionRequestsPending || canonicalPermissionRequests(desired) != canonicalPermissionRequests(current.RequestedAPIPermissions) {
		update.APIPermissionRequests = &desired
		update.ExpectedAPIPermissionRequests = &current.RequestedAPIPermissions
	}
	return nil
}

func validatePermissionSnapshot(ctx context.Context, desired, previous types.Set, current *azclient.Application) error {
	before, err := permissionRequestsFromModel(ctx, previous)
	if err != nil {
		return err
	}
	after, err := permissionRequestsFromModel(ctx, desired)
	if err != nil {
		return err
	}
	if current.RequestedAPIPermissions == nil {
		if canonicalPermissionRequests(before) != canonicalPermissionRequests(after) {
			return fmt.Errorf("the API did not return the current permission request configuration; upgrade AZExecute before changing permissions. The application will not be replaced")
		}
		return nil
	}
	actual := canonicalPermissionRequests(current.RequestedAPIPermissions)
	if actual != canonicalPermissionRequests(before) && actual != canonicalPermissionRequests(after) {
		return fmt.Errorf("permission requests changed after this Terraform plan was prepared; refresh the plan and retry. The application will not be replaced")
	}
	return nil
}

func mapPermissionRequests(ctx context.Context, source []azclient.APIPermissionRequest, target *types.Set, diagnostics *diag.Diagnostics) {
	if source == nil {
		return
	}
	configured, err := permissionRequestsFromModel(ctx, *target)
	if err == nil && !target.IsUnknown() && canonicalPermissionRequests(configured) == canonicalPermissionRequests(source) {
		return
	}
	blocks := make([]permissionRequestModel, 0, len(source))
	for _, r := range source {
		permissions := make([]permissionModel, 0, len(r.Permissions))
		for _, p := range r.Permissions {
			permissions = append(permissions, permissionModel{ID: types.StringValue(p.ID), DisplayName: stringTypeFromPointer(p.DisplayName), Value: stringTypeFromPointer(p.Value), RequiresAdminConsent: types.BoolValue(p.RequiresAdminConsent)})
		}
		values, d := types.SetValueFrom(ctx, permissionRequestObjectType().AttrTypes["permission"].(types.SetType).ElemType, permissions)
		diagnostics.Append(d...)
		blocks = append(blocks, permissionRequestModel{TargetType: types.StringValue(r.TargetType), GrantType: types.StringValue(r.GrantType), TargetApplicationEntityID: stringTypeFromPointer(r.TargetApplicationEntityID), TargetExternalAPIAppID: stringTypeFromPointer(r.TargetExternalAPIAppID), TargetExternalAPIDisplayName: stringTypeFromPointer(r.TargetExternalAPIDisplayName), Justification: stringTypeFromPointer(r.Justification), Permissions: values})
	}
	values, d := types.SetValueFrom(ctx, permissionRequestObjectType(), blocks)
	diagnostics.Append(d...)
	*target = values
}

func warnPermissionRemoval(update azclient.ApplicationUpdate, diagnostics *diag.Diagnostics) {
	if update.APIPermissionRequests == nil || update.ExpectedAPIPermissionRequests == nil {
		return
	}
	keys := func(requests []azclient.APIPermissionRequest) map[string]bool {
		result := map[string]bool{}
		for _, r := range requests {
			for _, p := range r.Permissions {
				target := ""
				if r.TargetApplicationEntityID != nil {
					target = *r.TargetApplicationEntityID
				}
				if r.TargetExternalAPIAppID != nil {
					target = *r.TargetExternalAPIAppID
				}
				result[strings.ToLower(r.TargetType+":"+target+":"+r.GrantType+":"+p.ID)] = true
			}
		}
		return result
	}
	desired := keys(*update.APIPermissionRequests)
	for key := range keys(*update.ExpectedAPIPermissionRequests) {
		if !desired[key] {
			diagnostics.AddWarning("Existing approved API access is retained", "Removed permission requests are no longer managed by Terraform. Affected pending requests are cancelled, but already approved permissions and consent are not revoked. Use AZExecute consent management to review and revoke access separately. The application and client ID are retained.")
			return
		}
	}
}
