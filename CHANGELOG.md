# Changelog

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
