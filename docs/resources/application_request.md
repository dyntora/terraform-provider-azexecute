---
page_title: "azexecute_application_request Resource"
description: |-
  Submits and tracks a governed application request without waiting for approval.
---

# azexecute_application_request

Submits a governed AZExecute application request and records its current status.
This is the recommended application resource because it supports both tenant
modes:

- an approval tenant normally returns `PendingApproval`;
- an automatic tenant returns `Provisioning` or `Ready`.

Create submits the request after checking API capabilities; read retrieves its
status. Neither waits for human approval. AZExecute applies requested registration
settings during approved provisioning before reporting `Ready`. Refresh Terraform
after approval to record the generated Entra identifiers.

## Example Usage

### Request app roles and delegated scopes together

Both `app_roles` and `exposed_scopes` belong to the application being requested.
Include them in the same resource to submit them together for approval.
`api_permission_request` separately requests access to another API.

```terraform
resource "azexecute_application_request" "deployment" {
  display_name           = "platform-deployment-production"
  description            = "Deployment identity managed through Terraform"
  business_justification = "Deploys the approved production platform."
  project_name           = "Platform"
  department_owner       = "Engineering"
  environment            = "Production"
  business_criticality   = 4
  contact_email          = "platform@example.com"
  owner_object_ids = [
    "11111111-2222-4333-8444-555555555555",
    "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
  ]

  configure_registration         = true
  sign_in_audience               = "AzureADMyOrg"
  web_redirect_uris              = ["https://platform.example.com/signin-oidc"]
  web_enable_id_token_issuance    = true
  requested_access_token_version = 2
  identifier_uris                = ["api://{applicationId}"]
  app_roles = [{
    id                     = "11111111-2222-4333-8444-555555555555"
    display_name           = "Deployment Reader"
    value                  = "Deployment.Reader"
    description            = "Reads deployment status."
    is_enabled             = true
    allow_users_and_groups = true
    allow_applications     = true
  }]

  exposed_scopes = [{
    id                         = "a1697003-ae63-49e6-9ac4-c952f139442b"
    value                      = "Deployment.Read"
    admin_consent_display_name = "Read deployment status"
    admin_consent_description  = "Allow this client to read deployment status on behalf of the signed-in user."
    consent_type               = "Admin"
    is_enabled                 = true
  }]

  api_permission_request {
    target_type                      = "ExternalApi"
    target_external_api_app_id       = "00000003-0000-0000-c000-000000000000"
    target_external_api_display_name = "Microsoft Graph"
    grant_type                       = "AppRole"
    justification                    = "Reads the approved directory inventory."

    permission {
      id                     = "7ab1d382-f21e-4acd-a863-ba3e13f7da61"
      display_name           = "Directory.Read.All"
      value                  = "Directory.Read.All"
      requires_admin_consent = true
    }
  }
}

output "request_status" {
  value = azexecute_application_request.deployment.status
}

output "application_client_id" {
  value = azexecute_application_request.deployment.application_id
}
```

Only include metadata and operations allowed by the tenant. The provider reads
the live Terraform policy during planning and reports missing tenant-required
metadata before apply.

### Request an API with delegated scopes

This example publishes `access_as_user` under **Expose an API** on the new
Customer API registration. It requires provider `0.10.0` or later, an API
advertising `supports_registration_requests`, and the tenant's Terraform
registration configuration permission. Add the metadata and owners required by
your tenant. App roles are optional; they are not required to expose a scope.

```terraform
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
}

output "middle_tier_request_status" {
  value = azexecute_application_request.middle_tier_api.status
}
```

Use `exposed_scopes = [{ ... }]`, just like `app_roles`; it is a set of objects,
not an `exposed_scopes { ... }` block. Add more objects to publish more scopes.
Generate each scope UUID once and keep it stable in configuration. Do not use
Terraform's `uuid()` function, which would change the identifier on later runs.

The server replaces `{applicationId}` with the new API's client ID. Clients then
request the scope `api://<customer-api-client-id>/access_as_user`. The scope's
`id` is a permission UUID, while `value` is the scope name; neither is the API's
client ID. In `pre_authorized_applications`, `application_id` is the calling
frontend's client ID and `delegated_permission_ids` references the scope UUIDs
defined in this request. Leave that argument out if pre-authorization is not
required. A new frontend must be provisioned before its client ID can be used.

