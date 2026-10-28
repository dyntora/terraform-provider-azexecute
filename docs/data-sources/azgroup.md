---
page_title: "azexecute_azgroup Data Source - AZExecute"
subcategory: "Groups"
description: |-
  Read an existing AZExecute group without adopting it into Terraform management.
---

# azexecute_azgroup (Data Source)

```hcl
data "azexecute_azgroup" "existing" {
  group_id = 42
}
```

`group_id` is the required numeric AZExecute group ID. Terraform must be enabled and the caller must be a group Owner or TenantAdmin.

Exports `id` (existing Terraform UUID, or the empty UUID for an unmanaged group), `group_id`, `name`, `description`, `source`, `actual_source`, `display_name`, `entra_object_id`, `status`, `group_type`, `active`, `color_hex`, and `icon`. Names are the current display names. The lookup never adopts the group, changes its memberships, or opts into directory deletion; `delete_entra_on_destroy` is always false for this read-only view.
