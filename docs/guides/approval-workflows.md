---
page_title: "AZExecute Provider: Approval Workflows"
subcategory: "Guides"
description: |-
  Understand automatic provisioning, approval, rejection, and repeated Terraform runs.
---

# Approval and Provisioning Workflows

AZExecute tenants independently control application approval and API-permission
approval. The recommended `azexecute_application_request` resource works in all
combinations without weakening tenant governance.

## Application Approval Enabled

1. Terraform validates the live tenant policy during planning.
2. Apply submits the application request once.
3. AZExecute returns `PendingApproval` and Terraform stores that state without
   waiting or holding the remote state lock.
4. A tenant administrator reviews the request in AZExecute.
5. Approval queues durable background provisioning. Rejection changes the
   request to `Rejected`.
6. Run Terraform again. A read refreshes the status once.
7. When AZExecute reports `Ready`, Terraform records the Entra identifiers and
   applies configured registration settings.

Terraform never approves its own request. Approval remains an administrative
AZExecute action.

## Automatic Application Provisioning

When application approval is disabled, the POST operation starts provisioning
immediately. It can return:

- `Ready` when provisioning finished before the response;
- `Provisioning` when background work is still running.

If it returns `Provisioning`, run Terraform again after the background workflow
finishes. The asynchronous resource does not poll while holding state.

## Status Values

- `PendingApproval` — an administrator must approve or reject the request.
- `Provisioning` — AZExecute accepted the request and background creation is in
  progress.
- `Ready` — the application is provisioned and its identifiers are available.
- `Rejected` — an administrator rejected the request. `status_reason` may
  explain why.

Computed application identifiers are null until `Ready`.

## Rejected Requests

Rejection is preserved in Terraform state so automation can report the outcome.
Correct the request outside Terraform according to the tenant's governance
process, or destroy the rejected Terraform resource to cancel/remove its
Terraform association before submitting a replacement. A rejected request is
not silently recreated on every plan.

## Permission Approval

`api_permission_request` blocks are submitted with the application request.
Their behavior is controlled separately:

- with the Terraform permission approval flow enabled, they become normal
  AZExecute permission requests for review;
- with it disabled, supported permissions are applied automatically using the
  tenant administrator's configured authority.

Application readiness does not necessarily mean every permission request has
already received consent. Review permission status in AZExecute when the tenant
uses its approval flow.

## Registration Configuration

Registration fields can be sent only after the application exists. Set
`configure_registration = true` and enable registration configuration in the
tenant settings. With the asynchronous resource:

- if create returns `Ready`, configuration is applied immediately;
- otherwise, a later Terraform run applies it after status becomes `Ready`.

AZExecute uses a concurrency token to avoid overwriting an unrelated concurrent
registration change.

## Destroy Behavior

- Destroying a pending approval request cancels it and does not require
  application-deletion permission because no Entra application exists.
- Destroying a rejected request removes/cancels it without application-deletion
  permission.
- Destroying a provisioned or automatically provisioning application requires
  the tenant's Terraform application-deletion capability.

Always inspect a destroy plan. Removing a resource block from configuration has
the same deletion semantics as an explicit `terraform destroy`.

## Synchronous Resource

`azexecute_application` waits only for automatic provisioning. It fails during
planning when application approval is enabled and tells the caller to use
`azexecute_application_request`. Its wait controls are:

- `poll_interval_seconds`: `1`–`300`, default `5`;
- `create_timeout_minutes`: `1`–`1440`, default `60`.

Use it only when the tenant contract guarantees automatic provisioning.
