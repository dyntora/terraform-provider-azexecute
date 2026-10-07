# Changelog

## 0.11.6

- Preserve registered automation ownership automatically and repair missing directory ownership during authorized updates, without requiring the identity in `owner_object_ids`.
- Exclude implicit automation owners from Terraform ownership drift while supporting explicit inclusion and continuing to detect changes to other owners.
- Keep human-owner minimums and access checks enforced. Deploy the matching API and provider together; no database migration is required for this fix.

## 0.11.5

- Return field validation errors when registration editor values are cleared instead of throwing on null values.
- Harden the matching API's recovery flow: keep checkpoints and the Terraform name reserved until deletion of a checkpointed enterprise application is confirmed, and explain when directory propagation requires a retry.
- Refused cleanup leaves completed/imported requests unchanged. Both cleanup actions resolve lost application-creation responses, and Terraform create retries leave administrator-paused requests paused.
- Show retirement completion in the request flow. Deploy the matching API/UI; no additional migration beyond the existing recovery-flow migration is required.

## 0.11.4

- Explain approval rejection, provisioning, and `NeedsAttention` recovery separately, including request IDs and status reasons. Synchronous creation stops polling paused requests while retaining their identity.
- Matching API/UI changes add tenant-admin **Clean up and retire** for incomplete or rejected requests. Verified resources are removed before the request is retired and its Terraform name reservation is released; history is retained and the next refreshed plan can create a replacement.
- Validate owner updates before mutation when they would remove the calling Terraform identity and lock it out. Deploy the matching API/UI and the recovery-flow migration introduced with 0.11.3; upgrading the provider alone is insufficient.

## 0.11.3

- Reject permission values shared by `app_roles` and `exposed_scopes` during planning when values are known, and validate the combined registration before submitting updates. This catches invalid configurations before Microsoft Graph rejects them with a duplicate-values error.
- Matching API changes validate registration settings before provisioning, record request progress and failures, and provide automatic and administrator-triggered cleanup of partial resources. Deploy the matching API and apply its database migration to use these recovery features.

## 0.11.2

- Matching API changes report manually deleted applications as missing instead of leaving completed requests in `Provisioning`. Terraform refresh removes the old resource from state and the next apply submits a fresh request with the same name and new identifiers, subject to current tenant policy.
- Release the deleted request's Terraform name reservation while preserving approval history. Existing completed requests whose application was deleted before this fix are repaired during refresh or replacement creation.
- Cover refresh and recreation for both application resources, and retain state on access-denied and service failures. Deploy the matching API fix; upgrading the provider alone cannot correct stale API responses.

## 0.11.1

- Expose `minimum_additional_owners` in tenant capabilities and validate known owner sets during planning. Unknown owner IDs remain deferred to apply.
- Matching API changes enforce the tenant's additional-owner minimum before creation, updates and owner removal. The original requester, service principals and deleted users do not satisfy the requirement; failures identify `owner_object_ids` and explain how to fix the request.

## 0.11.0

- Show every API validation error with its Terraform field name, nested item index, error code and one correlation reference across application, owner and event-listener operations.
- Send empty registration and permission-request collections as arrays instead of null, avoiding ASP.NET model-validation failures for omitted optional configuration.
- Matching API changes preserve nested validation paths, return all metadata and permission-block issues, and distinguish input errors from dependency timeouts and unexpected server failures.

## 0.10.1

- Expand application-request documentation with app roles and scopes in the same request, a complete exposed-API example, optional pre-authorized clients, and Admin/User consent examples. Clarify approval behavior and the distinction between exposing scopes and requesting downstream API permissions for OBO.

## 0.10.0

- Submit complete registration configuration with application requests, for approval and server-side provisioning without a follow-up Terraform apply.
- Add authoritative `exposed_scopes` and `pre_authorized_applications` to both application resources, including drift detection and safe omission.
- Preserve existing API v1 clients; new request configuration requires `supports_registration_requests`.
- Upgrade application state schemas from versions 0–2 to version 3 without recreating requests or adopting unmanaged scopes.

## 0.9.0

- Adds `azexecute_event_listener` with CRUD, import, drift detection, explicit application/broad scope, automation task mappings, and TOPdesk actions.
- Configures protected secret/certificate runtime references without retrieving credential material into Terraform state.
- Reuses the public event API's ownership, task-access, tenant and approval rules.
- Does not automatically retry listener creation after an ambiguous failure, avoiding duplicate event automation.

## 0.8.0

- Adds authoritative Microsoft Entra app-role definitions through the optional
  `app_roles` set on both application resources.
- Detects manual app-role changes and restores configured display names, token
  values, descriptions, enabled state, and allowed member types.
- Preserves existing app roles when `app_roles` is omitted and supports an
  explicit empty set after roles have first been disabled.

## 0.7.1

- Fixes registration updates failing with `Provider produced inconsistent
  result after apply` when an immediate AZExecute or Microsoft Graph read
  returns the pre-update registration.
- Verifies every managed registration field after updates and retries stale
  reads before committing Terraform state.
- Works with the matching AZExecute API change that returns the confirmed
  Microsoft Entra write result instead of replacing it with a stale replica
  read.

## 0.7.0

- Fixes perpetual update plans by retaining stable computed state during
  in-place changes.
- Fixes HTTP 405 failures during updates by reading the stable resource UUID
  from existing Terraform state rather than an unknown planned value.
- Adds `azexecute_application_owner` for atomic, independently managed owners
  on pending requests and provisioned applications.
- Supports inline authoritative ownership, individual owner resources, or
  unmanaged ownership without unsafe read-modify-write races.
- Documents native Terraform `lifecycle.ignore_changes` behavior and ownership
  mode conflicts.

## 0.6.1

- Adds authoritative application ownership through `owner_object_ids`.
- Refresh detects manual owner changes; apply adds missing owners and removes
  unexpected owners in both Microsoft Entra and AZExecute.
- Expands the Azure DevOps test module to exercise every supported metadata,
  registration, ownership, and permission-request option.

## 0.6.0

- AZExecute application entity identifiers are UUID strings across resources,
  data sources, internal permission targets, and imported state.
- Existing provider state is upgraded automatically without recreating managed
  applications.

## 0.5.0

- Publishes the complete Terraform Registry reference for both application
  resources, both data sources, authentication, Azure DevOps, approval
  workflows, migration, and troubleshooting.
- Documents every metadata, registration, permission-request, wait-control,
  status, and identifier field exposed by the provider.
- Makes `azexecute_application_request` the recommended resource for both
  approval-based and automatic tenant flows.
- Documents live tenant-policy validation and the rule that all metadata is
  optional in Terraform while tenant requirements are enforced dynamically.
- Adds complete Azure DevOps, GitHub Actions, import, migration, automatic-flow,
  and approval-flow examples.
- Adds release checks that reject mismatched tags, stale version references,
  missing Registry front matter, incomplete field coverage, and unexpectedly
  small resource references.

## 0.4.0

- Previous tagged provider release. Upgrade to `0.5.x` for the complete
  version-matched Registry documentation and examples.

## 0.3.0

- Added approval-aware application requests and provider schema improvements.
- Added state-preserving migration from `azexecute_application` to
  `azexecute_application_request`.

## 0.2.0

- Added live Terraform capability and tenant-policy discovery.
- Added governed metadata and registration configuration.

## 0.1.0

- Initial public provider release.
