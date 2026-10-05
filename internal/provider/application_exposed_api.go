package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	azclient "github.com/dyntora/terraform-provider-azexecute/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type scopeModel struct {
	ID                      types.String `tfsdk:"id"`
	Value                   types.String `tfsdk:"value"`
	AdminConsentDisplayName types.String `tfsdk:"admin_consent_display_name"`
	AdminConsentDescription types.String `tfsdk:"admin_consent_description"`
	UserConsentDisplayName  types.String `tfsdk:"user_consent_display_name"`
	UserConsentDescription  types.String `tfsdk:"user_consent_description"`
	ConsentType             types.String `tfsdk:"consent_type"`
	IsEnabled               types.Bool   `tfsdk:"is_enabled"`
}

type authorizedClientModel struct {
	AppID    types.String `tfsdk:"application_id"`
	ScopeIDs types.Set    `tfsdk:"delegated_permission_ids"`
}

func scopeObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{
		"id": types.StringType, "value": types.StringType,
		"admin_consent_display_name": types.StringType, "admin_consent_description": types.StringType,
		"user_consent_display_name": types.StringType, "user_consent_description": types.StringType,
		"consent_type": types.StringType, "is_enabled": types.BoolType,
	}}
}
func clientObjectType() types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{"application_id": types.StringType, "delegated_permission_ids": types.SetType{ElemType: types.StringType}}}
}
func exposedScopesSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{Optional: true, Description: "Authoritative delegated scopes exposed by this application. Omit to preserve existing scopes. Disable and apply before removing a live scope.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"id":                         schema.StringAttribute{Required: true, Description: "Stable non-empty scope UUID."},
		"value":                      schema.StringAttribute{Required: true},
		"admin_consent_display_name": schema.StringAttribute{Required: true},
		"admin_consent_description":  schema.StringAttribute{Required: true},
		"user_consent_display_name":  schema.StringAttribute{Optional: true},
		"user_consent_description":   schema.StringAttribute{Optional: true},
		"consent_type":               schema.StringAttribute{Required: true, Description: "Admin or User. User requires user consent text."},
		"is_enabled":                 schema.BoolAttribute{Required: true},
	}}}
}
func preAuthorizedApplicationsSchema() schema.SetNestedAttribute {
	return schema.SetNestedAttribute{Optional: true, Description: "Authoritative clients pre-authorized for this application's exposed scopes. Included in approval review. Omit to preserve existing clients.", NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
		"application_id":           schema.StringAttribute{Required: true},
		"delegated_permission_ids": schema.SetAttribute{Required: true, ElementType: types.StringType},
	}}}
}
func scopesFromModel(ctx context.Context, value types.Set, existing []azclient.PermissionScopeConfiguration) ([]azclient.PermissionScopeConfiguration, error) {
	if !setIsConfigured(value) {
		return existing, nil
	}
	var models []scopeModel
	if d := value.ElementsAs(ctx, &models, false); d.HasError() {
		return nil, fmt.Errorf("cannot read exposed_scopes: %s", d.Errors()[0].Summary())
	}
	result := make([]azclient.PermissionScopeConfiguration, 0, len(models))
	ids, values := map[string]bool{}, map[string]bool{}
	for _, m := range models {
		id := strings.ToLower(m.ID.ValueString())
		if !validScopeUUID(id) {
			return nil, fmt.Errorf("exposed_scopes id must be a non-empty UUID")
		}
		v := m.Value.ValueString()
		if !trimmedNonempty(v) || !trimmedNonempty(m.AdminConsentDisplayName.ValueString()) || !trimmedNonempty(m.AdminConsentDescription.ValueString()) {
			return nil, fmt.Errorf("exposed_scopes requires a trimmed value and admin consent text")
		}
		if ids[id] || values[strings.ToLower(v)] {
			return nil, fmt.Errorf("exposed_scopes IDs and values must be unique")
		}
		ids[id], values[strings.ToLower(v)] = true, true
		consent := m.ConsentType.ValueString()
		if consent != "Admin" && consent != "User" {
			return nil, fmt.Errorf("consent_type must be Admin or User")
		}
		if consent == "User" && (!trimmedNonempty(m.UserConsentDisplayName.ValueString()) || !trimmedNonempty(m.UserConsentDescription.ValueString())) {
			return nil, fmt.Errorf("User scopes require user_consent_display_name and user_consent_description")
		}
		if consent == "Admin" && (!m.UserConsentDisplayName.IsNull() || !m.UserConsentDescription.IsNull()) {
			return nil, fmt.Errorf("Admin scopes must omit user consent text")
		}
		result = append(result, azclient.PermissionScopeConfiguration{ID: id, Value: v, AdminConsentDisplayName: m.AdminConsentDisplayName.ValueString(), AdminConsentDescription: m.AdminConsentDescription.ValueString(), UserConsentDisplayName: stringPointer(m.UserConsentDisplayName), UserConsentDescription: stringPointer(m.UserConsentDescription), ConsentType: consent, IsEnabled: m.IsEnabled.ValueBool()})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}
func validScopeUUID(id string) bool {
	return uuidPattern.MatchString(id) && id != "00000000-0000-0000-0000-000000000000"
}
func clientsFromModel(ctx context.Context, value types.Set, existing []azclient.PreAuthorizedApplicationConfiguration) ([]azclient.PreAuthorizedApplicationConfiguration, error) {
	if !setIsConfigured(value) {
		return existing, nil
	}
	var models []authorizedClientModel
	if d := value.ElementsAs(ctx, &models, false); d.HasError() {
		return nil, fmt.Errorf("cannot read pre_authorized_applications")
	}
	result := make([]azclient.PreAuthorizedApplicationConfiguration, 0, len(models))
	ids := map[string]bool{}
	for _, m := range models {
		id := strings.ToLower(m.AppID.ValueString())
		if !validScopeUUID(id) || ids[id] {
			return nil, fmt.Errorf("pre_authorized_applications requires unique non-empty application UUIDs")
		}
		ids[id] = true
		var scopes []string
		if d := m.ScopeIDs.ElementsAs(ctx, &scopes, false); d.HasError() || len(scopes) == 0 {
			return nil, fmt.Errorf("pre-authorized clients require at least one delegated_permission_ids UUID")
		}
		seenScopes := map[string]bool{}
		for i, s := range scopes {
			if !validScopeUUID(s) {
				return nil, fmt.Errorf("delegated_permission_ids must contain non-empty UUIDs")
			}
			scopes[i] = strings.ToLower(s)
			if seenScopes[scopes[i]] {
				return nil, fmt.Errorf("delegated_permission_ids must contain unique UUIDs (case-insensitive)")
			}
			seenScopes[scopes[i]] = true
		}
		sort.Strings(scopes)
		result = append(result, azclient.PreAuthorizedApplicationConfiguration{AppID: id, DelegatedPermissionIDs: scopes})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AppID < result[j].AppID })
	return result, nil
}
func mapExposedAPIToModel(ctx context.Context, api azclient.APIConfiguration, target *applicationResourceModel, diagnostics *diag.Diagnostics) {
	if setIsConfigured(target.ExposedScopes) {
		var configured []scopeModel
		diagnostics.Append(target.ExposedScopes.ElementsAs(ctx, &configured, false)...)
		spellings := map[string]string{}
		for _, value := range configured {
			spellings[strings.ToLower(value.ID.ValueString())] = value.ID.ValueString()
		}
		models := make([]scopeModel, 0, len(api.Scopes))
		for _, s := range api.Scopes {
			spelling := strings.ToLower(s.ID)
			if original, ok := spellings[spelling]; ok {
				spelling = original
			}
			models = append(models, scopeModel{ID: types.StringValue(spelling), Value: types.StringValue(s.Value), AdminConsentDisplayName: types.StringValue(s.AdminConsentDisplayName), AdminConsentDescription: types.StringValue(s.AdminConsentDescription), UserConsentDisplayName: stringTypeFromPointer(s.UserConsentDisplayName), UserConsentDescription: stringTypeFromPointer(s.UserConsentDescription), ConsentType: types.StringValue(s.ConsentType), IsEnabled: types.BoolValue(s.IsEnabled)})
		}
		v, d := types.SetValueFrom(ctx, scopeObjectType(), models)
		diagnostics.Append(d...)
		target.ExposedScopes = v
	}
	if setIsConfigured(target.PreAuthorizedApplications) {
		var configured []authorizedClientModel
		diagnostics.Append(target.PreAuthorizedApplications.ElementsAs(ctx, &configured, false)...)
		spellings := map[string]authorizedClientModel{}
		for _, value := range configured {
			spellings[strings.ToLower(value.AppID.ValueString())] = value
		}
		models := make([]authorizedClientModel, 0, len(api.PreAuthorizedApplications))
		for _, c := range api.PreAuthorizedApplications {
			spelling := strings.ToLower(c.AppID)
			scopeIDs := append([]string(nil), c.DelegatedPermissionIDs...)
			if original, ok := spellings[spelling]; ok {
				spelling = original.AppID.ValueString()
				var requested []string
				diagnostics.Append(original.ScopeIDs.ElementsAs(ctx, &requested, false)...)
				for i, id := range scopeIDs {
					for _, wanted := range requested {
						if strings.EqualFold(id, wanted) {
							scopeIDs[i] = wanted
						}
					}
				}
			}
			ids, d := types.SetValueFrom(ctx, types.StringType, scopeIDs)
			diagnostics.Append(d...)
			models = append(models, authorizedClientModel{AppID: types.StringValue(spelling), ScopeIDs: ids})
		}
		v, d := types.SetValueFrom(ctx, clientObjectType(), models)
		diagnostics.Append(d...)
		target.PreAuthorizedApplications = v
	}
}
func exposedAPIDiffers(ctx context.Context, desired applicationResourceModel, observed azclient.APIConfiguration) bool {
	actual := desired
	var d diag.Diagnostics
	mapExposedAPIToModel(ctx, observed, &actual, &d)
	return d.HasError() || (setIsConfigured(desired.ExposedScopes) && !desired.ExposedScopes.Equal(actual.ExposedScopes)) || (setIsConfigured(desired.PreAuthorizedApplications) && !desired.PreAuthorizedApplications.Equal(actual.PreAuthorizedApplications))
}
func resolveIdentifierURIs(uris []azclient.URIValue, id string) []azclient.URIValue {
	result := make([]azclient.URIValue, len(uris))
	for i, u := range uris {
		result[i] = azclient.URIValue{Value: strings.ReplaceAll(u.Value, "{applicationId}", id)}
	}
	return result
}
func identifiersWithTemplates(desired types.Set, actual []string, id string) []string {
	if !setIsConfigured(desired) || id == "" {
		return actual
	}
	result := append([]string(nil), actual...)
	for _, v := range desired.Elements() {
		s, ok := v.(types.String)
		if !ok {
			continue
		}
		template := s.ValueString()
		resolved := strings.ReplaceAll(template, "{applicationId}", id)
		for i, a := range result {
			if a == resolved {
				result[i] = template
			}
		}
	}
	return result
}

func trimmedNonempty(value string) bool { return value != "" && value == strings.TrimSpace(value) }
