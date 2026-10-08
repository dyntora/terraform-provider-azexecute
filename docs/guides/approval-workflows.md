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
7. When AZExecute reports `Ready`, Terraform records the Entra identifiers.
   AZExecute has already applied the requested registration settings.

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

Set `configure_registration = true` and enable registration configuration in the
tenant settings. The provider sends the full configuration with the creation
request, including redirect URIs, app roles, exposed scopes and pre-authorized
clients. Reviewers see that snapshot. AZExecute applies it after approval and
before reporting `Ready`; no second Terraform apply is needed for provisioning.

Pending and rejected requests retain their requested configuration. Rejection
creates no registration. A later Terraform refresh reads status and configuration
without approving the request. Changes to an existing request still require it
to be ready, or an explicit replacement with a reviewed destroy/create plan.

The new request behavior requires the API's `supports_registration_requests`
capability. Old providers remain compatible with API v1 and continue to configure
registrations after creation. Existing live updates use concurrency tokens and
preserve omitted scopes and clients.

## Destroy Behavior

If a previously provisioned application is subsequently removed from AZExecute
and Entra, AZExecute retires the old Terraform association and releases
its creation-name reservation. The original request remains as history. A normal
refreshed Terraform plan reports a creation; apply submits a **new request** with
a new resource UUID. Current tenant policy still applies: automatic approval
starts provisioning, while manual approval returns `PendingApproval`. The new
Entra application receives a new client ID; recreation cannot preserve it.

An application that is still provisioning is not treated as deleted. Access
denials and service failures must not remove the resource from Terraform state.
Do not disable refresh when recovering a deleted application.

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
