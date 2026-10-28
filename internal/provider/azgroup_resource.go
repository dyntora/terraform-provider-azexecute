package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &azgroupResource{}
var _ resource.ResourceWithImportState = &azgroupResource{}
var _ resource.ResourceWithValidateConfig = &azgroupResource{}

type azgroupResource struct{ client *azclient.Client }
type azgroupModel struct {
	ID                   types.String `tfsdk:"id"`
	GroupID              types.Int64  `tfsdk:"group_id"`
	Name                 types.String `tfsdk:"name"`
	Description          types.String `tfsdk:"description"`
	Source               types.String `tfsdk:"source"`
	ActualSource         types.String `tfsdk:"actual_source"`
	DisplayName          types.String `tfsdk:"display_name"`
	EntraObjectID        types.String `tfsdk:"entra_object_id"`
	Status               types.String `tfsdk:"status"`
	GroupType            types.String `tfsdk:"group_type"`
	Active               types.Bool   `tfsdk:"active"`
	Color                types.String `tfsdk:"color_hex"`
	Icon                 types.String `tfsdk:"icon"`
	DeleteEntraOnDestroy types.Bool   `tfsdk:"delete_entra_on_destroy"`
}

func NewAZGroupResource() resource.Resource { return &azgroupResource{} }
func (r *azgroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_azgroup"
}
func (r *azgroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manages an AZExecute group using tenant group policy. Local groups are deleted on destroy; Entra groups are retained unless deletion is explicitly enabled. User membership of Entra groups remains managed by directory synchronization.", Attributes: map[string]schema.Attribute{
		"delete_entra_on_destroy": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Explicitly delete the Entra group on destroy. Requires TenantAdmin, enabled Entra management, and a group created by this resource. Imported Entra groups cannot be deleted. Apply this setting before destroying."},
		"id":                      schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Description: "Stable Terraform resource UUID. Import with this UUID, or a numeric AZExecute group ID."},
		"group_id":                schema.Int64Attribute{Computed: true, Description: "AZExecute group ID used by membership resources."},
		"name":                    schema.StringAttribute{Required: true, Description: "Requested group name. Tenant naming rules are applied when creating an Entra group; display_name contains the resolved name."},
		"description":             schema.StringAttribute{Optional: true},
		"source":                  schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("auto"), PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "auto follows tenant policy at creation; local explicitly requests local creation; entra requires enabled Entra creation. Changing this replaces the group."},
		"actual_source":           schema.StringAttribute{Computed: true}, "display_name": schema.StringAttribute{Computed: true}, "entra_object_id": schema.StringAttribute{Computed: true}, "status": schema.StringAttribute{Computed: true},
		"group_type": schema.StringAttribute{Optional: true}, "color_hex": schema.StringAttribute{Optional: true}, "icon": schema.StringAttribute{Optional: true},
		"active": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true), Description: "Local active status. Entra groups must be active; synchronization controls their availability."},
	}}
}
func (r *azgroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}
func validateAZGroup(m azgroupModel) diag.Diagnostics {
	var d diag.Diagnostics
	if !m.Name.IsUnknown() && (strings.TrimSpace(m.Name.ValueString()) == "" || m.Name.ValueString() != strings.TrimSpace(m.Name.ValueString()) || len([]rune(m.Name.ValueString())) > 200 || strings.ContainsFunc(m.Name.ValueString(), unicode.IsControl)) {
		d.AddError("Invalid group name", "Use 1–200 characters without surrounding whitespace or control characters.")
	}
	if !m.Source.IsUnknown() && !m.Source.IsNull() && m.Source.ValueString() != "auto" && m.Source.ValueString() != "local" && m.Source.ValueString() != "entra" {
		d.AddError("Invalid group source", "source must be auto, local, or entra.")
	}
	for label, item := range map[string]struct {
		value types.String
		limit int
	}{"description": {m.Description, 1000}, "group_type": {m.GroupType, 50}, "color_hex": {m.Color, 20}, "icon": {m.Icon, 100}} {
		if !item.value.IsUnknown() && len([]rune(item.value.ValueString())) > item.limit {
			d.AddError("Invalid group metadata", fmt.Sprintf("%s must contain at most %d characters.", label, item.limit))
		}
	}
	if m.Source.ValueString() == "entra" && !m.Active.IsUnknown() && !m.Active.IsNull() && !m.Active.ValueBool() {
		d.AddError("Invalid Entra status", "Entra groups must be active. Synchronization controls their status.")
	}
	return d
}
func (r *azgroupResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m azgroupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(validateAZGroup(m)...)
	}
}
func groupInput(m azgroupModel) azclient.GroupConfiguration {
	source := map[string]string{"auto": "Auto", "local": "Local", "entra": "Entra"}[m.Source.ValueString()]
	return azclient.GroupConfiguration{Name: m.Name.ValueString(), Description: m.Description.ValueStringPointer(), Source: source, GroupType: m.GroupType.ValueStringPointer(), IsActive: m.Active.ValueBool(), ColorHex: m.Color.ValueStringPointer(), Icon: m.Icon.ValueStringPointer(), DeleteEntraOnDestroy: m.DeleteEntraOnDestroy.ValueBool()}
}
func setGroupModel(m *azgroupModel, g *azclient.Group) {
	m.ID = types.StringValue(g.ResourceID)
	m.GroupID = types.Int64Value(g.GroupID)
	m.DisplayName = types.StringValue(g.DisplayName)
	m.ActualSource = types.StringValue(strings.ToLower(g.ActualSource))
	m.Status = types.StringValue(g.Status)
	m.EntraObjectID = types.StringPointerValue(g.EntraObjectID)
	m.Name = types.StringValue(g.Configuration.Name)
	m.Description = types.StringPointerValue(g.Configuration.Description)
	m.Source = types.StringValue(strings.ToLower(g.Configuration.Source))
	m.GroupType = types.StringPointerValue(g.Configuration.GroupType)
	m.Active = types.BoolValue(g.Configuration.IsActive)
	m.Color = types.StringPointerValue(g.Configuration.ColorHex)
	m.Icon = types.StringPointerValue(g.Configuration.Icon)
	m.DeleteEntraOnDestroy = types.BoolValue(g.Configuration.DeleteEntraOnDestroy)
}
func (r *azgroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m azgroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	resp.Diagnostics.Append(validateAZGroup(m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := uuid.NewString()
	result, err := r.client.CreateGroup(ctx, id, groupInput(m))
	if err != nil {
		// Recover a durable pending group without ever issuing a second directory creation.
		if found, readErr := r.client.GetGroup(ctx, id); readErr == nil {
			setGroupModel(&m, found)
			resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
		}
		resp.Diagnostics.AddError("Unable to create group", err.Error()+" Recovery ID: "+id+". If creation is unconfirmed, inspect/import this ID before retrying; do not force replacement of a pending Entra group.")
		return
	}
	setGroupModel(&m, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *azgroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m azgroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	result, err := r.client.GetGroup(ctx, m.ID.ValueString())
	if azclient.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read group", err.Error())
		return
	}
	setGroupModel(&m, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *azgroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m azgroupModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	resp.Diagnostics.Append(validateAZGroup(m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.UpdateGroup(ctx, m.ID.ValueString(), groupInput(m))
	if err != nil {
		resp.Diagnostics.AddError("Unable to update group", err.Error())
		return
	}
	setGroupModel(&m, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *azgroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m azgroupModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	if err := r.client.DeleteGroup(ctx, m.ID.ValueString()); err != nil && !azclient.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to destroy group", err.Error())
		return
	}
	if m.ActualSource.ValueString() == "entra" && !m.DeleteEntraOnDestroy.ValueBool() {
		resp.Diagnostics.AddWarning("Entra group retained", "The Entra group and its AZExecute assignments remain intact. Terraform management has been released.")
	}
}
func (r *azgroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.client == nil {
		resp.Diagnostics.AddError("Provider not configured", "Configure authentication before importing a group.")
		return
	}
	var result *azclient.Group
	var err error
	if _, parseErr := uuid.Parse(req.ID); parseErr == nil {
		result, err = r.client.GetGroup(ctx, req.ID)
	} else if id, parseErr := strconv.ParseInt(req.ID, 10, 32); parseErr == nil && id > 0 {
		result, err = r.client.ImportGroup(ctx, id)
	} else {
		resp.Diagnostics.AddError("Invalid group ID", "Use a positive AZExecute group ID or an existing Terraform resource UUID.")
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to import group", err.Error())
		return
	}
	var m azgroupModel
	setGroupModel(&m, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
