package provider

import (
	"context"
	"encoding/json"
	"fmt"
	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

type integrationField struct {
	name, wire, description      string
	required, replace, sensitive bool
	limit                        int
}

func integrationFields(kind string) []integrationField {
	fields := []integrationField{
		{"name", "name", "Friendly integration name.", true, false, false, 200},
		{"description", "description", "Optional description.", false, false, false, 1000},
		{"notes", "notes", "Optional notes.", false, false, false, 2000},
	}
	switch kind {
	case "aws":
		fields = append(fields,
			integrationField{"service", "service", "secrets_manager, parameter_store, or route53. Imported legacy integrations retain legacy.", true, true, false, 30},
			integrationField{"account_id", "accountId", "12-digit AWS account ID.", true, true, false, 12},
			integrationField{"role_arn", "roleArn", "IAM role ARN in the selected account, configured to trust AZExecute federation.", true, false, false, 500},
			integrationField{"account_alias", "accountAlias", "Optional friendly AWS account alias.", false, false, false, 200},
			integrationField{"default_region", "defaultRegion", "Default AWS region.", false, false, false, 50},
			integrationField{"external_id", "externalId", "Optional external ID. Omit to preserve the stored value; use an empty string to clear.", false, false, true, 200})
	case "vercel":
		fields = append(fields,
			integrationField{"team_id", "teamId", "Vercel team ID.", true, true, false, 100},
			integrationField{"team_slug", "teamSlug", "Optional Vercel team slug.", false, false, false, 100},
			integrationField{"access_token", "accessToken", "Required on creation. Omit after import to preserve the stored token. Stored in Terraform state when configured.", false, false, true, 500})
	case "github":
		fields = append(fields,
			integrationField{"app_id", "appId", "Numeric GitHub App ID.", true, true, false, 64},
			integrationField{"installation_id", "installationId", "Numeric GitHub App installation ID.", true, true, false, 64},
			integrationField{"private_key_pem", "privateKeyPem", "GitHub App private key PEM. Required on creation. Omit after import to preserve the stored key. Stored in Terraform state when configured.", false, false, true, 32768},
			integrationField{"webhook_secret", "webhookSecret", "Optional webhook secret. Omit to preserve the stored value; use an empty string to clear.", false, false, true, 500})
	}
	return fields
}

type integrationResource struct {
	client *azclient.Client
	kind   string
}

func NewAWSIntegrationResource() resource.Resource    { return &integrationResource{kind: "aws"} }
func NewVercelIntegrationResource() resource.Resource { return &integrationResource{kind: "vercel"} }
func NewGitHubIntegrationResource() resource.Resource { return &integrationResource{kind: "github"} }
func (r *integrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + r.kind + "_integration"
}
func (r *integrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}
func (r *integrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	a := map[string]schema.Attribute{
		"id":             schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Description: "Stable Terraform UUID. Import using this UUID or the numeric AZExecute integration ID."},
		"integration_id": schema.Int64Attribute{Computed: true, Description: "Numeric AZExecute ID for group memberships and application integration settings."},
		"enabled":        schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(true)},
		"global":         schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether the integration is available globally under AZExecute access rules. Defaults to group-scoped access."},
	}
	for _, f := range integrationFields(r.kind) {
		v := schema.StringAttribute{Required: f.required, Optional: !f.required, Sensitive: f.sensitive, Description: f.description}
		if f.replace {
			v.PlanModifiers = []planmodifier.String{stringplanmodifier.RequiresReplace()}
		}
		a[f.name] = v
	}
	resp.Schema = schema.Schema{Description: "Manage a " + r.kind + " integration. Requires the integration creator or TenantAdmin. Credentials are never returned by reads. Destroy requires group memberships to be removed first.", Attributes: a}
}
func integrationString(a map[string]attr.Value, name string) types.String {
	v, ok := a[name].(types.String)
	if !ok {
		return types.StringNull()
	}
	return v
}
func validateIntegration(kind string, a map[string]attr.Value, create bool) diag.Diagnostics {
	var d diag.Diagnostics
	for _, f := range integrationFields(kind) {
		v := integrationString(a, f.name)
		if v.IsUnknown() {
			continue
		}
		s := v.ValueString()
		if (f.required && strings.TrimSpace(s) == "") || len([]rune(s)) > f.limit {
			d.AddError("Invalid integration setting", fmt.Sprintf("%s must %scontain at most %d characters.", f.name, map[bool]string{true: "be non-empty and ", false: ""}[f.required], f.limit))
		}
	}
	name := integrationString(a, "name")
	if !name.IsUnknown() && (name.ValueString() != strings.TrimSpace(name.ValueString()) || strings.ContainsFunc(name.ValueString(), unicode.IsControl)) {
		d.AddError("Invalid integration name", "Do not use surrounding whitespace or control characters.")
	}
	if kind == "aws" {
		account := integrationString(a, "account_id")
		if !account.IsUnknown() && !regexp.MustCompile(`^[0-9]{12}$`).MatchString(account.ValueString()) {
			d.AddError("Invalid AWS account", "account_id must contain exactly 12 digits.")
		}
		service := integrationString(a, "service")
		if !service.IsUnknown() && service.ValueString() != "secrets_manager" && service.ValueString() != "parameter_store" && service.ValueString() != "route53" && service.ValueString() != "legacy" {
			d.AddError("Invalid AWS service", "Choose secrets_manager, parameter_store, or route53.")
		}
		if create && service.ValueString() == "legacy" {
			d.AddError("Legacy service cannot be created", "Choose a specific AWS service. legacy is supported only for imported integrations.")
		}
		role := integrationString(a, "role_arn")
		if !role.IsUnknown() && !account.IsUnknown() && !regexp.MustCompile(`^arn:aws(?:-us-gov|-cn)?:iam::`+regexp.QuoteMeta(account.ValueString())+`:role/[\w+=,.@/-]+$`).MatchString(role.ValueString()) {
			d.AddError("Invalid AWS role", "role_arn must identify an IAM role in account_id.")
		}
	}
	if kind == "github" {
		for _, key := range []string{"app_id", "installation_id"} {
			v := integrationString(a, key)
			if !v.IsUnknown() && !regexp.MustCompile(`^[0-9]+$`).MatchString(v.ValueString()) {
				d.AddError("Invalid GitHub identifier", key+" must contain only digits.")
			}
		}
	}
	if kind == "vercel" {
		v := integrationString(a, "team_id")
		if !v.IsUnknown() && !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(v.ValueString()) {
			d.AddError("Invalid Vercel team", "team_id must contain letters, digits, underscores, or hyphens.")
		}
	}
	secret := map[string]string{"vercel": "access_token", "github": "private_key_pem"}[kind]
	if secret != "" {
		v := integrationString(a, secret)
		if !v.IsUnknown() && (create || !v.IsNull()) && strings.TrimSpace(v.ValueString()) == "" {
			d.AddError("Integration credential required", secret+" is required on creation and cannot be empty. Omit it after import to preserve the stored credential.")
		}
	}
	return d
}
func (r *integrationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(validateIntegration(r.kind, m.Attributes(), false)...)
	}
}
func integrationInput(kind string, a map[string]attr.Value) map[string]any {
	input := map[string]any{}
	for _, f := range integrationFields(kind) {
		v := integrationString(a, f.name)
		if !v.IsNull() {
			input[f.wire] = v.ValueString()
		}
	}
	for _, key := range []string{"enabled", "global"} {
		input[key] = a[key].(types.Bool).ValueBool()
	}
	return input
}
func setIntegration(kind string, m types.Object, result *azclient.Integration) (types.Object, diag.Diagnostics) {
	a := m.Attributes()
	a["id"] = types.StringPointerValue(result.ResourceID)
	a["integration_id"] = types.Int64Value(result.IntegrationID)
	var d diag.Diagnostics
	for _, f := range integrationFields(kind) {
		if f.sensitive {
			continue
		}
		var v *string
		if raw, ok := result.Configuration[f.wire]; ok {
			if err := json.Unmarshal(raw, &v); err != nil {
				d.AddError("Invalid integration response", "Invalid "+f.name+"; Terraform state is retained.")
				continue
			}
		}
		a[f.name] = types.StringPointerValue(v)
	}
	for _, key := range []string{"enabled", "global"} {
		var v bool
		if err := json.Unmarshal(result.Configuration[key], &v); err != nil {
			d.AddError("Invalid integration response", "Invalid "+key)
			continue
		}
		a[key] = types.BoolValue(v)
	}
	resultModel, diags := types.ObjectValue(m.AttributeTypes(context.Background()), a)
	d.Append(diags...)
	return resultModel, d
}

