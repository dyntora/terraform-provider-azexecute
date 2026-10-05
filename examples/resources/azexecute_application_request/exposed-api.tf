# Requires provider 0.10+ and an API advertising
# supports_registration_requests. Add your tenant-required metadata and owners.
# UUIDs below are stable examples: generate your own once, never on every apply.
resource "azexecute_application_request" "middle_tier_api" {
  display_name           = "Customer API"
  business_justification = "Serve signed-in users and call downstream APIs on their behalf"
  configure_registration = true

  identifier_uris                = ["api://{applicationId}"]
  requested_access_token_version = 2

  exposed_scopes = [{
    id                         = "a1697003-ae63-49e6-9ac4-c952f139442b"
    value                      = "access_as_user"
    admin_consent_display_name = "Access Customer API"
    admin_consent_description  = "Allow this client to access Customer API on behalf of the signed-in user."
    consent_type               = "Admin"
    is_enabled                 = true
  }]

  # Optional: replace with an existing frontend application's client ID.
  # pre_authorized_applications = [{
  #   application_id           = "15a186bd-b911-47b7-a1dd-cd63521e9717"
  #   delegated_permission_ids = ["a1697003-ae63-49e6-9ac4-c952f139442b"]
  # }]

  # Add api_permission_request blocks with grant_type = "DelegatedScope"
  # for downstream delegated permissions.
  # Their approval/consent workflow is separate from publishing this API's scopes.
}

output "middle_tier_request_status" {
  value = azexecute_application_request.middle_tier_api.status
}
