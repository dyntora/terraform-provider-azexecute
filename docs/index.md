---
page_title: "Provider: AZExecute"
description: |-
  Manages the tenant-approved AZExecute application-registration surface.
---

# AZExecute Provider

Use this provider to create governed Microsoft Entra application registrations through AZExecute. The calling user or service principal needs an assigned AZExecute `User`, `Operator`, or `TenantAdmin` enterprise-application role, and the tenant Terraform policy must be enabled. Terraform never elevates the assigned role: `User` manages only its own Terraform resources, while `Operator` and `TenantAdmin` have tenant-wide access.

## Example Usage

```terraform
provider "azexecute" {
  endpoint  = "https://api.azexecute.com"
  tenant_id = var.tenant_id
  client_id = var.client_id
  scope     = "https://api.azexecute.com/.default"
}
```

## Schema

- `endpoint` (optional) — AZExecute API base URL; `AZEXECUTE_ENDPOINT` or `https://api.azexecute.com`.
- `tenant_id` (optional) — customer Entra tenant; `AZEXECUTE_TENANT_ID`, `ARM_TENANT_ID`, or `AZURE_TENANT_ID`.
- `client_id` (optional) — customer automation identity client ID; `AZEXECUTE_CLIENT_ID`, `ARM_CLIENT_ID`, or `AZURE_CLIENT_ID`.
- `client_secret` (optional, sensitive) — client-secret fallback; `AZEXECUTE_CLIENT_SECRET`, `ARM_CLIENT_SECRET`, or `AZURE_CLIENT_SECRET`.
- `client_certificate_path` (optional) — PEM or PKCS#12 certificate and private key; `AZEXECUTE_CLIENT_CERTIFICATE_PATH` or `ARM_CLIENT_CERTIFICATE_PATH`.
- `client_certificate_password` (optional, sensitive) — PKCS#12 password; `AZEXECUTE_CLIENT_CERTIFICATE_PASSWORD` or `ARM_CLIENT_CERTIFICATE_PASSWORD`.
- `send_certificate_chain` (optional) — sends the certificate chain for subject-name/issuer authentication.
- `access_token` (optional, sensitive) — pre-acquired API token; `AZEXECUTE_ACCESS_TOKEN`.
- `use_oidc` (optional) — enables workload identity federation; automatically enabled when an assertion or the GitHub OIDC environment is present.
- `oidc_token` (optional, sensitive) — federated assertion; `AZEXECUTE_OIDC_TOKEN` or `ARM_OIDC_TOKEN`.
- `oidc_token_file_path` (optional) — rotating assertion file; `AZEXECUTE_OIDC_TOKEN_FILE_PATH`, `ARM_OIDC_TOKEN_FILE_PATH`, or `AZURE_FEDERATED_TOKEN_FILE`.
- `oidc_audience` (optional) — GitHub OIDC audience; defaults to `api://AzureADTokenExchange`.
- `use_managed_identity` (optional) — uses a system- or user-assigned Azure managed identity; `AZEXECUTE_USE_MANAGED_IDENTITY` or `ARM_USE_MSI`.
- `scope` (optional) — API `.default` scope; `AZEXECUTE_SCOPE` or `https://api.azexecute.com/.default`.
- `request_timeout_seconds` (optional) — per-request timeout from 1 to 600 seconds.
