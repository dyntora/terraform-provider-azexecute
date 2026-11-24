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

type optionalClaimsModel struct {
	AccessToken types.Set `tfsdk:"access_token"`
	IDToken     types.Set `tfsdk:"id_token"`
	SAML2Token  types.Set `tfsdk:"saml2_token"`
}

type optionalClaimModel struct {
	Name                 types.String `tfsdk:"name"`
	Source               types.String `tfsdk:"source"`
	Essential            types.Bool   `tfsdk:"essential"`
	AdditionalProperties types.Set    `tfsdk:"additional_properties"`
}

func optionalClaimsBlock() schema.SingleNestedBlock {
	claim := schema.SetNestedBlock{NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
		"name":                  schema.StringAttribute{Required: true},
		"source":                schema.StringAttribute{Optional: true},
		"essential":             schema.BoolAttribute{Optional: true},
		"additional_properties": schema.SetAttribute{Optional: true, ElementType: types.StringType},
	}}}
	return schema.SingleNestedBlock{Description: "Manage optional claims for all three token types. An empty block clears them.", Blocks: map[string]schema.Block{
		"access_token": claim, "id_token": claim, "saml2_token": claim,
	}}
}

func tokenClaimsFromModel(ctx context.Context, model *optionalClaimsModel) (*azclient.OptionalClaims, error) {
	if model == nil {
		return nil, nil
	}
	claims := &azclient.OptionalClaims{}
	for _, pair := range []struct {
		from types.Set
		to   *[]azclient.OptionalClaim
	}{
		{model.AccessToken, &claims.AccessToken}, {model.IDToken, &claims.IDToken}, {model.SAML2Token, &claims.SAML2Token},
	} {
		*pair.to = []azclient.OptionalClaim{}
		if pair.from.IsNull() {
			continue
		}
		var entries []optionalClaimModel
		if d := pair.from.ElementsAs(ctx, &entries, false); d.HasError() {
			return nil, fmt.Errorf("cannot read optional claims: %s", d.Errors()[0].Detail())
		}
		seen := map[string]bool{}
		for _, entry := range entries {
			name := entry.Name.ValueString()
			if strings.TrimSpace(name) == "" || seen[strings.ToLower(name)] {
				return nil, fmt.Errorf("each optional claim needs a unique, nonempty name within its token type")
			}
			seen[strings.ToLower(name)] = true
			source := stringPointer(entry.Source)
			if source != nil && *source != "user" {
				return nil, fmt.Errorf("optional claim %q source must be user or omitted", name)
			}
			properties := []string{}
			if !entry.AdditionalProperties.IsNull() {
				if d := entry.AdditionalProperties.ElementsAs(ctx, &properties, false); d.HasError() {
					return nil, fmt.Errorf("cannot read additional properties for %q: %s", name, d.Errors()[0].Detail())
				}
			}
			for _, property := range properties {
				if strings.TrimSpace(property) == "" {
					return nil, fmt.Errorf("additional_properties for %q must contain nonempty strings", name)
				}
			}
			sort.Strings(properties)
			*pair.to = append(*pair.to, azclient.OptionalClaim{Name: name, Source: source, Essential: entry.Essential.ValueBool(), AdditionalProperties: properties})
		}
		sort.Slice(*pair.to, func(i, j int) bool { return (*pair.to)[i].Name < (*pair.to)[j].Name })
	}
	return claims, nil
}

func tokenClaimsToModel(ctx context.Context, claims *azclient.OptionalClaims, configured *optionalClaimsModel, diagnostics *diag.Diagnostics) *optionalClaimsModel {
	if claims == nil {
		claims = &azclient.OptionalClaims{}
	}
	return &optionalClaimsModel{
		AccessToken: claimSetFromAPI(ctx, claims.AccessToken, configured.AccessToken, diagnostics),
		IDToken:     claimSetFromAPI(ctx, claims.IDToken, configured.IDToken, diagnostics),
		SAML2Token:  claimSetFromAPI(ctx, claims.SAML2Token, configured.SAML2Token, diagnostics),
	}
}

func claimSetFromAPI(ctx context.Context, claims []azclient.OptionalClaim, configured types.Set, diagnostics *diag.Diagnostics) types.Set {
	if len(claims) == 0 && configured.IsNull() {
		return configured
	}
	var prior []optionalClaimModel
	if !configured.IsNull() && !configured.IsUnknown() {
		diagnostics.Append(configured.ElementsAs(ctx, &prior, false)...)
	}
	byName := map[string]optionalClaimModel{}
	for _, item := range prior {
		byName[item.Name.ValueString()] = item
	}
	entries := make([]optionalClaimModel, 0, len(claims))
	for _, claim := range claims {
		previous := byName[claim.Name]
		item := optionalClaimModel{Name: types.StringValue(claim.Name), Source: types.StringPointerValue(claim.Source), Essential: types.BoolNull(), AdditionalProperties: types.SetNull(types.StringType)}
		if claim.Essential || !previous.Essential.IsNull() {
			item.Essential = types.BoolValue(claim.Essential)
		}
		if len(claim.AdditionalProperties) > 0 || !previous.AdditionalProperties.IsNull() {
			props := append([]string{}, claim.AdditionalProperties...)
			var d diag.Diagnostics
			item.AdditionalProperties, d = types.SetValueFrom(ctx, types.StringType, props)
			diagnostics.Append(d...)
		}
		entries = append(entries, item)
	}
	element := types.ObjectType{AttrTypes: map[string]attr.Type{"name": types.StringType, "source": types.StringType, "essential": types.BoolType, "additional_properties": types.SetType{ElemType: types.StringType}}}
	result, d := types.SetValueFrom(ctx, element, entries)
	diagnostics.Append(d...)
	return result
}

func tokenClaimsMatch(ctx context.Context, desired *optionalClaimsModel, observed *azclient.OptionalClaims) bool {
	var d diag.Diagnostics
	actual := tokenClaimsToModel(ctx, observed, desired, &d)
	return !d.HasError() && actual.AccessToken.Equal(desired.AccessToken) && actual.IDToken.Equal(desired.IDToken) && actual.SAML2Token.Equal(desired.SAML2Token)
}
