package provider

import (
	"context"
	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type awsFederationDataSource struct{ client *azclient.Client }

func NewAWSFederationDataSource() datasource.DataSource { return &awsFederationDataSource{} }
func (d *awsFederationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_aws_federation"
}
func (d *awsFederationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}
func (d *awsFederationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{Description: "Read the public OIDC issuer and audience used by AZExecute to assume AWS IAM roles in this tenant.", Attributes: map[string]schema.Attribute{"provider_url": schema.StringAttribute{Computed: true}, "audience": schema.StringAttribute{Computed: true}}}
}
func (d *awsFederationDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		return
	}
	result, err := d.client.GetAWSFederationSetup(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read AWS federation setup", err.Error())
		return
	}
	m := struct {
		ProviderURL types.String `tfsdk:"provider_url"`
		Audience    types.String `tfsdk:"audience"`
	}{types.StringValue(result.ProviderURL), types.StringValue(result.Audience)}
	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}

type integrationDataSource struct {
	client *azclient.Client
	kind   string
}

func NewAWSIntegrationDataSource() datasource.DataSource { return &integrationDataSource{kind: "aws"} }
func NewVercelIntegrationDataSource() datasource.DataSource {
	return &integrationDataSource{kind: "vercel"}
}
func NewGitHubIntegrationDataSource() datasource.DataSource {
	return &integrationDataSource{kind: "github"}
}
func (d *integrationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + d.kind + "_integration"
}
func (d *integrationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = clientFromProviderData(req.ProviderData, &resp.Diagnostics)
}
func (d *integrationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	a := map[string]schema.Attribute{"integration_id": schema.Int64Attribute{Required: true, Description: "Existing numeric AZExecute integration ID."}, "id": schema.StringAttribute{Computed: true, Description: "Terraform resource UUID, or null if this integration has never been imported."}, "enabled": schema.BoolAttribute{Computed: true}, "global": schema.BoolAttribute{Computed: true}}
	for _, f := range integrationFields(d.kind) {
		if !f.sensitive {
			a[f.name] = schema.StringAttribute{Computed: true, Description: f.description}
		}
	}
	resp.Schema = schema.Schema{Description: "Read an existing " + d.kind + " integration without adopting or changing it. Requires the integration creator or TenantAdmin. No credentials are exposed.", Attributes: a}
}
func (d *integrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m types.Object
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() || d.client == nil {
		return
	}
	id := m.Attributes()["integration_id"].(types.Int64).ValueInt64()
	if id <= 0 || id > 2147483647 {
		resp.Diagnostics.AddError("Invalid integration ID", "Use a positive 32-bit AZExecute integration ID.")
		return
	}
	result, err := d.client.LookupIntegration(ctx, d.kind, id, false)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read integration", err.Error())
		return
	}
	m, diags := setIntegration(d.kind, m, result)
	resp.Diagnostics.Append(diags...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
	}
}
