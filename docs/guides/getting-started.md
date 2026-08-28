---
page_title: "AZExecute Provider: Getting Started"
subcategory: "Guides"
description: |-
  Configure a tenant and create the first governed application request.
---

# Getting Started

This guide creates an AZExecute application request using the recommended
approval-aware resource.

## 1. Configure the Tenant

In **Tenant administration → Application Configuration → Terraform**:

1. Enable the Terraform API.
2. Enable application creation.
3. Choose whether application requests require approval.
4. Enable registration configuration or API-permission requests only when they
   are needed.
5. Configure which metadata fields are included and required.

Assign the caller `User`, `Operator`, or `TenantAdmin` on the AZExecute
enterprise application. Use the least-privileged role that satisfies the
automation.

## 2. Configure Authentication

For CI/CD, workload identity federation is recommended. The provider needs the
customer tenant ID, the automation application's client ID, and a short-lived
OIDC assertion. It exchanges that assertion for an AZExecute API token.

For a local test after `az login`, set the customer tenant explicitly when
needed:

```shell
export AZEXECUTE_TENANT_ID="00000000-0000-0000-0000-000000000000"
```

See [Authentication](authentication.md) for all supported methods.

## 3. Create the Configuration

```terraform
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

data "azexecute_capabilities" "current" {}

resource "azexecute_application_request" "example" {
  display_name = "platform-deployment-production"
  description  = "Deployment identity managed through Terraform"

  # Add only the metadata required or enabled by your tenant.
  project_name  = "Platform"
  environment   = "Production"
  contact_email = "platform@example.com"
}

output "terraform_policy" {
  value = data.azexecute_capabilities.current
}

output "request_status" {
  value = azexecute_application_request.example.status
}

output "application_client_id" {
  value = azexecute_application_request.example.application_id
}
```

## 4. Initialize and Apply

```shell
terraform init
terraform plan
terraform apply
```

An automatic tenant may return `Ready` immediately or return `Provisioning`
while the background workflow completes. An approval tenant returns
`PendingApproval`. Both are successful Terraform applies.

When the result is not yet `Ready`, approve the request if necessary, wait for
AZExecute provisioning, and run Terraform again. The second run refreshes the
status and applies configured registration settings once the application is
ready.

## 5. Read the Outputs

`application_id`, `application_object_id`, and `application_entity_id` remain
null until provisioning has completed. `request_id` is available after the
request is accepted by AZExecute.

## Next Steps

- Configure [approval workflows](approval-workflows.md).
- Add registration settings using the
  `azexecute_application_request` resource reference.
- Add external or internal API-permission requests.
- Use the [Azure DevOps guide](azure-devops.md) for secretless automation.
