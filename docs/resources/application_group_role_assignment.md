---
page_title: "azexecute_application_group_role_assignment Resource"
description: |-
  Manage one Microsoft Entra group assignment to an enterprise application role.
---

# azexecute_application_group_role_assignment

Use this resource to create, import, repair or remove one assignment. Enable `wait_for_completion = true` on a new application request so Terraform waits for the enterprise application. Set `app_roles` on the request to define the role and enable `allow_users_and_groups`.

```terraform
resource "azexecute_application_group_role_assignment" "readers" {
  application_resource_id = azexecute_application_request.api.id
  app_role_id             = var.reader_role_id
  group_object_id         = var.reader_group_object_id
}
```

- `application_resource_id` is the AZExecute Terraform application request UUID.
- `app_role_id` is the UUID of the app role defined on the target application.
- `group_object_id` is the Microsoft Entra group object UUID.

When the tenant uses application approval, include the same group and role in the request’s `initial_group_role_assignments`. After approval, a workload identity with application access can maintain that pair. Under the tenant’s automatic application creation policy it can manage group assignments on its managed application. A signed-in caller uses delegated Entra authorization. Writes require Terraform registration configuration and an active application.

Terraform refresh checks the group, role and enterprise application together. Destroy removes that assignment. Import an existing assignment using the three UUIDs in this order.

```shell
terraform import azexecute_application_group_role_assignment.readers APPLICATION_RESOURCE_UUID/APP_ROLE_UUID/GROUP_OBJECT_UUID
```
