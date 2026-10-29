---
page_title: "azexecute_azgroup Resource - AZExecute"
subcategory: "Groups"
description: |-
  Manage local AZExecute groups and tenant-governed Entra group creation.
---

# azexecute_azgroup (Resource)

Requires provider 0.13.0 or later.

Creates and manages one AZExecute group. The caller must be registered in the tenant, Terraform must be enabled, and group creation must be allowed for the caller. The creator becomes an Owner. Group ownership does not grant application ownership or tenant-administrator permissions.

```hcl
data "azexecute_group_capabilities" "tenant" {}

resource "azexecute_azgroup" "team" {
  name        = "Platform"
  description = "Platform applications and operators"
  source      = "auto"
}

output "group_id" {
  value = azexecute_azgroup.team.group_id
}
```

## Local and Entra groups

`source = "auto"` follows the tenant's group-creation setting **at creation time**. Later policy changes do not move an existing group between local and Entra. `source = "local"` explicitly requests a local group; overriding an Entra default requires TenantAdmin. `source = "entra"` fails unless Entra creation is enabled and a configured Customer Broker or Agent Broker can write groups with `Group.ReadWrite.All`. The AZExecute broker is read-only.

Entra creation applies the tenant's prefix, name template, and postfix. `name` remains the requested input and `display_name` is the resulting directory name. Renaming a Terraform-created Entra group applies the current naming policy. Names of imported Entra groups are treated literally. Name and description changes are sent through the configured broker; user membership and availability remain governed by Entra synchronization. Keep synchronization enabled to detect changes made outside AZExecute.

Use [`azexecute_azgroup_membership`](azgroup_membership.md) for individual AZExecute assignments. User memberships of Entra groups must be managed in Entra; this resource does not replace synchronized members or owners.

## Arguments

| Name | Description |
| --- | --- |
| `name` | Required. 1–200 characters; no surrounding whitespace or control characters. |
| `description` | Optional. Up to 1,000 characters. |
| `source` | `auto` (default), `local`, or `entra`. Changing this requires replacement. |
| `group_type` | Optional AZExecute classification, up to 50 characters. |
| `active` | Defaults to `true`. Local groups can be deactivated; Entra synchronization controls Entra availability. |
| `color_hex` | Optional presentation color, up to 20 characters. |
| `icon` | Optional presentation icon, up to 100 characters. |
| `delete_entra_on_destroy` | Defaults to `false`. Explicitly opt into directory deletion. Apply this change before destroying. |

Computed fields: `id` (Terraform UUID), `group_id` (numeric AZExecute ID), `display_name`, `actual_source` (`local` or `entra`), `entra_object_id`, and `status` (`Ready`, `Pending`, or `Deleting`). Membership resources use `group_id`, not `id`.

## Destroy and recovery

Provider 0.13.6 requires an explicit group-absence response before forgetting a
group, and HTTP 204 before accepting deletion. Proxy 404s, changed group identities,
and pending deletion responses retain state. Older APIs may need an upgrade to
confirm absence; do not remove state to bypass that diagnostic.

Destroying a local group removes the group and its assignments, preserving the underlying users, applications, integrations, and machines. Non-admin callers must own every linked application. Machine assignments and integrations outside the supported owner-managed providers require TenantAdmin. Inactive groups require TenantAdmin for further management.

Entra groups are **retained by default**, including their AZExecute assignments. Destroy releases Terraform management and emits a warning. To delete an Entra group, set `delete_entra_on_destroy = true`, apply, then destroy using a TenantAdmin identity. Entra group management must still be enabled and the broker must permit deletion. Only groups created by this Terraform resource can be deleted; imported Entra groups are protected. Deletion can revoke access outside AZExecute too.

Creation uses a stable recovery UUID. If the directory result is uncertain, AZExecute retains the pending group and does not repeat an ambiguous creation. The diagnostic includes the recovery ID. Check the group in AZExecute/Entra, resolve the pending operation with an administrator, then import or refresh it. Do not remove a pending group from state or force replacement to retry creation. A failed create with saved state may be marked tainted by Terraform; resolve the pending operation before untainting it.

An uncertain deletion retains its claim and state for retry. Authorization failures, broker errors, and incomplete responses retain Terraform state; only confirmed resource absence removes it. Restoring a deleted Entra object does not automatically re-enable its AZExecute assignments.

## Import

```shell
terraform import azexecute_azgroup.team 42
```

A numeric AZExecute ID adopts an existing group under the caller's ownership permissions without creating a directory group. Import can also use a previously recorded Terraform UUID. Dedicated application-integration groups and pending unmanaged creations cannot be adopted. Match the imported `source` and metadata in configuration before applying to avoid replacement. One group should be managed by only one Terraform state.
