package provider

import (
	"context"
	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type azgroupDataSource struct{ client *azclient.Client }

func NewAZGroupDataSource() datasource.DataSource { return &azgroupDataSource{} }
func (d *azgroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_azgroup"
}
func (d *azgroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}
func (d *azgroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := map[string]schema.Attribute{"group_id": schema.Int64Attribute{Required: true}, "active": schema.BoolAttribute{Computed: true}, "delete_entra_on_destroy": schema.BoolAttribute{Computed: true}}
	for _, name := range []string{"id", "name", "description", "source", "actual_source", "display_name", "entra_object_id", "status", "group_type", "color_hex", "icon"} {
		attrs[name] = schema.StringAttribute{Computed: true}
	}
	resp.Schema = schema.Schema{Description: "Reads a group by its AZExecute numeric ID without adopting it or modifying memberships. Requires group-owner or tenant-administrator access.", Attributes: attrs}
}
func (d *azgroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m azgroupModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || d.client == nil {
		return
	}
	if m.GroupID.ValueInt64() <= 0 || m.GroupID.ValueInt64() > 2147483647 {
		resp.Diagnostics.AddError("Invalid group ID", "Use a positive 32-bit group ID.")
		return
	}
	result, err := d.client.LookupGroup(ctx, m.GroupID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read group", err.Error())
		return
	}
	setGroupModel(&m, result)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

type groupCapabilitiesDataSource struct{ client *azclient.Client }
type groupCapabilitiesModel struct {
	Enabled           types.Bool   `tfsdk:"enabled"`
	CanCreate         types.Bool   `tfsdk:"can_create_groups"`
	CanCreateLocal    types.Bool   `tfsdk:"can_create_local_groups"`
	EntraDefault      types.Bool   `tfsdk:"create_groups_in_entra"`
	UnavailableReason types.String `tfsdk:"entra_unavailable_reason"`
}

func NewGroupCapabilitiesDataSource() datasource.DataSource { return &groupCapabilitiesDataSource{} }
func (d *groupCapabilitiesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_capabilities"
}
func (d *groupCapabilitiesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}
func (d *groupCapabilitiesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Reads the current caller's group creation policy and broker availability. This preflight does not grant permissions; every operation is authorized again by AZExecute.", Attributes: map[string]schema.Attribute{
		"enabled": schema.BoolAttribute{Computed: true}, "can_create_groups": schema.BoolAttribute{Computed: true}, "can_create_local_groups": schema.BoolAttribute{Computed: true}, "create_groups_in_entra": schema.BoolAttribute{Computed: true}, "entra_unavailable_reason": schema.StringAttribute{Computed: true},
	}}
}
func (d *groupCapabilitiesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		return
	}
	result, err := d.client.GroupCapabilities(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read group policy", err.Error())
		return
	}
	m := groupCapabilitiesModel{types.BoolValue(result.Enabled), types.BoolValue(result.CanCreateGroups), types.BoolValue(result.CanCreateLocalGroups), types.BoolValue(result.CreateGroupsInEntra), types.StringPointerValue(result.EntraUnavailableReason)}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
