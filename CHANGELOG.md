# Changelog

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
