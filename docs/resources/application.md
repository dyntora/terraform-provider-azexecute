---
page_title: "azexecute_application Resource"
description: |-
  Creates a governed application and waits for automatic provisioning.
---

# azexecute_application

Creates an AZExecute-governed Microsoft Entra application and waits until
automatic provisioning reaches `Ready`. This synchronous resource is intended
for tenants where **Use App Request Approval Flow** is disabled.

When application approval is enabled, planning fails before create and directs
the caller to
[`azexecute_application_request`](application_request.md). The provider does
not ask the caller to weaken tenant governance and does not hold a Terraform
state lock while waiting for a person.

For new configurations, prefer `azexecute_application_request`. It supports
both approval and automatic tenants without polling.

## Example Usage

```terraform
resource "azexecute_application" "automatic" {
  display_name           = "platform-deployment-production"
  description            = "Deployment identity managed through Terraform"
  business_justification = "Deploys the approved production platform."
  project_name           = "Platform"
  department_owner       = "Engineering"
  environment            = "Production"
  business_criticality   = 4
  contact_email          = "platform@example.com"
  owner_object_ids       = ["11111111-2222-4333-8444-555555555555"]

  configure_registration         = true
  sign_in_audience               = "AzureADMyOrg"
  web_redirect_uris              = ["https://platform.example.com/signin-oidc"]
  web_enable_id_token_issuance   = true
  requested_access_token_version = 2
  app_roles = [{
    id                     = "11111111-2222-4333-8444-555555555555"
    display_name           = "Deployment Reader"
    value                  = "Deployment.Reader"
    description            = "Reads deployment status."
    is_enabled             = true
    allow_users_and_groups = true
    allow_applications     = true
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

  poll_interval_seconds  = 5
  create_timeout_minutes = 60
}

output "application_client_id" {
  value = azexecute_application.automatic.application_id
}
```

Only include metadata and operations allowed by the tenant. The provider reads
the live Terraform policy during planning and reports missing tenant-required
metadata before apply.

## Lifecycle

Create submits an idempotent application request, then polls until AZExecute
reports `Ready`. On success, the Entra application identifiers are stored in
Terraform state and supported registration settings are applied.

- If the tenant requires application approval, planning fails and recommends
  `azexecute_application_request`.
- If automatic provisioning exceeds `create_timeout_minutes`, apply returns an
  error. The stable provider-generated UUID lets the next apply resume the same
  request without creating a duplicate.
- `api_permission_request` changes replace the resource.
- Metadata and supported registration fields update in place after the
  application is ready.

## Schema

### Required

- `display_name` (String) â€” Microsoft Entra application display name. Must
  contain `1`â€“`200` characters. Changing it updates the ready application in place.

### In-place application details

Provider `0.11.7` and an API advertising `supports_application_details_updates`
update `display_name` and `description` on the existing ready application. These
changes preserve the resource UUID, request ID, Entra object ID and client ID.
Removing `description` clears it. The API confirms the Graph update before
returning success; concurrent edits report a conflict requiring a fresh plan.
An older API rejects these edits with an upgrade message instead of replacement.
These edits use the existing Terraform access and metadata rules and do not
require `configure_registration` or permission to delete applications.

The original approval request remains an audit snapshot, including its original
name and description. Its original creation name remains reserved until the
resource is retired; renaming changes the live application's display name.
Pending/rejected requests must complete the existing approval/recovery flow
before live details can be changed. A description edit never approves a request.

### Optional Metadata

All metadata fields are optional in Terraform. The tenant's live metadata
policy can require an enabled field during plan and apply.

- `description` (String) â€” application description, up to `500` characters.
  Changing it updates the ready application in place.
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
never count toward the human-owner minimum. Use provider `0.11.6` or later for
this behavior.

Use directory object IDs rather than names, emails, client IDs, or AZExecute
application entity IDs. New entries must resolve as tenant users. An automation
service principal already registered in AZExecute can be retained by object ID.

Alternatively, omit `owner_object_ids` and manage owners independently with
[`azexecute_application_owner`](application_owner.md). Never combine the inline
authoritative set with individual owner resources for the same application.

Terraform's native `lifecycle.ignore_changes` supports intentionally unmanaged
inline metadata, registration fields including `app_roles`, owner sets, and
permission-request blocks. Ignored values remain readable in state but are not
restored by apply.

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

For examples of scopes included in the initial request, see
[Request an API with delegated scopes](application_request.md#request-an-api-with-delegated-scopes).
The same scope and pre-authorization arguments apply here; use
`azexecute_application_request` when the tenant requires approval.

Requires `configure_registration = true` and tenant registration configuration permission.

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
  requests. Changing this set replaces the resource.

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

### Optional Wait Controls

- `poll_interval_seconds` (Number) â€” delay between automatic-provisioning
  status checks, from `1` through `300`. Defaults to `5`.
- `create_timeout_minutes` (Number) â€” maximum automatic-provisioning wait, from
  `1` through `1440`. Defaults to `60`.

### Read-Only

- `id` (String) â€” stable provider-generated resource UUID used for idempotency
  and import.
- `status` (String) â€” current AZExecute status, normally `Provisioning` or
  `Ready` during a successful automatic flow.
- `status_reason` (String) â€” status explanation when supplied.
- `request_id` (Number) â€” numeric AZExecute application request ID.
- `application_entity_id` (String) â€” AZExecute application entity UUID.
- `application_id` (String) â€” Microsoft Entra application/client ID.
- `application_object_id` (String) â€” Microsoft Entra application object ID.

## Approval Restriction

The resource performs a live capability check during plan and create. If the
tenant uses application approval, the error explains that
`azexecute_application_request` is required. The tenant setting is never
changed by Terraform.

## Timeout and Retry

If automatic provisioning exceeds `create_timeout_minutes`, apply returns an
error but the AZExecute request retains its stable provider-generated UUID.
Running apply again is idempotent: the API returns or resumes the existing
request instead of creating a duplicate.

## Import

Import using the stable AZExecute Terraform resource UUID:

```shell
terraform import azexecute_application.automatic 11111111-2222-4333-8444-555555555555
```

Do not use an Entra client ID, Entra object ID, numeric request ID, or numeric
application entity ID. After import, run `terraform plan` and align the
configuration with the imported state.

## Destroy

Destroying a provisioned or automatically provisioning application requires
**Allow Application Deletion** in the tenant Terraform settings. The API
enforces this policy on every request.

## Move to the Recommended Resource

Provider `0.5` supports a state-preserving cross-resource move:

```terraform
moved {
  from = azexecute_application.automatic
  to   = azexecute_application_request.automatic
}
```

Change the resource type and remove `poll_interval_seconds` and
`create_timeout_minutes`. See
[Upgrade to 0.5](../guides/migration-v0.5.md).
