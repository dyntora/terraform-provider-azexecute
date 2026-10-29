package provider

import (
	"context"
	"fmt"
	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"strconv"
	"strings"
)

type azgroupMembershipResource struct{ client *azclient.Client }
type azgroupMembershipModel struct {
	ID       types.String `tfsdk:"id"`
	GroupID  types.Int64  `tfsdk:"group_id"`
	Kind     types.String `tfsdk:"type"`
	TargetID types.String `tfsdk:"target_id"`
	Role     types.String `tfsdk:"role"`
}

var membershipKinds = map[string]string{"user": "users", "application": "applications", "integration": "integrations", "machine": "machines"}

func NewAZGroupMembershipResource() resource.Resource { return &azgroupMembershipResource{} }
func (r *azgroupMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_azgroup_membership"
}
func (r *azgroupMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manages one group assignment without replacing unrelated memberships. User membership in Entra groups is owned by directory sync. Application assignments require ownership of the group and application; machine assignments require tenant administrator authority. AWS, Vercel, and GitHub assignments require integration-creator or tenant-administrator access; other integration types require tenant-administrator access.", Attributes: map[string]schema.Attribute{
		"id":        schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		"group_id":  schema.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"type":      schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "user, application, integration, or machine."},
		"target_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "User object UUID, AZExecute application entity UUID, integration numeric ID, or AZExecute machine UUID. This is not an Entra application client ID."},
		"role":      schema.StringAttribute{Optional: true, Description: "User memberships only: Owner, Member, or Viewer. Omit for Member. A group must retain an active owner."},
	}}
}
func (r *azgroupMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}
func validateGroupMembership(m azgroupMembershipModel) diag.Diagnostics {
	var d diag.Diagnostics
	if !m.GroupID.IsUnknown() && (m.GroupID.ValueInt64() <= 0 || m.GroupID.ValueInt64() > 2147483647) {
		d.AddError("Invalid group ID", "group_id must be a positive 32-bit AZExecute group ID.")
	}
	if !m.Kind.IsUnknown() && membershipKinds[m.Kind.ValueString()] == "" {
		d.AddError("Invalid membership type", "Use user, application, integration, or machine.")
	}
	if !m.TargetID.IsUnknown() && !m.Kind.IsUnknown() {
		if m.Kind.ValueString() == "integration" {
			id, err := strconv.ParseInt(m.TargetID.ValueString(), 10, 32)
			if err != nil || id <= 0 || strconv.FormatInt(id, 10) != m.TargetID.ValueString() {
				d.AddError("Invalid integration ID", "Use a positive integer integration ID without leading zeros.")
			}
		} else if id, err := uuid.Parse(m.TargetID.ValueString()); err != nil || id == uuid.Nil || !strings.EqualFold(id.String(), m.TargetID.ValueString()) {
			d.AddError("Invalid target ID", "Use a non-empty UUID in hyphenated form.")
		}
	}
	if !m.Role.IsNull() && !m.Role.IsUnknown() {
		if m.Kind.ValueString() != "user" && !m.Kind.IsUnknown() {
			d.AddError("Role is only valid for users", "Omit role for application, integration, and machine assignments.")
		} else if role := m.Role.ValueString(); role != "Owner" && role != "Member" && role != "Viewer" {
			d.AddError("Invalid group role", "Use Owner, Member, or Viewer.")
		}
	}
	return d
}
func (r *azgroupMembershipResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m azgroupMembershipModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(validateGroupMembership(m)...)
	}
}
func membershipRole(m azgroupMembershipModel) *string {
	if m.Kind.ValueString() != "user" {
		return nil
	}
	role := m.Role.ValueString()
	if role == "" {
		role = "Member"
	}
	if role == "Owner" {
		role = "Admin"
	}
	return &role
}
func setMembership(m *azgroupMembershipModel, result *azclient.GroupMembership) {
	m.ID = types.StringValue(fmt.Sprintf("%d/%s/%s", result.GroupID, m.Kind.ValueString(), strings.ToLower(result.TargetID)))
	m.GroupID = types.Int64Value(result.GroupID)
	if !strings.EqualFold(m.TargetID.ValueString(), result.TargetID) {
		m.TargetID = types.StringValue(result.TargetID)
	}
	if result.Role != nil {
		role := *result.Role
		if role == "Admin" {
			role = "Owner"
		}
		if !(m.Role.IsNull() && role == "Member") {
			m.Role = types.StringValue(role)
		}
	} else {
		m.Role = types.StringNull()
	}
}
func (r *azgroupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m azgroupMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	resp.Diagnostics.Append(validateGroupMembership(m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.PutGroupMembership(ctx, m.GroupID.ValueInt64(), membershipKinds[m.Kind.ValueString()], m.TargetID.ValueString(), membershipRole(m))
	if err != nil {
		resp.Diagnostics.AddError("Unable to assign group membership", err.Error())
		return
	}
	setMembership(&m, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *azgroupMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m azgroupMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	resp.Diagnostics.Append(validateGroupMembership(m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.PutGroupMembership(ctx, m.GroupID.ValueInt64(), membershipKinds[m.Kind.ValueString()], m.TargetID.ValueString(), membershipRole(m))
	if err != nil {
		resp.Diagnostics.AddError("Unable to update group membership", err.Error())
		return
	}
	setMembership(&m, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *azgroupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m azgroupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	result, err := r.client.GetGroupMembership(ctx, m.GroupID.ValueInt64(), membershipKinds[m.Kind.ValueString()], m.TargetID.ValueString())
	if azclient.IsGroupMembershipNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read group membership", err.Error())
		return
	}
	setMembership(&m, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *azgroupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m azgroupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	if err := r.client.DeleteGroupMembership(ctx, m.GroupID.ValueInt64(), membershipKinds[m.Kind.ValueString()], m.TargetID.ValueString()); err != nil && !azclient.IsGroupMembershipNotFound(err) {
		resp.Diagnostics.AddError("Unable to remove group membership", err.Error())
	}
}
func (r *azgroupMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 3 {
		resp.Diagnostics.AddError("Invalid membership import ID", "Use group_id/type/target_id, for example 12/user/00000000-0000-0000-0000-000000000001.")
		return
	}
	group, _ := strconv.ParseInt(parts[0], 10, 32)
	m := azgroupMembershipModel{ID: types.StringValue(req.ID), GroupID: types.Int64Value(group), Kind: types.StringValue(parts[1]), TargetID: types.StringValue(parts[2]), Role: types.StringNull()}
	resp.Diagnostics.Append(validateGroupMembership(m)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
