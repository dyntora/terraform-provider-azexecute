---
page_title: "AZExecute Provider: Azure DevOps"
subcategory: "Guides"
description: |-
  Run the provider on Microsoft-hosted Azure DevOps agents with workload identity federation.
---

# Azure DevOps

Use an Azure Resource Manager service connection configured with workload
identity federation. The same short-lived Azure DevOps assertion can
authenticate both the Azure Blob backend and the AZExecute provider. Each
consumer exchanges the assertion for its own access token; no AZExecute token or
client secret is stored in the pipeline.

## Prerequisites

1. Create or select an automation application in the customer tenant.
2. Create an Azure Resource Manager service connection using workload identity
   federation for that application.
3. Assign the application's service principal an AZExecute `User`, `Operator`,
   or `TenantAdmin` role.
4. Grant the service principal Blob data access to the Terraform state storage
   account. `Storage Blob Data Contributor` is normally sufficient;
   `Storage Blob Data Owner` also works.
5. Enable the required operations in the tenant's AZExecute Terraform settings.
6. Authorize the pipeline to use the service connection.

The service connection's Azure subscription is used by the `azurerm` backend.
It does not grant any AZExecute permission by itself.

## Required Environment Variables

An `AzureCLI@2` task with `addSpnToEnvironment: true` exposes `idToken`,
`servicePrincipalId`, and `tenantId`. Map them as follows inside that same task:

```powershell
$env:ARM_CLIENT_ID       = $env:servicePrincipalId
$env:ARM_TENANT_ID       = $env:tenantId
$env:ARM_SUBSCRIPTION_ID = (az account show --query id --output tsv)
$env:ARM_USE_OIDC        = 'true'
$env:ARM_OIDC_TOKEN      = $env:idToken
$env:ARM_USE_AZUREAD     = 'true'

$env:AZEXECUTE_CLIENT_ID  = $env:servicePrincipalId
$env:AZEXECUTE_TENANT_ID  = $env:tenantId
$env:AZEXECUTE_USE_OIDC   = 'true'
$env:AZEXECUTE_OIDC_TOKEN = $env:idToken
```

Do not publish, echo, or save `idToken`. Mark it secret when using scripts that
may produce diagnostic output:

```powershell
Write-Host "##vso[task.setsecret]$($env:idToken)"
```

## Minimal Pipeline

The provider repository contains a complete example at
`examples/authentication/azure-devops.yml`. Its essential task is:

```yaml
- task: AzureCLI@2
  displayName: Terraform plan and apply
  inputs:
    azureSubscription: $(azureServiceConnection)
    scriptType: pscore
    scriptLocation: inlineScript
    addSpnToEnvironment: true
    inlineScript: |
      $ErrorActionPreference = 'Stop'
      Write-Host "##vso[task.setsecret]$($env:idToken)"

      $env:ARM_CLIENT_ID = $env:servicePrincipalId
      $env:ARM_TENANT_ID = $env:tenantId
      $env:ARM_SUBSCRIPTION_ID = (az account show --query id --output tsv).Trim()
      $env:ARM_USE_OIDC = 'true'
      $env:ARM_OIDC_TOKEN = $env:idToken
      $env:ARM_USE_AZUREAD = 'true'
      $env:AZEXECUTE_CLIENT_ID = $env:servicePrincipalId
      $env:AZEXECUTE_TENANT_ID = $env:tenantId
      $env:AZEXECUTE_USE_OIDC = 'true'
      $env:AZEXECUTE_OIDC_TOKEN = $env:idToken

      Set-Location '$(Build.SourcesDirectory)/terraform'
      terraform init -input=false `
        -backend-config="storage_account_name=$(stateStorageAccount)" `
        -backend-config="container_name=$(stateContainer)" `
        -backend-config="key=$(stateKey)" `
        -backend-config="use_azuread_auth=true" `
        -backend-config="use_oidc=true"
      terraform fmt -check -recursive
      terraform validate
      terraform plan -input=false -out="$(Agent.TempDirectory)/azexecute.tfplan"
      terraform apply -input=false "$(Agent.TempDirectory)/azexecute.tfplan"
```

Install a pinned Terraform version before this task. The full example downloads
and verifies the HashiCorp release on a Microsoft-hosted Ubuntu agent.

## Approval-Based Runs

With `azexecute_application_request`, the first run can finish successfully with
`status = "PendingApproval"`. After an administrator approves the request and
AZExecute finishes provisioning, run the same pipeline again. The later run
refreshes the identifiers and applies configured registration settings.

The pipeline should therefore be safe to run repeatedly: plan the checked-in
configuration and apply only when Terraform reports a change. It does not need
a queue-time switch for “create” versus “update”.

## Azure Blob Backend

Use an `azurerm` backend with OIDC and Microsoft Entra authorization:

```terraform
terraform {
  backend "azurerm" {}
}
```

Pass storage account, container, key, `use_azuread_auth=true`, and
`use_oidc=true` during `terraform init`. Keep backend coordinates in pipeline
variables; keep application configuration in version-controlled Terraform.

## Operational Guidance

- Save and apply the exact plan created in the same job.
- Use `terraform init -upgrade` deliberately when adopting a new compatible
  provider release.
- Do not publish saved plan files as artifacts; plans can contain sensitive
  values.
- Use a unique backend key for each independently managed application or root
  module.
- Configure an Azure Blob lease/state lock timeout suitable for the pipeline.
- A service connection with Blob access but no AZExecute role can access state
  but cannot manage applications.
