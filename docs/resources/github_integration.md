---
page_title: "azexecute_github_integration Resource - AZExecute"
subcategory: "Integrations"
description: |-
  Manage an AZExecute Github integration.
---

# azexecute_github_integration (Resource)

Requires provider 0.13.5 or later, the enabled Terraform API, and the Integrations license feature. A registered caller can create a connection and manage connections they created. TenantAdmin can also manage existing connections; Operator alone does not grant that access. Application-owned integrations require TenantAdmin. Every operation rechecks tenant policy and access.

Create and install a GitHub App on the required repositories, generate a private key, and supply the App ID and installation ID. AZExecute verifies installation access before saving. Grant only the repository permissions needed by your workflows, such as Actions write for dispatch and the relevant secrets or variables permissions for publication. This resource manages the AZExecute connection, not the GitHub App or installation. Public GitHub is supported; GitHub Enterprise endpoints are not supported by this resource. See [GitHub App installation authentication](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation).

## Example

```hcl
variable "github_private_key_pem" {
  type      = string
  sensitive = true
}

resource "azexecute_github_integration" "production" {
  name            = "Production GitHub"
  app_id          = "123456"
  installation_id = "789012"
  private_key_pem = var.github_private_key_pem
}

resource "azexecute_azgroup_membership" "connection" {
  group_id  = azexecute_azgroup.team.group_id
  type      = "integration"
  target_id = tostring(azexecute_github_integration.production.integration_id)
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
| `app_id` | Required numeric GitHub App ID, maximum 64 digits. Immutable. |
| `installation_id` | Required numeric installation ID, maximum 64 digits. Immutable. |
| `private_key_pem` | Sensitive GitHub App private key PEM, maximum 32768 characters. Required on creation; omit after import to retain the stored key. Supply a new value to rotate it. |
| `webhook_secret` | Optional sensitive webhook secret, maximum 500 characters. Omit to preserve; empty string clears. |

## Read-only attributes

- `id`: stable Terraform resource UUID, used for recovery and import.
- `integration_id`: numeric AZExecute integration ID, used by group memberships and application integration settings.

## Credentials and updates

Credentials are never returned by the API. Configured secrets remain in Terraform state and are marked sensitive; use an encrypted remote state backend with restricted access. Reads preserve configured secrets, but cannot detect rotations made outside Terraform. Import does not retrieve secrets. Omit credentials after import to keep stored values, or provide replacements to take control of rotation. Removing an optional secret from configuration preserves its stored value; use an explicit empty string to clear optional secrets. Required credentials cannot be cleared.

Names, metadata, enabled/global flags, and connection settings other than the immutable identifiers update in place. Updates verify connectivity before saving. Read refreshes metadata without making an external connection check. A disabled integration is still editable and importable.

## Import and recovery

```shell
terraform import azexecute_github_integration.production 42
# Or use a previously recorded Terraform UUID:
terraform import azexecute_github_integration.production 11111111-2222-4333-8444-555555555555
```

The numeric ID must identify the matching integration type in this tenant. Use a data source to reuse an integration without managing its lifecycle. Do not declare the same connection in multiple resources or states. Duplicate account/installation creation is rejected; import the existing integration instead. After an interrupted create, inspect/import the recovery UUID reported in the error before retrying. Permission errors, incomplete responses, and unconfirmed deletion retain state.

## Destroy

Destroy removes the AZExecute connection, not the external account, IAM role, Vercel team, or GitHub App. Remove all group assignments first. Membership resources that reference `integration_id` are destroyed before the integration automatically. Detach dependent application, task, script, or schedule references if deletion reports a reference conflict.
