# AZExecute Terraform Provider

The public `dyntora/azexecute` provider creates and manages governed Microsoft
Entra application registrations through the dedicated AZExecute Terraform API.
It respects the caller's AZExecute role and the tenant's live application,
metadata, permission, registration, approval, and deletion settings on every
operation.

Provider `0.5` supports:

- approval-aware asynchronous application requests;
- synchronous application creation for automatic tenants;
- tenant-driven application metadata;
- external and internal API-permission requests;
- account audience, identifier URIs, redirect URIs, token issuance, fallback
  public-client behavior, and requested access-token version;
- import, drift reads, state-preserving migration, and controlled deletion;
- tenant-capability and application data sources;
- OIDC, certificate, secret, managed-identity, developer, and static-token
  authentication.

Terraform does not elevate the caller or bypass approval. `User` identities can
manage only the Terraform resources they created. `Operator` and `TenantAdmin`
identities can manage Terraform resources throughout their AZExecute tenant.

## Documentation

The complete Registry-facing documentation is maintained with the provider:

- [Provider configuration](docs/index.md)
- [Getting started](docs/guides/getting-started.md)
- [Authentication](docs/guides/authentication.md)
- [Azure DevOps](docs/guides/azure-devops.md)
- [Approval and provisioning workflows](docs/guides/approval-workflows.md)
- [`azexecute_application_request`](docs/resources/application_request.md)
- [`azexecute_application`](docs/resources/application.md)
- [`azexecute_capabilities`](docs/data-sources/capabilities.md)
- [`azexecute_application` data source](docs/data-sources/application.md)
- [Upgrade and state migration](docs/guides/migration-v0.5.md)
- [Troubleshooting](docs/guides/troubleshooting.md)

After publication, the same reference is available at the
[Terraform Registry](https://registry.terraform.io/providers/dyntora/azexecute/latest/docs).

## Quick Start

```hcl
terraform {
  required_version = ">= 1.8"

  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.5"
    }
  }
}

provider "azexecute" {}

resource "azexecute_application_request" "example" {
  display_name = "platform-deployment-production"
  description  = "Deployment identity managed through Terraform"
}

output "status" {
  value = azexecute_application_request.example.status
}
```

The recommended `azexecute_application_request` resource works with approval
and automatic tenants. `PendingApproval` and `Provisioning` are successful
results: approve or wait for provisioning, then run Terraform again to refresh
the resource and apply registration settings after it reaches `Ready`.

Use `azexecute_application` only when the tenant guarantees automatic
provisioning and Terraform should wait for completion in one apply.

## Tenant Prerequisites

Under **Tenant administration → Application Configuration → Terraform**:

1. enable the Terraform API and application creation;
2. decide whether application and permission requests require approval;
3. configure included and required metadata;
4. enable registration configuration and permission requests only when needed;
5. enable application deletion only for workflows allowed to destroy
   provisioned applications.

Assign the automation identity the minimum required AZExecute enterprise
application role.

## Authentication

Workload identity federation is recommended for CI/CD. The provider accepts a
short-lived OIDC assertion and exchanges it for a token scoped to
`https://api.azexecute.com/.default`. It also supports certificates, client
secrets, managed identity, Azure's default credential chain, and a pre-acquired
access token.

Credentials should be supplied through environment variables or a secret
manager, never committed to Terraform files. See the
[authentication guide](docs/guides/authentication.md) and the complete
[Azure DevOps](examples/authentication/azure-devops.yml) and
[GitHub Actions](examples/authentication/github-actions.yml) examples.

## Tenant-Driven Metadata

Only `display_name` is statically required. All tenant-controlled metadata
arguments are optional in the provider schema. During planning, the provider
reads `required_metadata_fields` from the live capabilities endpoint and
reports missing values. The API validates the same current policy before it
creates or changes anything, so an invalid request does not leave an orphaned
approval item.

## Development

```shell
go test ./...
go build ./...
```

For local Terraform testing, build the provider and configure a Terraform CLI
`dev_overrides` entry as shown in [`.terraformrc.example`](.terraformrc.example).

## Releasing

1. Configure the Terraform Registry signing key and GitHub release secrets.
2. Push an annotated semantic-version tag matching `VERSION`, such as `v0.5.0`.
3. The release workflow tests the provider and publishes signed Windows, Linux,
   and macOS archives plus checksums.
4. The Terraform Registry discovers the tagged release from the public GitHub
   repository.

The provider is licensed under the Mozilla Public License 2.0.
