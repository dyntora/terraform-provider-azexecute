---
page_title: "azexecute_github_integration Data Source - AZExecute"
subcategory: "Integrations"
description: |-
  Read an existing AZExecute github integration without managing its lifecycle.
---

# azexecute_github_integration (Data Source)

Requires provider 0.13.5 or later. Reads the matching integration by its numeric AZExecute ID without adopting it, changing its settings, or modifying memberships. Requires the creator or TenantAdmin, the enabled Terraform API, and the Integrations license feature. Credentials are not exposed.

```hcl
data "azexecute_github_integration" "existing" {
  integration_id = 42
}

data "azexecute_azgroup" "existing" {
  group_id = 7
}

resource "azexecute_azgroup_membership" "connection" {
  group_id  = data.azexecute_azgroup.existing.group_id
  type      = "integration"
  target_id = tostring(data.azexecute_github_integration.existing.integration_id)
}
```

## Arguments

- `integration_id` (required): positive numeric AZExecute integration ID.

## Attributes

- `id`: Terraform UUID, or null if the connection has never been imported.
- `name`, `description`, `notes`, `enabled`, `global`: current connection metadata.
- `app_id`, `installation_id`: connection settings.

See the [resource reference](../resources/github_integration.md) for field meanings and provider-specific limitations.
