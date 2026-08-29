---
page_title: "azexecute_application Data Source"
description: |-
  Reads an AZExecute application request by its stable Terraform resource UUID.
---

# azexecute_application

Reads an application created through the AZExecute Terraform API. It can read
resources managed by either `azexecute_application_request` or
`azexecute_application` because both use the same stable API resource UUID.

## Example Usage

```terraform
data "azexecute_application" "existing" {
  id = "11111111-2222-4333-8444-555555555555"
}

output "status" {
  value = data.azexecute_application.existing.status
}

output "client_id" {
  value = data.azexecute_application.existing.application_id
}
```

## Schema

### Required

- `id` (String) — stable AZExecute Terraform resource UUID. This is not the
  Entra client ID, Entra object ID, numeric request ID, or numeric AZExecute
  application entity ID.

### Read-Only

- `display_name` (String) — application/request display name.
- `description` (String) — application description when present.
- `status` (String) — current lifecycle status, such as `PendingApproval`,
  `Provisioning`, `Ready`, or `Rejected`.
- `status_reason` (String) — status or rejection explanation when present.
- `request_id` (Number) — numeric AZExecute application request ID.
- `application_entity_id` (String) — AZExecute application entity UUID;
  null until provisioning completes.
- `application_id` (String) — Microsoft Entra application/client ID; null until
  provisioning completes.
- `application_object_id` (String) — Microsoft Entra application object ID;
  null until provisioning completes.
- `business_justification` (String) — normalized business justification stored
  by AZExecute.

Use a managed resource when Terraform should own updates or deletion. This data
source is read-only.
