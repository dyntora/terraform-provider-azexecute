---
page_title: "AZExecute Provider: Upgrade to 0.5"
subcategory: "Guides"
description: |-
  Upgrade to the fully documented approval-aware provider and migrate synchronous resource state safely.
---

# Upgrade to 0.5 and State Migration

Provider `0.5` is the first release that ships the complete Registry
documentation and approval-aware examples in the same version tag. It provides
`azexecute_application_request` for approval-aware, non-blocking workflows.
Existing applications managed by `azexecute_application` can move to the new
resource without deleting or recreating the remote application.

## Before Upgrading

1. Commit or otherwise back up the Terraform configuration and state.
2. Confirm no other Terraform run is active.
3. Update the required provider constraint to `~> 0.5`.
4. Change the resource type when adopting the request resource.
5. Add a `moved` block.

## Configuration Migration

Before:

```terraform
resource "azexecute_application" "this" {
  display_name = "platform-deployment-production"
}
```

After:

```terraform
resource "azexecute_application_request" "this" {
  display_name = "platform-deployment-production"
}

moved {
  from = azexecute_application.this
  to   = azexecute_application_request.this
}
```

Remove `poll_interval_seconds` and `create_timeout_minutes`; the asynchronous
resource does not wait. Provider `0.5` supplies a cross-resource state mover
that converts state while preserving the stable AZExecute resource UUID and all
managed values.

Run:

```shell
terraform init -upgrade
terraform plan
```

The plan should report the address move and must not propose deleting and
recreating the application. Review the plan before applying it.

Keep the `moved` block long enough for every shared workspace/state instance to
consume the migration. It can be removed later when no state still uses the old
address.

## Provider Lock File

Commit `.terraform.lock.hcl` for normal root modules. After changing the version
constraint, run `terraform init -upgrade` and verify that the lock file selects
a `0.5.x` release. A lock pinned to an earlier release cannot guarantee the
documented `0.5` behavior.

## Import Instead of Move

When state is unavailable but the stable AZExecute Terraform resource UUID is
known:

```shell
terraform import azexecute_application_request.this 11111111-2222-4333-8444-555555555555
```

The import ID is not the Entra client ID, Entra object ID, or AZExecute numeric
application entity ID. After import, run `terraform plan` and align the
configuration with the values read from AZExecute.
