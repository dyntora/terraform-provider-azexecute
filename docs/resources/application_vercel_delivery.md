---
page_title: "azexecute_application_vercel_delivery Resource - AZExecute"
subcategory: "Applications"
description: |-
  Deliver a managed application's rotated client secret to Vercel.
---

# azexecute_application_vercel_delivery

Available in 0.15.0. This resource owns the complete list of Vercel destinations for one AZExecute application. Configure automatic renewal with [azexecute_application_secret_rotation](application_secret_rotation.md). Each connection must be enabled and assigned to an active group containing the application. The caller needs application owner or tenant administrator access, the Integrations feature and tenant permission to configure applications through Terraform.

```hcl
resource "azexecute_application_vercel_delivery" "portal" {
  application_entity_id = var.application_entity_id
  destinations = [{
    integration_id       = azexecute_vercel_integration.portal.integration_id
    project_id           = var.vercel_project_id
    variable_name        = "CLIENT_SECRET"
    environments         = [var.uat_environment_id, var.staging_environment_id]
    redeploy_environments = [var.uat_environment_id]
  }]
}
```

| Argument | Meaning |
| --- | --- |
| `application_entity_id` | Required AZExecute managed application UUID. |
| `destinations` | Required ordered list of 1–10 distinct Vercel projects. |
| `integration_id` | Numeric AZExecute connection ID from the integration resource or data source. |
| `project_id` | Vercel project ID. |
| `variable_name` | Environment variable receiving the rotated application secret. |
| `environments` | Set of built-in names (`production`, `preview`, `development`) or custom environment IDs. Use the IDs for UAT and Staging. |
| `redeploy_environments` | Optional subset of the selected hosted environments to rebuild. Omit for variable updates only. Development reads the variable locally. |

Import using the application UUID, for example `terraform import azexecute_application_vercel_delivery.portal 11111111-2222-4333-8444-555555555555`. Destroy disconnects Vercel delivery for the application. Its rotation schedule, existing Vercel values, and other delivery providers are preserved.

If the same configuration creates the group assignments, add `depends_on` for the application and integration membership resources to the delivery resource. The project's custom environment IDs can come from the Vercel provider in the same configuration.
