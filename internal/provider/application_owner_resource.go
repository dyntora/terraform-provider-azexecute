package provider

import (
	"context"
	"fmt"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &applicationOwnerResource{}
var _ resource.ResourceWithConfigure = &applicationOwnerResource{}
var _ resource.ResourceWithImportState = &applicationOwnerResource{}

type applicationOwnerResource struct{ client *azclient.Client }

type applicationOwnerResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	ApplicationResourceID types.String `tfsdk:"application_resource_id"`
	OwnerObjectID         types.String `tfsdk:"owner_object_id"`
}

func NewApplicationOwnerResource() resource.Resource { return &applicationOwnerResource{} }

func (r *applicationOwnerResource) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_application_owner"
}

func (r *applicationOwnerResource) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		Description: "Manages one owner on an AZExecute application request or provisioned application. Do not combine this resource with inline owner_object_ids on the same application.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Composite owner resource ID in application-resource-id/owner-object-id form.",
			},
			"application_resource_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Stable resource UUID exported by azexecute_application or azexecute_application_request.",
			},
			"owner_object_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Microsoft Entra object UUID of the owner.",
			},
		},
	}
}

func (r *applicationOwnerResource) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	r.client = clientFromProviderData(request.ProviderData, &response.Diagnostics)
}

func (r *applicationOwnerResource) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan applicationOwnerResourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}
	resourceID, ownerID, ok := validateOwnerResourceIdentity(plan, &response.Diagnostics)
	if !ok {
		return
	}
	result, err := r.client.AddApplicationOwner(ctx, resourceID, ownerID)
	if err != nil {
		response.Diagnostics.AddError("Unable to add AZExecute application owner", err.Error())
		return
	}
	setApplicationOwnerModel(&plan, result)
	response.Diagnostics.Append(response.State.Set(ctx, &plan)...)
}

func (r *applicationOwnerResource) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state applicationOwnerResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}
	resourceID, ownerID, ok := validateOwnerResourceIdentity(state, &response.Diagnostics)
	if !ok {
		return
	}
	result, err := r.client.GetApplicationOwner(ctx, resourceID, ownerID)
	if azclient.IsNotFound(err) {
		response.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		response.Diagnostics.AddError("Unable to read AZExecute application owner", err.Error())
		return
	}
	setApplicationOwnerModel(&state, result)
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *applicationOwnerResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}

func (r *applicationOwnerResource) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state applicationOwnerResourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || r.client == nil {
		return
	}
	resourceID, ownerID, ok := validateOwnerResourceIdentity(state, &response.Diagnostics)
	if !ok {
		return
	}
	if err := r.client.RemoveApplicationOwner(ctx, resourceID, ownerID); err != nil && !azclient.IsNotFound(err) {
		response.Diagnostics.AddError("Unable to remove AZExecute application owner", err.Error())
	}
}

func (r *applicationOwnerResource) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	resourceID, ownerID, ok := splitOwnerResourceID(request.ID)
	if !ok {
		response.Diagnostics.AddError("Invalid application owner import ID", "Use application-resource-uuid/owner-object-uuid.")
		return
	}
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("id"), ownerResourceID(resourceID, ownerID))...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("application_resource_id"), resourceID)...)
	response.Diagnostics.Append(response.State.SetAttribute(ctx, path.Root("owner_object_id"), ownerID)...)
}

func validateOwnerResourceIdentity(model applicationOwnerResourceModel, diagnostics *diag.Diagnostics) (string, string, bool) {
	resourceID := strings.ToLower(strings.TrimSpace(model.ApplicationResourceID.ValueString()))
	ownerID := strings.ToLower(strings.TrimSpace(model.OwnerObjectID.ValueString()))
	if !uuidPattern.MatchString(resourceID) || !uuidPattern.MatchString(ownerID) {
		diagnostics.AddError("Invalid application owner identity", "application_resource_id and owner_object_id must both be non-empty UUIDs.")
		return "", "", false
	}
	return resourceID, ownerID, true
}

func setApplicationOwnerModel(model *applicationOwnerResourceModel, source *azclient.ApplicationOwner) {
	model.ApplicationResourceID = types.StringValue(strings.ToLower(source.ResourceID))
	model.OwnerObjectID = types.StringValue(strings.ToLower(source.OwnerObjectID))
	model.ID = types.StringValue(ownerResourceID(source.ResourceID, source.OwnerObjectID))
}

func ownerResourceID(resourceID, ownerID string) string {
	return fmt.Sprintf("%s/%s", strings.ToLower(resourceID), strings.ToLower(ownerID))
}

func splitOwnerResourceID(value string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(value), "/")
	if len(parts) != 2 || !uuidPattern.MatchString(parts[0]) || !uuidPattern.MatchString(parts[1]) {
		return "", "", false
	}
	return strings.ToLower(parts[0]), strings.ToLower(parts[1]), true
}
