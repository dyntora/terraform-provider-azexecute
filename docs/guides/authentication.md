---
page_title: "AZExecute Provider: Authentication"
subcategory: "Guides"
description: |-
  Authenticate the provider with OIDC, certificates, managed identity, secrets, or developer credentials.
---

# Authentication

The provider authenticates to Microsoft Entra in the customer tenant and
requests an AZExecute token for `https://api.azexecute.com/.default`. The user or
service principal must also have an AZExecute enterprise-application role.

## Recommended Order

1. Workload identity federation for CI/CD.
2. Managed identity for Azure-hosted automation.
3. Client certificate.
4. Client secret only when federation and certificates are unavailable.

Never commit assertions, access tokens, certificate passwords, or client
secrets to Terraform configuration or variable files.

## Workload Identity Federation

Set:

```shell
AZEXECUTE_TENANT_ID=00000000-0000-0000-0000-000000000000
AZEXECUTE_CLIENT_ID=11111111-1111-1111-1111-111111111111
AZEXECUTE_OIDC_TOKEN=<short-lived-assertion>
```

The provider enables OIDC automatically when an assertion is present. A
rotating assertion file can be supplied with
`AZEXECUTE_OIDC_TOKEN_FILE_PATH`. Azure DevOps and GitHub examples are covered
below and in the [Azure DevOps guide](azure-devops.md).

## GitHub Actions

Create a federated credential on the customer automation application for the
repository, branch, or GitHub environment. Then grant the workflow permission
to request an ID token:

```yaml
permissions:
  contents: read
  id-token: write

env:
  AZEXECUTE_TENANT_ID: ${{ vars.AZEXECUTE_TENANT_ID }}
  AZEXECUTE_CLIENT_ID: ${{ vars.AZEXECUTE_CLIENT_ID }}
```

The provider detects GitHub's OIDC environment and requests an assertion using
the `api://AzureADTokenExchange` audience. Set `AZEXECUTE_OIDC_AUDIENCE` only
when the federated credential expects a different audience. `azure/login` and
an Azure subscription are not required solely for AZExecute authentication.

## Client Certificate

Set:

```shell
AZEXECUTE_TENANT_ID=00000000-0000-0000-0000-000000000000
AZEXECUTE_CLIENT_ID=11111111-1111-1111-1111-111111111111
AZEXECUTE_CLIENT_CERTIFICATE_PATH=/secure/path/automation.pem
```

PEM and PKCS#12/PFX files containing the private key are supported. For an
encrypted PKCS#12/PFX file, set `AZEXECUTE_CLIENT_CERTIFICATE_PASSWORD`. Set
`send_certificate_chain = true` only when subject-name/issuer authentication
requires the chain.

## Client Secret

Set `AZEXECUTE_TENANT_ID`, `AZEXECUTE_CLIENT_ID`, and
`AZEXECUTE_CLIENT_SECRET`. Treat the secret as a fallback and store it in the
CI/CD secret manager rather than Terraform state or `.tfvars` files.

## Managed Identity

Set `AZEXECUTE_USE_MANAGED_IDENTITY=true`. Set `AZEXECUTE_CLIENT_ID` to select a
user-assigned identity; omit it for the system-assigned identity. The managed
identity's service principal must be registered in the customer tenant and
assigned an AZExecute role.

## Azure Default Credential Chain

When no explicit method is selected, the provider uses Azure
`DefaultAzureCredential`. This supports Azure workload identity, managed
identity, Azure CLI, Azure Developer CLI, Azure PowerShell, and supported
developer credentials. `tenant_id` constrains tenant-aware developer
credentials.

## Pre-acquired Token

Set `AZEXECUTE_ACCESS_TOKEN` to use an existing bearer token. The provider does
not refresh a static token, so this is best suited to short operations and
diagnostics.

## Credential Precedence

A higher-precedence method wins when more than one is configured:

1. access token;
2. OIDC;
3. certificate;
4. client secret;
5. explicit managed identity;
6. default credential chain.

Remove unintended environment variables when diagnosing why a different
credential was selected.
