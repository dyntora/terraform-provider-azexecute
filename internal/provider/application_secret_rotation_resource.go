package provider

import (
	"context"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type applicationSecretRotationResource struct{ client *azclient.Client }
type rotationModel struct {
	ApplicationEntityID types.String `tfsdk:"application_entity_id"`
	LifetimeDays        types.Int64  `tfsdk:"lifetime_days"`
	RenewBeforeHours    types.Int64  `tfsdk:"renew_before_hours"`
}

func NewApplicationSecretRotationResource() resource.Resource {
	return &applicationSecretRotationResource{}
}

func (r *applicationSecretRotationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_secret_rotation"
}

func (r *applicationSecretRotationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Enables automatic client secret rotation for one managed application. Destroying this resource disables scheduled rotation and preserves existing credentials.", Attributes: map[string]schema.Attribute{
		"application_entity_id": schema.StringAttribute{Required: true, Description: "AZExecute managed application UUID.", PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"lifetime_days":         schema.Int64Attribute{Required: true, Description: "Lifetime in days for newly issued secrets, subject to tenant policy."},
		"renew_before_hours":    schema.Int64Attribute{Required: true, Description: "Start renewal this many hours before expiry (1–24, less than the lifetime)."},
	}}
}

func (r *applicationSecretRotationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

func (r *applicationSecretRotationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.apply(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}
func (r *applicationSecretRotationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.apply(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *applicationSecretRotationResource) apply(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	var m rotationModel
	diagnostics.Append(plan.Get(ctx, &m)...)
	if diagnostics.HasError() {
		return
	}
	if !uuidPattern.MatchString(m.ApplicationEntityID.ValueString()) {
		diagnostics.AddError("Invalid managed application ID", "Use the application_entity_id of a provisioned AZExecute application.")
		return
	}
	settings := azclient.SecretRotationSettings{LifetimeDays: m.LifetimeDays.ValueInt64(), RenewBeforeHours: m.RenewBeforeHours.ValueInt64()}
	if _, err := r.client.SecretRotation(ctx, m.ApplicationEntityID.ValueString(), &settings); err != nil {
		diagnostics.AddError("Unable to configure application secret rotation", err.Error())
		return
	}
	diagnostics.Append(state.Set(ctx, &m)...)
}

func (r *applicationSecretRotationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m rotationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.SecretRotation(ctx, m.ApplicationEntityID.ValueString(), nil)
	if azclient.IsSecretRotationNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read application secret rotation", err.Error())
		return
	}
	m.LifetimeDays = types.Int64Value(result.LifetimeDays)
	m.RenewBeforeHours = types.Int64Value(result.RenewBeforeHours)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *applicationSecretRotationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m rotationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DisableSecretRotation(ctx, m.ApplicationEntityID.ValueString()); err != nil && !azclient.IsSecretRotationNotFound(err) {
		resp.Diagnostics.AddError("Unable to disable application secret rotation", err.Error())
	}
}

func (r *applicationSecretRotationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id := strings.TrimSpace(req.ID)
	if !uuidPattern.MatchString(id) {
		resp.Diagnostics.AddError("Invalid import ID", "Import using the AZExecute application entity UUID.")
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_entity_id"), id)...)
}
