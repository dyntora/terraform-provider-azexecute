---
page_title: "Provider: AZExecute"
description: |-
  Creates and manages governed Microsoft Entra application registrations through AZExecute.
---

# AZExecute Provider

The AZExecute provider manages the deliberately limited application-registration
surface exposed by the AZExecute Terraform API. It creates governed application
requests, tracks approval and provisioning, updates supported metadata and
registration settings, creates API-permission requests, and reads the tenant's
live Terraform policy.

Terraform does not bypass AZExecute governance. The caller keeps its assigned
AZExecute `User`, `Operator`, or `TenantAdmin` role, and every operation is
checked against the current tenant settings.

## Choose a Resource

- Use `azexecute_application_request` for the normal and recommended workflow.
  It supports both approval-based and automatic tenants and never waits for a
  human approval while holding the Terraform state lock.
- Use `azexecute_application` only when the tenant always provisions Terraform
  applications automatically and a single blocking apply is preferred.

See the [approval workflow guide](guides/approval-workflows.md) for the complete
lifecycle and the [getting started guide](guides/getting-started.md) for a
minimal working configuration.

## Example Usage

```terraform
terraform {
  required_version = ">= 1.8"

  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.5"
    }
  }
}

provider "azexecute" {
  tenant_id = var.tenant_id
  client_id = var.client_id
  # Authentication material should normally be provided through environment
  # variables. The default API endpoint and scope need no configuration.
}

resource "azexecute_application_request" "example" {
  display_name = "platform-deployment-production"
  description  = "Deployment identity managed through Terraform"
}

output "request_status" {
  value = azexecute_application_request.example.status
}

output "application_client_id" {
  value = azexecute_application_request.example.application_id
}
```

## Tenant Prerequisites

Before running Terraform, a tenant administrator must:

1. Enable the Terraform API under **Tenant administration → Application
   Configuration → Terraform**.
2. Enable application creation.
3. Enable registration configuration and API-permission requests only when the
   Terraform configuration uses those features.
4. Decide whether application and permission requests use their approval flows.
5. Assign the calling user or service principal an AZExecute enterprise
   application role.

`User` can manage only Terraform resources created by that identity.
`Operator` and `TenantAdmin` can manage Terraform resources across the tenant.
Application deletion is required only when destroying a provisioned
application; cancelling a pending or rejected request does not require it.

## Authentication

The provider tries credentials in this order:

1. pre-acquired access token;
2. OIDC workload identity federation;
3. client certificate;
4. client secret;
5. explicitly selected managed identity;
6. Azure `DefaultAzureCredential`.

The identity belongs to the customer tenant. Tokens are requested for
`https://api.azexecute.com/.default` unless `scope` is overridden. Use the
[authentication guide](guides/authentication.md) for Azure DevOps, GitHub
Actions, certificates, managed identity, and local development.

## Provider Schema

All provider arguments are optional because credentials can be supplied through
environment variables or Azure's default credential chain.

### Optional

- `endpoint` (String) — AZExecute API base URL. Uses `AZEXECUTE_ENDPOINT`, then
  `https://api.azexecute.com`. Non-HTTPS endpoints are accepted only for
  `localhost` development.
- `tenant_id` (String) — customer Microsoft Entra tenant ID. Uses
  `AZEXECUTE_TENANT_ID`, `ARM_TENANT_ID`, or `AZURE_TENANT_ID`.
- `client_id` (String) — automation application client ID or user-assigned
  managed-identity client ID. Uses `AZEXECUTE_CLIENT_ID`, `ARM_CLIENT_ID`, or
  `AZURE_CLIENT_ID`.
- `client_secret` (String, Sensitive) — client-secret fallback. Uses
  `AZEXECUTE_CLIENT_SECRET`, `ARM_CLIENT_SECRET`, or `AZURE_CLIENT_SECRET`.
- `client_certificate_path` (String) — path to a PEM or PKCS#12/PFX certificate
  containing its private key. Uses `AZEXECUTE_CLIENT_CERTIFICATE_PATH` or
  `ARM_CLIENT_CERTIFICATE_PATH`.
- `client_certificate_password` (String, Sensitive) — password for an encrypted
  PKCS#12/PFX file. Uses `AZEXECUTE_CLIENT_CERTIFICATE_PASSWORD` or
  `ARM_CLIENT_CERTIFICATE_PASSWORD`.
- `send_certificate_chain` (Boolean) — sends the certificate chain for
  subject-name/issuer authentication. Defaults to `false`.
- `access_token` (String, Sensitive) — pre-acquired AZExecute bearer token. Uses
  `AZEXECUTE_ACCESS_TOKEN`.
- `use_oidc` (Boolean) — enables workload identity federation. It is also
  enabled by `AZEXECUTE_USE_OIDC`, `ARM_USE_OIDC`, an OIDC token or file, or the
  GitHub Actions OIDC environment.
- `oidc_token` (String, Sensitive) — short-lived federated assertion. Uses
  `AZEXECUTE_OIDC_TOKEN` or `ARM_OIDC_TOKEN`.
- `oidc_token_file_path` (String) — rotating assertion file. Uses
  `AZEXECUTE_OIDC_TOKEN_FILE_PATH`, `ARM_OIDC_TOKEN_FILE_PATH`, or
  `AZURE_FEDERATED_TOKEN_FILE`.
- `oidc_audience` (String) — audience requested from GitHub's OIDC endpoint.
  Uses `AZEXECUTE_OIDC_AUDIENCE` or `ARM_OIDC_AUDIENCE`; defaults to
  `api://AzureADTokenExchange`.
- `use_managed_identity` (Boolean) — explicitly uses Azure managed identity.
  Uses `AZEXECUTE_USE_MANAGED_IDENTITY` or `ARM_USE_MSI`.
- `scope` (String) — OAuth scope. Uses `AZEXECUTE_SCOPE`; defaults to
  `https://api.azexecute.com/.default`.
- `request_timeout_seconds` (Number) — timeout for each API request, from `1`
  through `600`. Defaults to `30`.

## Tenant-Driven Metadata

Only `display_name` is statically required by the application resources. Every
metadata argument is optional in the Terraform schema because tenants decide
which fields are included and required. During planning, the provider reads the
live capabilities endpoint and reports missing tenant-required values before
apply. The API repeats that validation before it creates or changes anything.

Use the `azexecute_capabilities` data source to inspect the effective policy.

## Related Guides

- [Getting started](guides/getting-started.md)
- [Authentication](guides/authentication.md)
- [Azure DevOps](guides/azure-devops.md)
- [Approval and provisioning workflows](guides/approval-workflows.md)
- [Upgrade and state migration](guides/migration-v0.5.md)
- [Troubleshooting](guides/troubleshooting.md)
