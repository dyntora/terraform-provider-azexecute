package provider

import (
	"context"
	"net/http"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type vercelDeliveryResource struct{ client *azclient.Client }
type vercelDeliveryModel struct {
	Application  types.String `tfsdk:"application_entity_id"`
	Destinations types.List   `tfsdk:"destinations"`
}
type destinationModel struct {
	Integration  types.Int64  `tfsdk:"integration_id"`
	Project      types.String `tfsdk:"project_id"`
	Variable     types.String `tfsdk:"variable_name"`
	Environments types.Set    `tfsdk:"environments"`
	Redeploy     types.Set    `tfsdk:"redeploy_environments"`
}

func NewVercelDeliveryResource() resource.Resource { return &vercelDeliveryResource{} }
func (*vercelDeliveryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application_vercel_delivery"
}
func (*vercelDeliveryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Configure the Vercel destinations for a managed application's rotated client secret.", Attributes: map[string]schema.Attribute{
		"application_entity_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		"destinations": schema.ListNestedAttribute{Required: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"integration_id": schema.Int64Attribute{Required: true}, "project_id": schema.StringAttribute{Required: true}, "variable_name": schema.StringAttribute{Required: true},
			"environments": schema.SetAttribute{Required: true, ElementType: types.StringType}, "redeploy_environments": schema.SetAttribute{Optional: true, ElementType: types.StringType},
		}}},
	}}
}
func (r *vercelDeliveryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}
func (r *vercelDeliveryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.apply(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}
func (r *vercelDeliveryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.apply(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}
func (r *vercelDeliveryResource) apply(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, d *diag.Diagnostics) {
	var m vercelDeliveryModel
	d.Append(plan.Get(ctx, &m)...)
	if d.HasError() {
		return
	}
	var items []destinationModel
	d.Append(m.Destinations.ElementsAs(ctx, &items, false)...)
	if len(items) < 1 || len(items) > 10 {
		d.AddError("Invalid Vercel delivery", "Choose between one and ten project destinations.")
	}
	if d.HasError() {
		return
	}
	var targets []azclient.VercelDestination
	for _, item := range items {
		var envs, deploy []string
		d.Append(item.Environments.ElementsAs(ctx, &envs, false)...)
		if !item.Redeploy.IsNull() {
			d.Append(item.Redeploy.ElementsAs(ctx, &deploy, false)...)
		}
		for _, selection := range [][]string{envs, deploy} {
			for _, id := range selection {
				if !vercelEnvironment.MatchString(id) {
					d.AddError("Invalid environment ID", "Use one built-in name or custom env_ ID per set element.")
				}
			}
		}
		if !item.Redeploy.IsNull() && len(deploy) == 0 {
			d.AddError("Invalid redeployment selection", "Omit redeploy_environments to disable redeployment.")
		}
		targets = append(targets, azclient.VercelDestination{IntegrationID: item.Integration.ValueInt64(), ProjectID: item.Project.ValueString(), VariableName: item.Variable.ValueString(), Environments: strings.Join(envs, ","), TriggerRedeploy: len(deploy) > 0, DeploymentTargets: strings.Join(deploy, ",")})
	}
	if d.HasError() {
		return
	}
	if _, err := r.client.VercelDelivery(ctx, http.MethodPut, m.Application.ValueString(), targets); err != nil {
		d.AddError("Unable to save Vercel delivery", err.Error())
		return
	}
	d.Append(state.Set(ctx, &m)...)
}
func (r *vercelDeliveryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m vercelDeliveryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	result, err := r.client.VercelDelivery(ctx, http.MethodGet, m.Application.ValueString(), nil)
	if azclient.IsVercelDeliveryNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Vercel delivery", err.Error())
		return
	}
	var items []destinationModel
	for _, v := range result.Destinations {
		envs, d := types.SetValueFrom(ctx, types.StringType, strings.Split(v.Environments, ","))
		resp.Diagnostics.Append(d...)
		item := destinationModel{Integration: types.Int64Value(v.IntegrationID), Project: types.StringValue(v.ProjectID), Variable: types.StringValue(v.VariableName), Environments: envs, Redeploy: types.SetNull(types.StringType)}
		if v.TriggerRedeploy {
			item.Redeploy, d = types.SetValueFrom(ctx, types.StringType, strings.Split(v.DeploymentTargets, ","))
			resp.Diagnostics.Append(d...)
		}
		items = append(items, item)
	}
	element := types.ObjectType{AttrTypes: map[string]attr.Type{"integration_id": types.Int64Type, "project_id": types.StringType, "variable_name": types.StringType, "environments": types.SetType{ElemType: types.StringType}, "redeploy_environments": types.SetType{ElemType: types.StringType}}}
	list, d := types.ListValueFrom(ctx, element, items)
	resp.Diagnostics.Append(d...)
	m.Destinations = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *vercelDeliveryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m vercelDeliveryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.VercelDelivery(ctx, http.MethodDelete, m.Application.ValueString(), nil); err != nil && !azclient.IsVercelDeliveryNotFound(err) {
		resp.Diagnostics.AddError("Unable to remove Vercel delivery", err.Error())
	}
}
func (*vercelDeliveryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !uuidPattern.MatchString(req.ID) {
		resp.Diagnostics.AddError("Invalid application ID", "Import using the AZExecute managed application UUID.")
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("application_entity_id"), req, resp)
}