func validateIntegrationIdentity(previous types.Object, current *azclient.Integration) error {
	id := previous.Attributes()["integration_id"].(types.Int64)
	if !id.IsNull() && !id.IsUnknown() && id.ValueInt64() > 0 && id.ValueInt64() != current.IntegrationID {
		return fmt.Errorf("AZExecute returned a changed integration ID for this resource; existing Terraform state is retained. Reconcile the API record before retrying")
	}
	return nil
}
func (r *integrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	resp.Diagnostics.Append(validateIntegration(r.kind, m.Attributes(), true)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := uuid.NewString()
	result, err := r.client.CreateIntegration(ctx, r.kind, id, integrationInput(r.kind, m.Attributes()))
	if err != nil {
		if found, readErr := r.client.GetIntegration(ctx, r.kind, id); readErr == nil {
			recovered, d := setIntegration(r.kind, m, found)
			resp.Diagnostics.Append(d...)
			if !d.HasError() {
				resp.Diagnostics.Append(resp.State.Set(ctx, recovered)...)
			}
		}
		resp.Diagnostics.AddError("Unable to create integration", err.Error()+" Recovery ID: "+id+". Inspect or import this ID before retrying an unconfirmed creation.")
		return
	}
	resultModel, d := setIntegration(r.kind, m, result)
	resp.Diagnostics.Append(d...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, resultModel)...)
	}
}
func (r *integrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	result, err := r.client.GetIntegration(ctx, r.kind, integrationString(m.Attributes(), "id").ValueString())
	if azclient.IsIntegrationNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read integration", err.Error())
		return
	}
	if err := validateIntegrationIdentity(m, result); err != nil {
		resp.Diagnostics.AddError("Integration identity changed unexpectedly", err.Error())
		return
	}
	resultModel, d := setIntegration(r.kind, m, result)
	resp.Diagnostics.Append(d...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, resultModel)...)
	}
}
func (r *integrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m types.Object
	var previous types.Object
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &previous)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	resp.Diagnostics.Append(validateIntegration(r.kind, m.Attributes(), false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := integrationString(previous.Attributes(), "id").ValueString()
	current, err := r.client.GetIntegration(ctx, r.kind, id)
	if err == nil {
		err = validateIntegrationIdentity(previous, current)
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to verify integration identity before update", err.Error())
		return
	}
	result, err := r.client.UpdateIntegration(ctx, r.kind, id, integrationInput(r.kind, m.Attributes()))
	if err != nil {
		resp.Diagnostics.AddError("Unable to update integration", err.Error())
		return
	}
	if err := validateIntegrationIdentity(previous, result); err != nil {
		resp.Diagnostics.AddError("Integration identity changed unexpectedly", err.Error())
		return
	}
	resultModel, d := setIntegration(r.kind, m, result)
	resp.Diagnostics.Append(d...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, resultModel)...)
	}
}
func (r *integrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m types.Object
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || r.client == nil {
		return
	}
	if err := r.client.DeleteIntegration(ctx, r.kind, integrationString(m.Attributes(), "id").ValueString()); err != nil && !azclient.IsIntegrationNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete integration", err.Error())
	}
}
func (r *integrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if r.client == nil {
		return
	}
	var result *azclient.Integration
	var err error
	if n, e := strconv.ParseInt(req.ID, 10, 32); e == nil && n > 0 {
		result, err = r.client.LookupIntegration(ctx, r.kind, n, true)
	} else if id, e := uuid.Parse(req.ID); e == nil && id != uuid.Nil {
		result, err = r.client.GetIntegration(ctx, r.kind, id.String())
	} else {
		resp.Diagnostics.AddError("Invalid integration ID", "Use a positive numeric AZExecute integration ID or a Terraform resource UUID.")
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to import integration", err.Error())
		return
	}
	m := emptyIntegrationObject(ctx, r.kind)
	m, d := setIntegration(r.kind, m, result)
	resp.Diagnostics.Append(d...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}
func emptyIntegrationObject(ctx context.Context, kind string) types.Object {
	t := map[string]attr.Type{"id": types.StringType, "integration_id": types.Int64Type, "enabled": types.BoolType, "global": types.BoolType}
	a := map[string]attr.Value{"id": types.StringNull(), "integration_id": types.Int64Null(), "enabled": types.BoolNull(), "global": types.BoolNull()}
	for _, f := range integrationFields(kind) {
		t[f.name] = types.StringType
		a[f.name] = types.StringNull()
	}
	return types.ObjectValueMust(t, a)
}
