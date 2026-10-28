---
page_title: "azexecute_azgroup_membership Resource - AZExecute"
subcategory: "Groups"
description: |-
  Manage individual user, application, integration, and machine assignments to AZExecute groups.
---

# azexecute_azgroup_membership (Resource)

Manages one assignment. Other memberships are preserved. Removing an assignment does not delete the underlying resource.

```hcl
resource "azexecute_azgroup_membership" "application" {
  group_id  = azexecute_azgroup.team.group_id
  type      = "application"
  target_id = azexecute_application.api.application_entity_id
}

resource "azexecute_azgroup_membership" "owner" {
  group_id  = azexecute_azgroup.team.group_id
  type      = "user"
  target_id = var.owner_object_id
  role      = "Owner"
}
```

| Argument | Description |
| --- | --- |
| `group_id` | Required positive numeric AZExecute group ID. |
| `type` | Required: `user`, `application`, `integration`, or `machine`. |
| `target_id` | Required string. User object UUID, AZExecute application entity UUID, numeric integration ID, or AZExecute machine UUID. |
| `role` | Users only: `Owner`, `Member`, or `Viewer`. Omission means `Member`. |

`id` is computed as `group_id/type/target_id`. Changing the group, type, or target replaces the assignment; user roles update in place. An existing active assignment is reconciled to the requested role, so do not declare the same assignment in multiple states.

Group ownership is required. Application links additionally require application ownership. Machine and integration links require TenantAdmin. Terraform policy is checked on every call. The last active Owner cannot be removed or demoted. Entra user memberships are read-only in AZExecute and must be maintained in Entra; application, integration, and machine links still work with Entra groups.

The creator's automatic Owner membership is not removed by other membership resources. If managing that assignment explicitly, retain another active Owner before destroying it. Access loss produces an error and retains state; it is not treated as a deleted membership.

## Import

```shell
terraform import azexecute_azgroup_membership.owner 42/user/11111111-2222-4333-8444-555555555555
terraform import azexecute_azgroup_membership.connection 42/integration/7
```
