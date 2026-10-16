package provider

import (
	"context"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func importApplicationState(ctx context.Context, api *azclient.Client, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	if !uuidPattern.MatchString(request.ID) {
		response.Diagnostics.AddError("Invalid application import ID", "Use the AZExecute Terraform resource UUID, not an Entra client ID, object ID, or numeric request ID.")
		return
	}
	if api == nil {
		response.Diagnostics.AddError("Unable to import application", "Configure the AZExecute provider before importing an application.")
		return
	}
	app, err := api.GetApplication(ctx, request.ID)
	if err != nil {
		response.Diagnostics.AddError("Unable to import application", err.Error())
		return
	}
	if app.RequestedAPIPermissions == nil {
		response.Diagnostics.AddError("AZExecute API upgrade required for import", "This API does not return the original API-permission request configuration. Upgrade AZExecute before importing so that Terraform can recover it without planning an accidental application replacement.")
		return
	}
	permissionType := permissionRequestObjectType().AttrTypes["permission"].(types.SetType).ElemType
	blocks := make([]permissionRequestModel, 0, len(app.RequestedAPIPermissions))
	for _, item := range app.RequestedAPIPermissions {
		permissions := make([]permissionModel, 0, len(item.Permissions))
		for _, permission := range item.Permissions {
			permissions = append(permissions, permissionModel{ID: types.StringValue(permission.ID),
				DisplayName: stringTypeFromPointer(permission.DisplayName), Value: stringTypeFromPointer(permission.Value),
				RequiresAdminConsent: types.BoolValue(permission.RequiresAdminConsent)})
		}
		permissionSet, diagnostics := types.SetValueFrom(ctx, permissionType, permissions)
		response.Diagnostics.Append(diagnostics...)
		blocks = append(blocks, permissionRequestModel{
			TargetType: types.StringValue(item.TargetType), GrantType: types.StringValue(item.GrantType),
			TargetApplicationEntityID:    stringTypeFromPointer(item.TargetApplicationEntityID),
			TargetExternalAPIAppID:       stringTypeFromPointer(item.TargetExternalAPIAppID),
			TargetExternalAPIDisplayName: stringTypeFromPointer(item.TargetExternalAPIDisplayName),
			Justification:                stringTypeFromPointer(item.Justification), Permissions: permissionSet,
		})
	}
	if response.Diagnostics.HasError() {
		return
	}
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), app.ResourceID)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("api_permission_request"), blocks)...)
}
