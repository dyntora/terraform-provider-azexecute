---
page_title: "AZExecute Provider: Troubleshooting"
subcategory: "Guides"
description: |-
  Diagnose authentication, tenant policy, approval, state, and provider-version problems.
---

# Troubleshooting

## A Metadata Field Is Reported as Required

Only `display_name` is statically required. Other metadata requirements come
from the tenant's live AZExecute settings.

1. Read `data.azexecute_capabilities.current.required_metadata_fields`.
2. Open **Tenant administration → Application Configuration → Terraform**.
3. Either provide the field or change the tenant policy intentionally.

The provider checks this during plan and the API checks again before creating a
request. An invalid configuration should not leave a pending request.

If Terraform says the provider schema itself marks a metadata field as required,
verify the installed provider version and lock file. That message indicates an
older provider build rather than a live tenant-policy error.

## Provider Version Is Stale

Run:

```shell
terraform version
terraform providers
terraform init -upgrade
```

Verify the configuration requires `dyntora/azexecute` `~> 0.5` and inspect
`.terraform.lock.hcl`. CI/CD caches and mirrors must also contain the selected
release. Provider `0.5.0` is the first release containing the complete Registry
reference and the approval-aware resource documentation in the same tag.

## 401 Unauthorized

- Confirm the token was issued by the customer tenant.
- Confirm the scope is `https://api.azexecute.com/.default`.
- Check certificate or secret validity.
- For OIDC, confirm the assertion has not expired and the federated credential
  subject and audience match the pipeline.

## 403 Forbidden

- Confirm the Terraform API and requested operation are enabled.
- Confirm the caller's service principal is assigned `User`, `Operator`, or
  `TenantAdmin` on the AZExecute enterprise application.
- A `User` can manage only resources created by that same identity.
- Blob data access on Terraform state does not grant AZExecute access.

## OIDC Fails in Azure DevOps

- The service connection must use workload identity federation.
- `AzureCLI@2` must set `addSpnToEnvironment: true`.
- Read `idToken`, `servicePrincipalId`, and `tenantId` inside the same task.
- Set both the `ARM_*` variables for the backend and `AZEXECUTE_*` variables for
  the provider.
- Do not use the Azure management access token as `AZEXECUTE_OIDC_TOKEN`; the
  provider expects the federated assertion.

## OIDC Fails in GitHub Actions

- Grant `id-token: write`.
- Configure a federated credential for the exact repository, ref, or GitHub
  environment subject.
- Ensure `AZEXECUTE_TENANT_ID` and `AZEXECUTE_CLIENT_ID` are present.
- Match `AZEXECUTE_OIDC_AUDIENCE` to the federated credential when it differs
  from `api://AzureADTokenExchange`.

## Status Remains PendingApproval

This is expected until an administrator approves or rejects the AZExecute
application request. Terraform does not self-approve. After approval and
background provisioning, run Terraform again.

## Status Remains Provisioning

Check the request and background operation in AZExecute. Do not repeatedly
remove state or change the resource UUID: create is idempotent and a later run
will refresh the same request.

## Status Is Rejected

Read `status_reason` and review the request in AZExecute. Rejection is retained
in state. Decide whether to correct the governance request outside Terraform or
destroy/cancel the rejected resource before submitting a replacement.

## 409 Conflict

A conflict can mean:

- the request is not ready for an update;
- a live Terraform application name already has different requested settings;
- a registration concurrency token became stale;
- a resource UUID was reused for different configuration.

Refresh state, inspect the existing request in AZExecute, and rerun plan. Do not
manually invent or reuse resource UUIDs.

## Destroy Is Denied

Pending and rejected requests can be cancelled without application-deletion
permission. A provisioned application requires **Allow Application Deletion**
in the tenant Terraform policy. Review the destroy plan before enabling that
capability.

## State Lock Problems

Confirm no other run is active. Azure Blob state uses a lease for locking; the
service connection needs Blob data access. Do not break a lease unless the
owning run is known to be gone. Use Terraform's `-lock-timeout` option for normal
contention.

## Getting More Detail

Set `TF_LOG=INFO` or `TF_LOG=DEBUG` only for a controlled diagnostic run. Logs
can contain identifiers and request details; store and share them accordingly.
Never enable verbose logging while printing secrets or OIDC assertions.
