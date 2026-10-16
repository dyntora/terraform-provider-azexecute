package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Approval can assign registration defaults that were absent from the submitted
// snapshot. Do not send pending computed values back as desired configuration.
func planPendingRegistration(ctx context.Context, configured applicationResourceModel, response *resource.ModifyPlanResponse) {
	for _, field := range []struct {
		name                string
		configured, unknown attr.Value
	}{
		{"sign_in_audience", configured.SignInAudience, types.StringUnknown()},
		{"is_fallback_public_client", configured.IsFallbackPublicClient, types.BoolUnknown()},
		{"identifier_uris", configured.IdentifierURIs, types.SetUnknown(types.StringType)},
		{"web_home_page_url", configured.WebHomePageURL, types.StringUnknown()},
		{"web_logout_url", configured.WebLogoutURL, types.StringUnknown()},
		{"web_enable_access_token_issuance", configured.WebEnableAccessTokenIssuance, types.BoolUnknown()},
		{"web_enable_id_token_issuance", configured.WebEnableIDTokenIssuance, types.BoolUnknown()},
		{"web_redirect_uris", configured.WebRedirectURIs, types.SetUnknown(types.StringType)},
		{"spa_redirect_uris", configured.SpaRedirectURIs, types.SetUnknown(types.StringType)},
		{"public_client_redirect_uris", configured.PublicClientRedirectURIs, types.SetUnknown(types.StringType)},
		{"requested_access_token_version", configured.RequestedAccessTokenVersion, types.Int64Unknown()},
	} {
		if field.configured.IsNull() {
			response.Diagnostics.Append(response.Plan.SetAttribute(ctx, path.Root(field.name), field.unknown)...)
		}
	}
}
