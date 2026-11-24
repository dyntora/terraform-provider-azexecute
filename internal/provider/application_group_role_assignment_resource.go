package provider

import (
	"context"
	"net/http"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type applicationGroupRoleAssignmentResource struct{ client *azclient.Client }
type groupRoleAssignmentModel struct {
	ResourceID types.String `tfsdk:"application_resource_id"`
	RoleID     types.String `tfsdk:"app_role_id"`
	GroupID    types.String `tfsdk:"group_object_id"`
}

func (m groupRoleAssignmentModel) assignment() azclient.GroupRoleAssignment {
	return azclient.GroupRoleAssignment{ResourceID: m.ResourceID.ValueString(), RoleID: m.RoleID.ValueString(), GroupID: m.GroupID.ValueString()}
}

func validAssignmentUUID(id string) bool {
	return uuidPattern.MatchString(id) && id != "00000000-0000-0000-0000-000000000000"
}

func NewApplicationGroupRoleAssignmentResource() resource.Resource {
	return &applicationGroupRoleAssignmentResource{}
}
func (r *applicationGroupRoleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_group_role_assignment"
}
func (r *applicationGroupRoleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	id := func(description string) schema.StringAttribute {
		return schema.StringAttribute{Description: description, Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}}
	}
	resp.Schema = schema.Schema{Description: "Assigns one Entra group to one app role on a provisioned AZExecute application.", Attributes: map[string]schema.Attribute{
		"application_resource_id": id("Stable Terraform resource UUID of the AZExecute application request."),
		"app_role_id":             id("UUID of the enabled app role that allows users and groups."),
		"group_object_id":         id("Microsoft Entra group object UUID."),
	}}
}
func (r *applicationGroupRoleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}
func (r *applicationGroupRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m groupRoleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.GroupRoleAssignment(ctx, http.MethodPut, m.assignment()); err != nil {
		resp.Diagnostics.AddError("Unable to assign group to app role", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *applicationGroupRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m groupRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.GroupRoleAssignment(ctx, http.MethodGet, m.assignment())
	if azclient.IsGroupRoleAssignmentNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read group role assignment", err.Error())
	}
}
func (r *applicationGroupRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m groupRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.GroupRoleAssignment(ctx, http.MethodDelete, m.assignment()); err != nil && !azclient.IsGroupRoleAssignmentNotFound(err) {
		resp.Diagnostics.AddError("Unable to remove group role assignment", err.Error())
	}
}
func (r *applicationGroupRoleAssignmentResource) Update(context.Context, resource.UpdateRequest, *resource.UpdateResponse) {
}
func (r *applicationGroupRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 || !validAssignmentUUID(parts[0]) || !validAssignmentUUID(parts[1]) || !validAssignmentUUID(parts[2]) {
		resp.Diagnostics.AddError("Invalid import ID", "Use application-resource-uuid/app-role-uuid/group-object-uuid.")
		return
	}
	for i, name := range []string{"application_resource_id", "app_role_id", "group_object_id"} {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), parts[i])...)
	}
}
