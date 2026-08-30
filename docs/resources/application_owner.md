---
page_title: "azexecute_application_owner Resource"
description: |-
  Manages one Microsoft Entra owner on an AZExecute application request or provisioned application.
---

# azexecute_application_owner

Manages a single owner independently from the application resource. It can be
used while an application request is pending or after the application reaches
`Ready`. AZExecute stores the pending owner with the request and reconciles the
owner into both AZExecute and Microsoft Entra during or after provisioning.

This resource is intended for `for_each`, reusable ownership modules, and cases
where different Terraform modules own different people or automation
identities.

## Example Usage

```terraform
resource "azexecute_application_request" "example" {
  display_name = "platform-production"

  # owner_object_ids must be omitted when owners are separate resources.
}

variable "owner_object_ids" {
  type = set(string)
}

resource "azexecute_application_owner" "example" {
  for_each = var.owner_object_ids

  application_resource_id = azexecute_application_request.example.id
  owner_object_id         = each.value
}
```

## Ownership Modes

Choose exactly one ownership mode for an application:

- Inline authoritative mode: set `owner_object_ids` on
  `azexecute_application` or `azexecute_application_request`.
- Individual mode: omit `owner_object_ids` and create one
  `azexecute_application_owner` resource per desired owner.
- Unmanaged mode: omit both forms. AZExecute reports live owners but Terraform
  does not reconcile them.

Do not combine inline and individual ownership. The inline set is authoritative
and would remove owners represented only by individual resources. Individual
owner API operations are atomic, so several resources can safely run in
parallel without overwriting each other.

## Schema

### Required

- `application_resource_id` (String) — stable resource UUID exported as `id`
  by `azexecute_application` or `azexecute_application_request`. Changing it
  replaces this owner resource.
- `owner_object_id` (String) — Microsoft Entra directory object UUID for the
  owner. Changing it replaces this owner resource.

### Read-Only

- `id` (String) — composite identifier in
  `application-resource-uuid/owner-object-uuid` form.

## Drift and Lifecycle

If the owner is removed manually, the next refresh removes this resource from
state and Terraform plans to add it again. Removing the resource from
configuration removes the AZExecute-managed owner from both AZExecute and
Microsoft Entra. Graph-only operational identities are not exposed as this
resource and are not removed.

To allow ownership to be changed outside Terraform, use unmanaged mode instead
of retaining individual owner resources. Terraform cannot use
`lifecycle.ignore_changes` to ignore the absence of an entire remote resource.

## Import

Import an existing AZExecute-managed owner with the application Terraform
resource UUID and owner object UUID separated by `/`:

```shell
terraform import azexecute_application_owner.example 11111111-2222-4333-8444-555555555555/aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee
```

The first UUID is not the Entra client ID, Entra object ID, or AZExecute
application entity ID. It is the stable `id` exported by the parent Terraform
application resource.
