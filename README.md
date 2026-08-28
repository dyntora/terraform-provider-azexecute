# AZExecute Terraform Provider

The AZExecute provider manages the deliberately limited application-registration surface exposed by the dedicated AZExecute Terraform API. Version `0.1` supports:

- idempotent application creation and import;
- tenant-governed application and API-permission approval flows;
- AZExecute application metadata;
- external and internal API permission requests;
- common Entra registration settings (account type, identifier and redirect URIs, token issuance, and public-client settings);
- drift reads and optional deletion.

The provider does not turn AZExecute's general API into a Terraform-shaped API. Every operation is checked against the current tenant application settings. Callers use the same `User`, `Operator`, or `TenantAdmin` role assigned on the AZExecute enterprise application; Terraform grants no additional privilege.

## Requirements

- Terraform 1.0 or later
- a user or service principal assigned an AZExecute enterprise-application role in the customer tenant
- the Terraform API and the desired operations enabled under **Tenant administration → Application Configuration → Terraform**

For automation, use a service principal assigned the minimum required role. `User` identities can manage only the Terraform applications they create. `Operator` and `TenantAdmin` identities can manage Terraform applications across their AZExecute tenant.

## Authentication

The provider accepts credentials in this order:

1. `access_token` / `AZEXECUTE_ACCESS_TOKEN`;
2. workload identity federation using an OIDC assertion, rotating assertion file, or the native GitHub Actions OIDC environment;
3. a client certificate;
4. a client secret;
5. an explicitly selected managed identity, or Azure's default credential chain for workload identity, managed identity, Azure CLI, Azure Developer CLI, Azure PowerShell, and supported developer credentials.

The automation application registration belongs to the customer tenant. Configure a federated credential, certificate, or secret on it, then assign its service principal the minimum AZExecute enterprise-application role. All methods exchange that customer credential for an AZExecute API token scoped to `https://api.azexecute.com/.default`. Secrets and tokens should be supplied through environment variables or a secret manager, not checked into Terraform files.

```hcl
terraform {
  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.1"
    }
  }
}

provider "azexecute" {
  tenant_id = var.tenant_id
  client_id = var.client_id
  # client_secret is normally supplied as AZEXECUTE_CLIENT_SECRET
  scope = "https://api.azexecute.com/.default"
}
```

### GitHub Actions without a stored secret

Add a federated identity credential to the customer's automation app registration for the repository, branch, or GitHub environment. Assign its service principal an AZExecute role. The provider detects GitHub's `ACTIONS_ID_TOKEN_REQUEST_URL` and `ACTIONS_ID_TOKEN_REQUEST_TOKEN` variables and performs the Azure client-assertion exchange itself; `azure/login` and an Azure subscription are not required.

```yaml
permissions:
  contents: read
  id-token: write

env:
  AZEXECUTE_TENANT_ID: ${{ vars.AZEXECUTE_TENANT_ID }}
  AZEXECUTE_CLIENT_ID: ${{ vars.AZEXECUTE_CLIENT_ID }}

steps:
  - uses: actions/checkout@v4
  - uses: hashicorp/setup-terraform@v3
  - run: terraform init
  - run: terraform apply -auto-approve
```

The default GitHub assertion audience is `api://AzureADTokenExchange`. Override it only when the customer's Entra federated credential uses a different audience. See [`examples/authentication/github-actions.yml`](examples/authentication/github-actions.yml).

### Certificate authentication

Set `AZEXECUTE_TENANT_ID`, `AZEXECUTE_CLIENT_ID`, and `AZEXECUTE_CLIENT_CERTIFICATE_PATH`. PEM and PKCS#12/PFX certificates containing the private key are supported. Set `AZEXECUTE_CLIENT_CERTIFICATE_PASSWORD` for an encrypted PKCS#12 file. Certificates are preferred over secrets when federation is unavailable.

See [`examples/resources/azexecute_application/resource.tf`](examples/resources/azexecute_application/resource.tf) for a complete application with metadata, registration configuration, and Microsoft Graph permissions.

## Approval behavior

Create waits until AZExecute reports `Ready`. When application approval is enabled it remains in `PendingApproval`; after approval, normal AZExecute background provisioning creates and reconciles the Entra registration. Permission requests are either left in the normal permission request queue or applied automatically according to the tenant's Terraform permission-flow setting.

The create request uses a provider-generated UUID and the API enforces a tenant-scoped unique index, so a retry after a timeout or lost response cannot create a duplicate application.

## Development

```shell
go test ./...
go build ./...
```

For local Terraform testing, build the provider and configure a Terraform CLI `dev_overrides` entry as shown in [`.terraformrc.example`](.terraformrc.example).

## Releasing 0.1

1. Create the public GitHub repository `dyntora/terraform-provider-azexecute`.
2. Add a GPG signing key to the Terraform Registry and configure the GitHub secrets `GPG_PRIVATE_KEY` and `PASSPHRASE`.
3. Push an annotated semantic-version tag such as `v0.1.0`.
4. The release workflow tests all packages and GoReleaser publishes signed Windows, Linux, and macOS archives plus checksums.
5. Sign in to the Terraform Registry, choose **Publish provider**, and select the GitHub repository. Later tags are discovered automatically.

The provider is licensed under the Mozilla Public License 2.0.
