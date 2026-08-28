---
page_title: "azexecute_capabilities Data Source"
description: |-
  Reads the live tenant policy enforced by the AZExecute Terraform API.
---

# azexecute_capabilities

Reads the tenant's current Terraform capabilities. Use it to expose policy in
outputs, gate modules, or diagnose why a plan is rejected. Application
resources also read these capabilities automatically during planning.

## Example Usage

```terraform
data "azexecute_capabilities" "current" {}

output "terraform_enabled" {
  value = data.azexecute_capabilities.current.enabled
}

output "application_approval_enabled" {
  value = data.azexecute_capabilities.current.use_application_request_flow
}

output "required_metadata_fields" {
  value = data.azexecute_capabilities.current.required_metadata_fields
}
```

## Schema

This data source has no arguments.

### Read-Only

- `id` (String) — stable data-source ID; currently `tenant`.
- `api_version` (String) — AZExecute Terraform API contract version.
- `enabled` (Boolean) — whether the dedicated Terraform API is enabled.
- `allow_application_creation` (Boolean) — whether Terraform can submit
  application creates/requests.
- `allow_application_deletion` (Boolean) — whether Terraform can delete a
  provisioned application.
- `allow_api_permission_requests` (Boolean) — whether application resources can
  contain `api_permission_request` blocks.
- `allow_registration_configuration` (Boolean) — whether Terraform can manage
  the supported registration settings.
- `use_application_request_flow` (Boolean) — whether application creation needs
  normal AZExecute administrative approval.
- `use_api_permission_request_flow` (Boolean) — whether permission requests use
  the normal approval flow. When false, supported permission requests can be
  applied automatically using tenant-admin authority.
- `included_metadata_fields` (Set of String) — metadata fields enabled in the
  tenant's Terraform/application metadata policy, using provider argument
  names.
- `required_metadata_fields` (Set of String) — enabled metadata fields that the
  tenant currently requires, using provider argument names.

The capability response is evaluated on every read. A previously generated
plan can still be rejected by the API if a tenant administrator changes policy
before apply; this is intentional defense in depth.