With manual approval, the initial apply succeeds with `PendingApproval` and
stores the proposed scopes for review. Approval starts provisioning; AZExecute
creates the scopes before reporting `Ready`, without a second apply. Refresh
Terraform afterward to update status and generated IDs. A request denied before
provisioning remains `Rejected` with the proposal retained in state.

For an on-behalf-of (OBO) application, these scopes describe how a frontend calls
your API as a signed-in user. Use separate `api_permission_request` blocks with
`grant_type = "DelegatedScope"` for permissions your API needs on downstream
APIs; their approval and consent follow the separate permission workflow. This
configures registrations; your application still implements the OBO token exchange.

See [scope fields and update rules](#exposed-delegated-scopes-and-pre-authorized-clients)
and the [approval workflow guide](../guides/approval-workflows.md).

## Lifecycle

- `PendingApproval` is a successful apply. An administrator must review the
  request in AZExecute.
- `Provisioning` is a successful apply. AZExecute background work is still
  running.
- `Ready` means the application identifiers are available.
- `Rejected` is retained in state, including `status_reason` when available.

Terraform never approves a request. See the
[approval workflow guide](../guides/approval-workflows.md).

## Schema

### Required

- `display_name` (String) â€” Microsoft Entra application display name. Must
  contain `1`â€“`200` characters. Changing it replaces the request/resource.

### Optional Metadata

All metadata fields are optional in Terraform. The tenant's live metadata
policy can require an enabled field during plan and apply.

- `description` (String) â€” application description, up to `500` characters.
  Changing it replaces the request/resource.
- `business_justification` (String) â€” business reason, `5`â€“`1000` characters
  when supplied. Optional and computed because AZExecute can normalize an
  omitted value.
- `technical_requirements` (String) â€” integrations, dependencies, or technical
  needs, up to `500` characters.
- `intended_audience` (String) â€” intended users, up to `200` characters.
- `data_access_requirements` (String) â€” data access and classification, up to
  `500` characters.
- `compliance_notes` (String) â€” compliance or regulatory context, up to `300`
  characters.
- `expected_go_live_date` (String) â€” `YYYY-MM-DD` or an RFC 3339 timestamp.
- `project_name` (String) â€” project, programme, or initiative, up to `100`
  characters.
- `department_owner` (String) â€” owning department or team, up to `100`
  characters.
- `business_criticality` (Number) â€” criticality from `1` through `5`. Defaults
  to `3` when omitted.
- `requires_elevated_permissions` (Boolean) â€” whether the application needs
  elevated permissions. Defaults to `false` when omitted.
- `elevated_permissions_justification` (String) â€” explanation for elevated
  permissions, up to `500` characters. A tenant can require it when
  `requires_elevated_permissions` is true.
- `environment` (String) â€” environment label, up to `50` characters.
- `contact_email` (String) â€” valid contact email, up to `200` characters.
- `contact_phone` (String) â€” contact telephone number, up to `20` characters.

### Authoritative Owners

- `owner_object_ids` (Set of String, Computed) â€” complete desired set of
  Microsoft Entra owner object UUIDs. Apply adds missing owners and removes
  AZExecute-managed owners not present in the set from both Microsoft Entra and
  AZExecute. A refresh exposes manual owner changes made in AZExecute as
  Terraform drift. Set `[]` to remove every customer-managed owner only when the tenant minimum permits it; omit the
  argument to adopt current ownership. Graph-only operational identities used
  by AZExecute are preserved so later app-only updates continue to work.

Registered automation identities for the original requester and current caller
are maintained automatically, repaired during authorized updates, and excluded
from implicit ownership drift. Including them explicitly is supported; they
never count toward the human-owner minimum. Use the matching updated API and
provider for this behavior.

Use directory object IDs rather than names, emails, client IDs, or AZExecute
application entity IDs. New entries must resolve as tenant users. An automation
service principal already registered in AZExecute, such as the calling Azure
DevOps identity, can be retained by object ID.

Alternatively, omit `owner_object_ids` and manage owners independently with
[`azexecute_application_owner`](application_owner.md). Never combine the inline
authoritative set with individual owner resources for the same application.

### Ignoring Selected Drift

Terraform's native lifecycle rules can intentionally leave selected inline
settings under manual control:

```terraform
resource "azexecute_application_request" "example" {
  display_name     = "platform-production"
  owner_object_ids = var.initial_owners

  lifecycle {
    ignore_changes = [
      owner_object_ids,
      business_justification,
      web_redirect_uris,
      app_roles,
      api_permission_request,
    ]
  }
}
```

Ignored attributes are still read and exposed in state, but Terraform does not
plan to restore their configured values. Do not ignore tenant-required metadata
unless another process guarantees it remains valid. To ignore ownership
completely, omit `owner_object_ids` and do not create individual owner
resources.

### Optional Registration Configuration

Registration arguments are used only when `configure_registration = true` and
the tenant enables Terraform registration configuration. Omitted fields retain
the value returned by AZExecute/Entra.

- `configure_registration` (Boolean) â€” manages the supported registration
  fields. Defaults to `false`.
- `sign_in_audience` (String, Computed) â€” supported Microsoft Entra audience:
  `AzureADMyOrg`, `AzureADMultipleOrgs`,
  `AzureADandPersonalMicrosoftAccount`, or `PersonalMicrosoftAccount`.
- `is_fallback_public_client` (Boolean, Computed) â€” enables fallback public
  client behavior.
- `identifier_uris` (Set of String, Computed) â€” application identifier URIs.
- `web_home_page_url` (String, Computed) â€” web home page URL.
- `web_logout_url` (String, Computed) â€” web logout URL.
- `web_enable_access_token_issuance` (Boolean, Computed) â€” enables implicit-flow
  access-token issuance.
- `web_enable_id_token_issuance` (Boolean, Computed) â€” enables implicit-flow
  ID-token issuance.
- `web_redirect_uris` (Set of String, Computed) â€” web redirect URIs.
- `spa_redirect_uris` (Set of String, Computed) â€” single-page application
  redirect URIs.
- `public_client_redirect_uris` (Set of String, Computed) â€” mobile and desktop
  public-client redirect URIs.
- `requested_access_token_version` (Number, Computed) â€” requested access-token
  version, normally `1` or `2`. Personal Microsoft account audiences require
  version `2`.
- `app_roles` (Set of Object) â€” authoritative Microsoft Entra app-role
  definitions. Omit this argument to preserve existing roles without managing
  them. Set `[]` to remove all roles after they have been disabled.

Each `app_roles` object supports:

- `id` (String, Required) â€” stable, non-empty UUID. Never regenerate it for an
  existing role.
- `display_name` (String, Required) â€” role name shown to administrators.
- `value` (String, Required) â€” unique value emitted in the `roles` token claim.
- `description` (String, Required) â€” assignment and consent description.
- `is_enabled` (Boolean, Required) â€” whether the role can be assigned and used.
- `allow_users_and_groups` (Boolean, Required) â€” permits user and group
  assignments.
- `allow_applications` (Boolean, Required) â€” permits application/service
  principal assignments.

At least one allowed-member flag must be true. IDs and values must be unique.
To delete an enabled role, first keep it in `app_roles` with
`is_enabled = false` and apply. Remove the disabled role from the set in a
second apply. Manual edits to a configured role are detected during refresh and
restored on apply.

AZExecute validates redirect URI security, audience/token-version combinations,
identifier URIs, app roles, and registration concurrency before writing to
Entra.

### Exposed Delegated Scopes and Pre-authorized Clients

Requires `configure_registration = true` and tenant registration configuration permission.

See [Request an API with delegated scopes](#request-an-api-with-delegated-scopes)
for a complete creation request, including optional pre-authorized clients.

- `exposed_scopes` (Set of Object) — authoritative scopes published by this API.
  Omit to preserve existing scopes. An explicit `[]` removes disabled scopes.
- `pre_authorized_applications` (Set of Object) — authoritative clients authorized
  for these scopes. Omit to preserve existing clients; `[]` removes them.

Each `exposed_scopes` object has required `id` (stable non-empty UUID), `value`,
`admin_consent_display_name`, `admin_consent_description`, `consent_type` (`Admin`
or `User`), and `is_enabled`. `user_consent_display_name` and
`user_consent_description` are required for `User` and must be omitted for `Admin`.
IDs and values must be unique. Disable a live scope and apply before removing it
in a second apply. Remove any pre-authorization references to deleted scopes too.

For a scope eligible for user consent, an entry in `exposed_scopes` looks like:

```terraform
exposed_scopes = [{
  id                         = "da196c2b-33ab-4c29-80d1-d78638b14d1c"
  value                      = "Profile.Read"
  admin_consent_display_name = "Read user profiles"
  admin_consent_description  = "Allow this client to read profiles on behalf of signed-in users."
  user_consent_display_name  = "Read your profile"
  user_consent_description   = "Allow this client to read your profile."
  consent_type               = "User"
  is_enabled                 = true
}]
```

Tenant consent policy still applies. `consent_type` describes consent to use the
scope; it does not control AZExecute's approval of the application request.
For `Admin` scopes, omit both `user_consent_*` fields entirely, including empty strings.

Each `pre_authorized_applications` object has required `application_id` (client
UUID) and `delegated_permission_ids` (non-empty set of this API's scope UUIDs).
Pre-authorization is security-relevant and is included in the request review.

Registration settings are stored with the creation request and applied by the
server after approval, before `Ready`. A rejected request remains `Rejected`
with its requested configuration in state; it creates no application or scopes.
Changes to an existing pending/rejected Terraform request are not applied through
this resource: finish approval first, or explicitly replace the request after
reviewing the destroy/create plan. A retry or refresh does not approve it.

For a new API, use `identifier_uris = ["api://{applicationId}"]`; the API resolves
this placeholder to the generated client ID during provisioning. If scopes are
requested without identifier URIs, the API generates `api://<client-id>`.
Use existing client IDs for pre-authorizations, or provision and approve the
client request first before requesting an API that depends on its generated ID.

New providers require `supports_registration_requests` from the API before
submitting creation-time configuration. Older providers continue using API v1
and their existing post-creation update behavior. Existing live applications
retain their normal registration update behavior and concurrency checks.

### Optional API-Permission Requests

- `api_permission_request` (Set of Block) â€” API permissions requested during
  application creation. The tenant must enable Terraform API-permission
  requests. Changing this set replaces the application request/resource.

Each `api_permission_request` supports:

- `target_type` (String, Required) â€” `ExternalApi` or `InternalApplication`.
- `target_application_entity_id` (String) â€” AZExecute application entity UUID required for
  `InternalApplication`.
- `target_external_api_app_id` (String) â€” required for `ExternalApi`; this is
  the target API's Microsoft Entra application/client UUID.
- `target_external_api_display_name` (String) â€” optional display name for an
  external API, up to `255` characters.
- `grant_type` (String, Required) â€” `AppRole`, `DelegatedScope`, or
  `AuthorizedClient`.
- `justification` (String) â€” request justification, up to `1000` characters.
- `permission` (Set of Block, Required) â€” at least one permission. Permission
  IDs must be unique within the request.

Each `permission` supports:

- `id` (String, Required) â€” non-empty Microsoft Entra permission UUID.
- `display_name` (String) â€” friendly permission name, up to `255` characters.
- `value` (String) â€” permission value, up to `255` characters.
- `requires_admin_consent` (Boolean) â€” records whether admin consent is
  required. Defaults to `false`.

The same target and grant type cannot occur twice. Permission approval follows
the tenant's separate Terraform permission-flow setting.

### Read-Only

- `id` (String) â€” stable provider-generated resource UUID used for idempotency
  and import.
- `status` (String) â€” `PendingApproval`, `Provisioning`, `NeedsAttention`, `Ready`, or `Rejected`.
- `status_reason` (String) â€” status or rejection explanation when supplied.
- `request_id` (Number) â€” numeric AZExecute application request ID.
- `application_entity_id` (String) â€” AZExecute application entity UUID;
  null until provisioning completes.
- `application_id` (String) â€” Microsoft Entra application/client ID; null until
  provisioning completes.
- `application_object_id` (String) â€” Microsoft Entra application object ID;
  null until provisioning completes.

## Import

Import using the stable AZExecute Terraform resource UUID:

```shell
terraform import azexecute_application_request.example 11111111-2222-4333-8444-555555555555
```

Do not use an Entra client ID, Entra object ID, numeric request ID, or numeric
application entity ID. After import, run `terraform plan` and align the
configuration with the imported state.

## Destroy

Destroy cancels a pending or rejected request without requiring application
deletion. Destroying a provisioned or automatically provisioning application
requires **Allow Application Deletion** in the tenant Terraform settings.

## Move from azexecute_application

Provider `0.5` supports a state-preserving cross-resource move:

```terraform
moved {
  from = azexecute_application.example
  to   = azexecute_application_request.example
}
```

Remove the synchronous resource's wait arguments. See
[Upgrade to 0.5](../guides/migration-v0.5.md).
