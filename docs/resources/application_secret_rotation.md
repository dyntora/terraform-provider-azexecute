---
page_title: "azexecute_application_secret_rotation Resource"
description: |-
  Schedule automatic client secret rotation for a managed application.
---

# azexecute_application_secret_rotation

Schedules automatic client secret renewal for one managed application. The caller must own the app or be a tenant administrator. The tenant must enable the Terraform API and Allow Registration Configuration and license application lifecycle management. The app must be active when applying changes.

```hcl
resource "azexecute_application_request" "api" {
  display_name             = "orders-api"
  business_justification   = "Runs the orders service"
  department_owner         = "Platform engineering"
  environment              = "Production"
  data_access_requirements = "Reads order status"
  wait_for_completion      = true
  create_timeout_minutes   = 60
}

resource "azexecute_application_secret_rotation" "api" {
  application_entity_id = azexecute_application_request.api.application_entity_id
  lifetime_days         = 30
  renew_before_hours    = 12
}
```

The reference makes Terraform wait for the request to finish before configuring rotation in the same apply. The request defaults to asynchronous submission; enable `wait_for_completion` when chaining resources. Waiting includes approval time. For an existing app, supply its AZExecute UUID directly.

- `application_entity_id` is the AZExecute managed application UUID. It is available after provisioning and approval.
- `lifetime_days` is the positive lifetime for future secrets, subject to the tenant's maximum.
- `renew_before_hours` is the lead time for renewal, from 1 to 24 hours and shorter than the lifetime. The worker honors the app's existing UTC rotation window and configured delivery destinations.

Applying the resource enables the schedule. An app without a previous renewal can rotate on the next eligible scheduler run. The secret value stays out of Terraform state. Changes apply in place. Destroying the schedule disables automatic secret renewal and preserves the app, existing credentials, and delivery configuration. Use a single Terraform resource per app.

Import an existing enabled schedule using the managed application UUID:

```shell
terraform import azexecute_application_secret_rotation.api 11111111-2222-4333-8444-555555555555
```
