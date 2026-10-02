---
page_title: "azexecute_event_listener Resource - AZExecute"
subcategory: "Automation"
description: |-
  Manage application event listeners and protected renewal input mappings.
---

# azexecute_event_listener (Resource)

Manages one listener through the shared public `api/v1/EventListeners` API. Multiple resources may target the same application. Available from provider version 0.9.0. Requires a server with event-listener catalog/task discovery support and the application-event authorization migration. It does not apply migrations or create a task.

Application owners can manage self-service listeners using published User tasks or active User/Editor/Owner task grants to their user, tenant group, or User role. Viewer grants do not allow execution. Ownership and task access are checked again for future runs; approvals remain in force. Operator/TenantAdmin callers can also manage broad listeners and TOPdesk actions. Saving an owner rule as an operator makes it operator-managed. Use a stable authenticated identity for ongoing management; a service principal does not impersonate a human owner.

The Integrations license feature must be enabled. The application provisioning Terraform settings govern application resources; event listeners use the public event API's licensing and permissions.

## Example

```terraform
resource "azexecute_event_listener" "deploy_secret" {
  name                  = "Deploy renewed production secret"
  application_entity_id = azexecute_application.production.application_entity_id
  event_type            = "ApplicationCredentialRenewed"
  credential_type       = "Secret"
  automation_task_id    = 42
  enabled               = true
  execution_order       = 100

  parameters = {
    ApplicationName = "{{ application_name }}"
    DeploymentInput = "{{ renewed_secret }}"
  }
}
```

`application_entity_id` is the **AZExecute application entity UUID**, not the Terraform resource `id`, Entra application/client ID, or object ID. Wait until application provisioning has completed before adding listeners; an asynchronous application's entity ID may not yet exist.

Use `GET /api/v1/EventListeners/Catalog` to discover event types and variables, `GET /api/v1/EventListeners/AutomationTasks?applicationEntityId=<uuid>&search=<name>` to select an allowed task, and `GET /api/v1/automation-tasks/<id>` for its safe input contract. The task picker returns at most 50 matches; narrow the search if needed.

## Credential handling

Only configure **references**, never a renewed credential value or an encrypted runtime envelope. Protected mappings require `ApplicationCredentialRenewed`, an explicit application, `Secret` or `Certificate`, and an automation task. A protected token must occupy the entire input value:

- Secret: `{{ renewed_secret }}`.
- Certificate: `{{ renewed_certificate_pem }}` and `{{ renewed_certificate_private_key_pem }}`.

Protected runtime material expires after seven days. The listener resource never fetches it; Terraform stores only the configured reference. Scripts and destinations receiving the material are responsible for secure handling. Other literal inputs are stored in Terraform state even though `parameters` is marked sensitive; secure your state backend.

## Arguments

- `name` (String, required): 1–160 characters, without surrounding whitespace.
- `event_type` (String, required): Name from the event catalog.
- `application_entity_id` (String, optional): Application entity UUID. Required unless `broad_listener` is true. Changing scope to another application replaces the listener.
- `broad_listener` (Boolean, optional): Defaults to false. Explicitly opt into tenant-wide scope, without an application ID. Requires Operator/TenantAdmin.
- `credential_type` (String, optional): `Secret` or `Certificate`; omitted means all credentials. Only valid for credential events.
- `automation_task_id` (Number, optional): Existing task ID. Exactly one of this or `topdesk_settings` is required; automation is the normal action.
- `parameters` (Map of String, sensitive, optional): Visible task input names to literal values or event templates. Defaults to an empty map. Unknown/hidden inputs are rejected for self-service rules.
- `description` (String, optional): Up to 500 characters, without surrounding whitespace.
- `enabled` (Boolean, optional): Defaults to true. Enabling a listener authorizes future event-triggered work.
- `execution_order` (Number, optional): 0–10000, default 100. Lower numbers run first.
- `topdesk_settings` (Object, optional): Operator-only action using an existing integration. Requires `integration_id`. Supports the public TOPdesk settings listed below. Cannot be combined with task inputs.

### TOPdesk settings

```terraform
resource "azexecute_event_listener" "renewal_incident" {
  name                  = "Record renewal failure"
  application_entity_id = "11111111-2222-4333-8444-555555555555"
  event_type            = "ApplicationCredentialRenewalFailed"
  topdesk_settings = {
    integration_id    = 4
    operation_type    = "CreateIncident"
    brief_description = "Credential renewal failed"
    request_text      = "Application: {{ application_name }}; error: {{ error }}"
  }
}
```

Optional fields (server defaults are read back when omitted): `operation_type`, `incident_reference_name`, `incident_number_or_id`, `brief_description`, `request_text`, `action_text`, `caller_id`, `caller_display_name`, `caller_lookup_value`, `caller_dynamic_name`, `caller_email`, `entry_type_id`, `entry_type_name`, `incident_line_status`, `processing_status_id`, `processing_status_name`, `operator_group_id`, `operator_group_name`, `operator_id`, `operator_name`, `category_id`, `category_name`, `subcategory_id`, `subcategory_name`. `integration_name` is read-only. Operation names include `CreateIncident`, `UpdateIncident`, `CloseIncident`, `CreateChange`, `UpdateChange`, `CancelChange`, `ArchiveChange`, and `UnarchiveChange`.

## Read-only attributes

- `id`: Listener ID.
- `action_type`: Action inferred from the configured settings.
- `authorized_by_user_id`: Server-assigned authorizing owner ID, or null for operator-managed rules.

## Import

```shell
terraform import azexecute_event_listener.deploy_secret 123
```

Import adopts the visible listener configuration. A 404 removes it from state; permission and other errors are reported. Application owners cannot import operator-managed rules.

Create requests are deliberately not retried automatically. If a response is lost, check the application’s listeners and import the created listener before another apply to avoid duplicate automation. Deleting a listener stops future triggers; it does not cancel runs already created.
