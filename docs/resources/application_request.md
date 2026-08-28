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

Create and read perform one API operation and do not poll while holding the
Terraform state lock. Run Terraform again after approval or background
provisioning. When status becomes `Ready`, a later apply records the Entra
identifiers and applies configured registration settings.

## Example Usage

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

  configure_registration         = true
  sign_in_audience               = "AzureADMyOrg"
  web_redirect_uris              = ["https://platform.example.com/signin-oidc"]
  web_enable_id_token_issuance   = true
  requested_access_token_version = 2

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

- `display_name` (String) — Microsoft Entra application display name. Must
  contain `1`–`200` characters. Changing it replaces the request/resource.

### Optional Metadata

All metadata fields are optional in Terraform. The tenant's live metadata
policy can require an enabled field during plan and apply.

- `description` (String) — application description, up to `500` characters.
  Changing it replaces the request/resource.
- `business_justification` (String) — business reason, `5`–`1000` characters
  when supplied. Optional and computed because AZExecute can normalize an
  omitted value.
- `technical_requirements` (String) — integrations, dependencies, or technical
  needs, up to `500` characters.
- `intended_audience` (String) — intended users, up to `200` characters.
- `data_access_requirements` (String) — data access and classification, up to
  `500` characters.
- `compliance_notes` (String) — compliance or regulatory context, up to `300`
  characters.
- `expected_go_live_date` (String) — `YYYY-MM-DD` or an RFC 3339 timestamp.
- `project_name` (String) — project, programme, or initiative, up to `100`
  characters.
- `department_owner` (String) — owning department or team, up to `100`
  characters.
- `business_criticality` (Number) — criticality from `1` through `5`. Defaults
  to `3` when omitted.
- `requires_elevated_permissions` (Boolean) — whether the application needs
  elevated permissions. Defaults to `false` when omitted.
- `elevated_permissions_justification` (String) — explanation for elevated
  permissions, up to `500` characters. A tenant can require it when
  `requires_elevated_permissions` is true.
- `environment` (String) — environment label, up to `50` characters.
- `contact_email` (String) — valid contact email, up to `200` characters.
- `contact_phone` (String) — contact telephone number, up to `20` characters.

### Optional Registration Configuration

Registration arguments are used only when `configure_registration = true` and
the tenant enables Terraform registration configuration. Omitted fields retain
the value returned by AZExecute/Entra.

- `configure_registration` (Boolean) — manages the supported registration
  fields. Defaults to `false`.
- `sign_in_audience` (String, Computed) — supported Microsoft Entra audience:
  `AzureADMyOrg`, `AzureADMultipleOrgs`,
  `AzureADandPersonalMicrosoftAccount`, or `PersonalMicrosoftAccount`.
- `is_fallback_public_client` (Boolean, Computed) — enables fallback public
  client behavior.
- `identifier_uris` (Set of String, Computed) — application identifier URIs.
- `web_home_page_url` (String, Computed) — web home page URL.
- `web_logout_url` (String, Computed) — web logout URL.
- `web_enable_access_token_issuance` (Boolean, Computed) — enables implicit-flow
  access-token issuance.
- `web_enable_id_token_issuance` (Boolean, Computed) — enables implicit-flow
  ID-token issuance.
- `web_redirect_uris` (Set of String, Computed) — web redirect URIs.
- `spa_redirect_uris` (Set of String, Computed) — single-page application
  redirect URIs.
- `public_client_redirect_uris` (Set of String, Computed) — mobile and desktop
  public-client redirect URIs.
- `requested_access_token_version` (Number, Computed) — requested access-token
  version, normally `1` or `2`. Personal Microsoft account audiences require
  version `2`.

AZExecute validates redirect URI security, audience/token-version combinations,
identifier URIs, and registration concurrency before writing to Entra.

### Optional API-Permission Requests

- `api_permission_request` (Set of Block) — API permissions requested during
  application creation. The tenant must enable Terraform API-permission
  requests. Changing this set replaces the application request/resource.

Each `api_permission_request` supports:

- `target_type` (String, Required) — `ExternalApi` or `InternalApplication`.
- `target_application_entity_id` (Number) — required for
  `InternalApplication`; this is the numeric AZExecute application entity ID.
- `target_external_api_app_id` (String) — required for `ExternalApi`; this is
  the target API's Microsoft Entra application/client UUID.
- `target_external_api_display_name` (String) — optional display name for an
  external API, up to `255` characters.
- `grant_type` (String, Required) — `AppRole`, `DelegatedScope`, or
  `AuthorizedClient`.
- `justification` (String) — request justification, up to `1000` characters.
- `permission` (Set of Block, Required) — at least one permission. Permission
  IDs must be unique within the request.

Each `permission` supports:

- `id` (String, Required) — non-empty Microsoft Entra permission UUID.
- `display_name` (String) — friendly permission name, up to `255` characters.
- `value` (String) — permission value, up to `255` characters.
- `requires_admin_consent` (Boolean) — records whether admin consent is
  required. Defaults to `false`.

The same target and grant type cannot occur twice. Permission approval follows
the tenant's separate Terraform permission-flow setting.

### Read-Only

- `id` (String) — stable provider-generated resource UUID used for idempotency
  and import.
- `status` (String) — `PendingApproval`, `Provisioning`, `Ready`, or `Rejected`.
- `status_reason` (String) — status or rejection explanation when supplied.
- `request_id` (Number) — numeric AZExecute application request ID.
- `application_entity_id` (Number) — numeric AZExecute application entity ID;
  null until provisioning completes.
- `application_id` (String) — Microsoft Entra application/client ID; null until
  provisioning completes.
- `application_object_id` (String) — Microsoft Entra application object ID;
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
