---
page_title: "azexecute_vercel_task_step Resource - AZExecute"
subcategory: "Automation"
description: |-
  Configure a Vercel action in an existing AZExecute automation task.
---

# azexecute_vercel_task_step

Available in 0.15.0. The caller needs access to the Terraform API, editor access to the task, and access to the integrations. Creation appends a step. Updates preserve its position, group and other task settings. Use `depends_on` when creation order matters. Applying the configuration saves the step. Running the task executes it.

```hcl
resource "azexecute_vercel_task_step" "update" {
  task_id        = var.task_id
  integration_id = azexecute_vercel_integration.portal.integration_id
  operation      = "update_variable"
  project_id     = var.vercel_project_id
  environments   = [var.uat_environment_id, var.staging_environment_id]
  variable_name  = "CLIENT_SECRET"
  value_reference = "{{ NewSecret }}"
}

resource "azexecute_vercel_task_step" "redeploy_uat" {
  depends_on     = [azexecute_vercel_task_step.update]
  task_id        = var.task_id
  integration_id = azexecute_vercel_integration.portal.integration_id
  operation      = "redeploy"
  project_id     = var.vercel_project_id
  environments   = [var.uat_environment_id]
}

resource "azexecute_vercel_task_step" "rotate_team_token" {
  task_id               = var.maintenance_task_id
  integration_id        = azexecute_vercel_integration.account.integration_id
  target_integration_id = azexecute_vercel_integration.portal.integration_id
  operation             = "rotate_access_token"
}
```

| Argument | Meaning |
| --- | --- |
| `task_id` | Required numeric ID of an existing task. Changing it replaces the step. |
| `step_id` | Read-only AZExecute step ID. |
| `integration_id` | Required AZExecute Vercel integration ID. |
| `operation` | Required. Choose `update_variable`, `redeploy`, or `rotate_access_token`. |
| `team_id` | Team that owns the project. A Full Account connection can select a team or use its default. Team connections use their configured team. |
| `project_id` | Required for variable updates and redeployment. |
| `environments` | Set of target IDs. Use `production`, `preview`, `development`, or a custom `env_...` ID. Redeployment takes one hosted environment per step. |
| `variable_name` | Project environment variable key for an update. |
| `value_reference` | Reference to a task input or earlier output. Use the spaced form `{{ NewSecret }}`. The worker resolves it at execution. |
| `target_integration_id` | For rotation, omit to rotate the selected connection itself. A Full Account connection can also rotate a team token owned by the same Vercel user when Vercel permits issuance. |
| `enabled` | Defaults to true. |
| `stop_on_failure` | Defaults to true. |

Import an existing Vercel step using its task and step IDs, for example `terraform import azexecute_vercel_task_step.update 42/17`. If a create response is interrupted, check the task and import the new step before retrying. Destroy removes this step from the task. It leaves the integration, deployments, and Vercel variable values in place.
