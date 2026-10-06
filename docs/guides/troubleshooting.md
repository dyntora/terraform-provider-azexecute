---
page_title: "AZExecute Provider: Troubleshooting"
subcategory: "Guides"
description: |-
  Diagnose authentication, tenant policy, approval, state, and provider-version problems.
---

# Troubleshooting

## Minimum Additional Owners

The tenant's minimum additional-owner setting applies to Terraform, including
requests from service principals. Set `owner_object_ids` to distinct Entra user
object IDs. The original requester, service principals and deleted users do not
satisfy this minimum. Read `minimum_additional_owners` from
`data.azexecute_capabilities.current` to see the current requirement.

Required owners must be supplied on `azexecute_application_request` or
`azexecute_application` when creating it. Separate `azexecute_application_owner`
resources run after creation and cannot satisfy this validation. Planning catches
an insufficient known set; the API checks identities and current tenant policy
again during apply. Unknown IDs are deferred until apply.

Updates with unmanaged owners still check the existing owner set. Removing an
owner or replacing the set cannot leave fewer additional user owners than the
tenant requires. Add replacement owners before removing existing ones. Existing
applications below the minimum are not automatically changed by this release;
add the missing user owners before applying further updates.

## 400 Validation Failed

Validation diagnostics list the fields to correct, for example:

```text
AZExecute API returned HTTP 400: Terraform configuration validation failed
- project_name: Project Name is required by tenant metadata settings.
- app_roles[1].value: Choose a unique role value.
Error code: terraform_validation_failed (trace request-reference)
```

Correct each listed value and run `terraform plan` again. Nested indexes identify
items in the API request; Terraform sets do not have a stable configuration order,
so also use the role, scope or permission named in the message to locate the item.
The HTTP API retains full paths such as `metadata.projectName`; the provider
translates these into Terraform attribute names.

Deploy both the API and provider fixes to receive the complete diagnostics. Older
providers ignore the API's `errors` object. Older providers can also send unused
registration collections as `null`, causing required-field errors even when no
redirect URI or pre-authorized client was configured. The updated provider sends
empty arrays for those collections.

If no field details are returned, give the reference to an administrator. They
can find the original exception and API failure event using the same `TraceId` in
server logs. A generic summary alone does not establish which field was invalid.

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

Verify the configuration requires `dyntora/azexecute` `~> 0.11` and inspect
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

## A No-Change Run Plans an Update

Provider `0.7.0` preserves stable computed identifiers and status during an
ordinary update. Earlier builds could show `id`, `request_id`, application IDs,
status, and tenant-defaulted metadata as `(known after apply)` and could then
call the collection endpoint without a resource UUID, resulting in HTTP 405.

Provider `0.7.1` also protects registration updates from Microsoft Graph
replication delay. It verifies every managed registration field and retries a
stale post-update read before writing Terraform state. Deploy the matching
AZExecute API build so the update endpoint returns the configuration that
Microsoft Entra already confirmed.

Run `terraform init -upgrade`, confirm provider `0.7.0` or newer, and apply the
one real registration or metadata change left in the plan. The following plan
should be empty when AZExecute returns the configured values. If real drift is
intentional, use `lifecycle.ignore_changes` on the selected inline attributes,
or omit ownership management entirely instead of hiding all resource changes.

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
