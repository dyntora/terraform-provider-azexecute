---
page_title: "azexecute_group_capabilities Data Source - AZExecute"
subcategory: "Groups"
description: |-
  Inspect tenant group policy and Entra broker availability for the current caller.
---

# azexecute_group_capabilities (Data Source)

```hcl
data "azexecute_group_capabilities" "tenant" {}
```

No arguments. Exports:

- `enabled`: whether the tenant enables Terraform.
- `can_create_groups`: whether the caller's role and group policy allow creation.
- `can_create_local_groups`: whether local creation is permitted without overriding policy.
- `create_groups_in_entra`: the tenant's default creation destination.
- `entra_unavailable_reason`: a prerequisite problem, or null when the configured broker is available.

These are preflight hints, not permission grants. Creation still checks the registered identity, current tenant policy, broker permissions, and group inputs. A broker availability result does not certify that a future Graph write will succeed.
