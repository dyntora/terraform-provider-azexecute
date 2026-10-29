---
page_title: "azexecute_aws_integration Resource - AZExecute"
subcategory: "Integrations"
description: |-
  Manage an AZExecute AWS integration.
---

# azexecute_aws_integration (Resource)

Requires provider 0.13.5 or later, the enabled Terraform API, and the Integrations license feature. A registered caller can create a connection and manage connections they created. TenantAdmin can also manage existing connections; Operator alone does not grant that access. Application-owned integrations require TenantAdmin. Every operation rechecks tenant policy and access.

Configure an AWS IAM OIDC provider and role trust using [`azexecute_aws_federation`](../data-sources/aws_federation.md). Attach permissions for the selected service and resources before creating the integration. AZExecute verifies that it can assume the role; successful verification does not prove every downstream service action is permitted. See the [complete setup and group linkage guide](../guides/integrations.md).

## Example

```hcl
resource "azexecute_aws_integration" "production" {
  name           = "Production secrets"
  service        = "secrets_manager"
  account_id     = "123456789012"
  role_arn       = "arn:aws:iam::123456789012:role/azexecute"
  default_region = "eu-west-1"
  global         = false
}

resource "azexecute_azgroup_membership" "connection" {
  group_id  = azexecute_azgroup.team.group_id
  type      = "integration"
  target_id = tostring(azexecute_aws_integration.production.integration_id)
}
```

The group can be a resource or an existing `azexecute_azgroup` data source. Creating a connection does not assign it to a group automatically. Group linkage requires group ownership and integration-creator access, or TenantAdmin.

## Arguments

| Argument | Description |
| --- | --- |
| `name` | Required friendly name, 1–200 characters, without surrounding whitespace or control characters. |
| `description` | Optional description, maximum 1000 characters. |
| `notes` | Optional notes, maximum 2000 characters. |
| `enabled` | Optional boolean, default `true`. |
| `global` | Optional boolean, default `false`. Controls global availability under AZExecute access rules; use group-scoped access for the linked workflow. |
| `service` | Required. `secrets_manager`, `parameter_store`, or `route53`. Immutable; changing it replaces the integration. Imported older multi-service integrations use `legacy`; new legacy integrations cannot be created. |
| `account_id` | Required 12-digit AWS account ID. Immutable. |
| `role_arn` | Required IAM role ARN belonging to `account_id`. Editable. |
| `default_region` | Optional AWS region, maximum 50 characters. |
| `account_alias` | Optional account alias, maximum 200 characters. |
| `external_id` | Optional sensitive legacy setting, maximum 200 characters. Omit to preserve; empty string clears. The current OIDC connection uses `AssumeRoleWithWebIdentity`, which does not send an external ID. |

## Read-only attributes

- `id`: stable Terraform resource UUID, used for recovery and import.
- `integration_id`: numeric AZExecute integration ID, used by group memberships and application integration settings.

## Credentials and updates

Credentials are never returned by the API. Configured secrets remain in Terraform state and are marked sensitive; use an encrypted remote state backend with restricted access. Reads preserve configured secrets, but cannot detect rotations made outside Terraform. Import does not retrieve secrets. Omit credentials after import to keep stored values, or provide replacements to take control of rotation. Removing an optional secret from configuration preserves its stored value; use an explicit empty string to clear optional secrets. Required credentials cannot be cleared.

Names, metadata, enabled/global flags, and connection settings other than the immutable identifiers update in place. Updates verify connectivity before saving. Read refreshes metadata without making an external connection check. A disabled integration is still editable and importable.

## Import and recovery

```shell
terraform import azexecute_aws_integration.production 42
# Or use a previously recorded Terraform UUID:
terraform import azexecute_aws_integration.production 11111111-2222-4333-8444-555555555555
```

The numeric ID must identify the matching integration type in this tenant. Use a data source to reuse an integration without managing its lifecycle. Do not declare the same connection in multiple resources or states. Duplicate account/installation creation is rejected; import the existing integration instead. After an interrupted create, inspect/import the recovery UUID reported in the error before retrying. Permission errors, incomplete responses, and unconfirmed deletion retain state.

## Destroy

Destroy removes the AZExecute connection, not the external account, IAM role, Vercel team, or GitHub App. Remove all group assignments first. Membership resources that reference `integration_id` are destroyed before the integration automatically. Detach dependent application, task, script, or schedule references if deletion reports a reference conflict.
