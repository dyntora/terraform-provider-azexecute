package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type vercelStepResource struct{ client *azclient.Client }
type vercelStepModel struct {
	TaskID       types.Int64  `tfsdk:"task_id"`
	StepID       types.Int64  `tfsdk:"step_id"`
	Integration  types.Int64  `tfsdk:"integration_id"`
	Operation    types.String `tfsdk:"operation"`
	Target       types.Int64  `tfsdk:"target_integration_id"`
	Team         types.String `tfsdk:"team_id"`
	Project      types.String `tfsdk:"project_id"`
	Environments types.Set    `tfsdk:"environments"`
	Variable     types.String `tfsdk:"variable_name"`
	Reference    types.String `tfsdk:"value_reference"`
	Enabled      types.Bool   `tfsdk:"enabled"`
	Stop         types.Bool   `tfsdk:"stop_on_failure"`
}

func NewVercelStepResource() resource.Resource { return &vercelStepResource{} }
func (*vercelStepResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vercel_task_step"
}
func (*vercelStepResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Manage a Vercel step in an existing AZExecute automation task.", Attributes: map[string]schema.Attribute{
		"task_id":               schema.Int64Attribute{Required: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}},
		"step_id":               schema.Int64Attribute{Computed: true},
		"integration_id":        schema.Int64Attribute{Required: true},
		"operation":             schema.StringAttribute{Required: true},
		"target_integration_id": schema.Int64Attribute{Optional: true},
		"team_id":               schema.StringAttribute{Optional: true},
		"project_id":            schema.StringAttribute{Optional: true},
		"environments":          schema.SetAttribute{Optional: true, ElementType: types.StringType},
		"variable_name":         schema.StringAttribute{Optional: true},
		"value_reference":       schema.StringAttribute{Optional: true},
		"enabled":               schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"stop_on_failure":       schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
	}}
}
func (r *vercelStepResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}

var vercelReference = regexp.MustCompile(`^\{\{ [^{}\r\n]+ \}\}$`)
var vercelEnvironment = regexp.MustCompile(`^(production|preview|development|env_[A-Za-z0-9_-]+)$`)

func (m vercelStepModel) input(ctx context.Context, d *diag.Diagnostics) azclient.VercelStepInput {
	var envs []string
	if !m.Environments.IsNull() {
		d.Append(m.Environments.ElementsAs(ctx, &envs, false)...)
	}
	a := azclient.VercelStepAction{IntegrationID: m.Integration.ValueInt64(), Operation: m.Operation.ValueString(), TargetIntegrationID: m.Target.ValueInt64(), TeamID: m.Team.ValueString(), ProjectID: m.Project.ValueString(), Environments: envs, VariableName: m.Variable.ValueString(), ValueReference: m.Reference.ValueString()}
	invalid := func(message string) { d.AddError("Invalid Vercel step", message) }
	if m.TaskID.ValueInt64() < 1 || a.IntegrationID < 1 {
		invalid("Use positive task and integration IDs.")
	}
	for _, env := range envs {
		if !vercelEnvironment.MatchString(env) {
			invalid("Choose production, preview, development, or a custom env_ ID.")
			break
		}
	}
	switch a.Operation {
	case "rotate_access_token":
		if !m.Target.IsNull() && a.TargetIntegrationID < 1 {
			invalid("Use a positive rotation target ID.")
		}
		if !m.Team.IsNull() || !m.Project.IsNull() || !m.Environments.IsNull() || !m.Variable.IsNull() || !m.Reference.IsNull() {
			invalid("Token rotation accepts only the integration and optional rotation target.")
		}
	case "update_variable", "redeploy":
		if !m.Target.IsNull() || a.ProjectID == "" || len(envs) == 0 {
			invalid("Project actions require a project and environment selection. Omit target_integration_id.")
		}
		if a.Operation == "redeploy" && (len(envs) != 1 || envs[0] == "development" || !m.Variable.IsNull() || !m.Reference.IsNull()) {
			invalid("Redeployment requires one hosted environment and no variable settings.")
		}
		if a.Operation == "update_variable" && (a.VariableName == "" || !vercelReference.MatchString(a.ValueReference) || a.ValueReference != "{{ "+strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(a.ValueReference, "{{"), "}}"))+" }}") {
			invalid("Supply a variable name and a task reference such as {{ NewSecret }}.")
		}
	default:
		invalid("Choose update_variable, redeploy, or rotate_access_token.")
	}
	return azclient.VercelStepInput{Enabled: m.Enabled.ValueBool(), StopOnFailure: m.Stop.ValueBool(), Action: a}
}

func (r *vercelStepResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.apply(ctx, req.Plan, &resp.State, 0, &resp.Diagnostics)
}
func (r *vercelStepResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var old vercelStepModel
	resp.Diagnostics.Append(req.State.Get(ctx, &old)...)
	if !resp.Diagnostics.HasError() {
		r.apply(ctx, req.Plan, &resp.State, old.StepID.ValueInt64(), &resp.Diagnostics)
	}
}
func (r *vercelStepResource) apply(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, id int64, d *diag.Diagnostics) {
	var m vercelStepModel
	d.Append(plan.Get(ctx, &m)...)
	if d.HasError() {
		return
	}
	input := m.input(ctx, d)
	if d.HasError() {
		return
	}
	method := http.MethodPut
	if id == 0 {
		method = http.MethodPost
	}
	result, err := r.client.VercelStep(ctx, method, m.TaskID.ValueInt64(), id, &input)
	if err != nil {
		message := err.Error()
		if id == 0 {
			message += ". If creation was interrupted, check the task and import the created step before retrying."
		}
		d.AddError("Unable to save Vercel task step", message)
		return
	}
	m.StepID = types.Int64Value(result.StepID)
	d.Append(state.Set(ctx, &m)...)
}
func vercelOptionalString(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
func (r *vercelStepResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m vercelStepModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	s, err := r.client.VercelStep(ctx, http.MethodGet, m.TaskID.ValueInt64(), m.StepID.ValueInt64(), nil)
	if azclient.IsVercelStepNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Vercel task step", err.Error())
		return
	}
	a := s.Action
	m.Integration = types.Int64Value(a.IntegrationID)
	m.Operation = types.StringValue(a.Operation)
	m.Target = types.Int64Null()
	if a.TargetIntegrationID != 0 {
		m.Target = types.Int64Value(a.TargetIntegrationID)
	}
	m.Team = vercelOptionalString(a.TeamID)
	m.Project = vercelOptionalString(a.ProjectID)
	m.Variable = vercelOptionalString(a.VariableName)
	m.Reference = vercelOptionalString(a.ValueReference)
	m.Environments = types.SetNull(types.StringType)
	if len(a.Environments) > 0 {
		set, d := types.SetValueFrom(ctx, types.StringType, a.Environments)
		resp.Diagnostics.Append(d...)
		m.Environments = set
	}
	m.Enabled = types.BoolValue(s.Enabled)
	m.Stop = types.BoolValue(s.StopOnFailure)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
func (r *vercelStepResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m vercelStepModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if _, err := r.client.VercelStep(ctx, http.MethodDelete, m.TaskID.ValueInt64(), m.StepID.ValueInt64(), nil); err != nil && !azclient.IsVercelStepNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete Vercel task step", err.Error())
	}
}
func (*vercelStepResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", "Use task_id/step_id.")
		return
	}
	for i, name := range []string{"task_id", "step_id"} {
		id, err := strconv.ParseInt(parts[i], 10, 32)
		if err != nil || id <= 0 {
			resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("Use a positive %s.", name))
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), id)...)
	}
}
