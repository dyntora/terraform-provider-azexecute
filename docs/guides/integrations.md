---
page_title: "Integrations and groups - AZExecute"
subcategory: "Guides"
description: |-
  Create AWS, Vercel, and GitHub connections and link them with applications in new or existing groups.
---

# Integrations and groups

Provider 0.13.5 supports `azexecute_aws_integration`, `azexecute_vercel_integration`, and `azexecute_github_integration`. Each has a matching data source for existing connections. The resources manage connections in AZExecute; they do not create external accounts, Vercel tokens, or GitHub Apps.

Enable the Terraform API and the Integrations license feature. The caller must be registered in the tenant. Creators can manage their own connections; TenantAdmin can manage existing connections too. Group assignments require group ownership and access to the integration. Applications still follow application permissions, tenant metadata requirements, and approval policy. Operator alone does not grant group ownership or integration administration.

## AWS setup

Read the issuer and audience from AZExecute, create an IAM OIDC provider, and configure a role trust policy. If the AWS account already has this OIDC provider, import or reference it rather than creating a duplicate. The AWS provider and AZExecute provider authenticate independently.

```hcl
terraform {
  required_version = ">= 1.8"
  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.13.7"
    }
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.81"
    }
  }
}

provider "azexecute" {}
provider "aws" {
  region = "eu-west-1"
}

data "azexecute_aws_federation" "current" {}
data "aws_caller_identity" "current" {}

resource "aws_iam_openid_connect_provider" "azexecute" {
  url            = data.azexecute_aws_federation.current.provider_url
  client_id_list = [data.azexecute_aws_federation.current.audience]
}

resource "aws_iam_role" "azexecute" {
  name = "azexecute-production-secrets"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Federated = aws_iam_openid_connect_provider.azexecute.arn
      }
      Action = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "${trimprefix(data.azexecute_aws_federation.current.provider_url, "https://")}:aud" = data.azexecute_aws_federation.current.audience
        }
      }
    }]
  })
}

variable "secret_arns" {
  description = "Existing Secrets Manager secrets this connection may update."
  type        = set(string)
}

resource "aws_iam_role_policy" "secrets" {
  role = aws_iam_role.azexecute.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:DescribeSecret", "secretsmanager:GetSecretValue", "secretsmanager:PutSecretValue"]
      Resource = var.secret_arns
    }]
  })
}

resource "azexecute_aws_integration" "production" {
  name           = "Production Secrets Manager"
  service        = "secrets_manager"
  account_id     = data.aws_caller_identity.current.account_id
  role_arn       = aws_iam_role.azexecute.arn
  default_region = "eu-west-1"
  global         = false

  depends_on = [aws_iam_role_policy.secrets]
}
```

Tailor the IAM permissions to the actions you use. The example permits updating selected existing secrets; creating secrets, listing resources, or using customer-managed KMS keys can require additional permissions. For Parameter Store or Route 53, use `service = "parameter_store"` or `service = "route53"` and the corresponding scoped IAM permissions. Each AWS account/service pair has its own connection; multiple service connections can share one IAM role. `external_id` is retained for compatibility but is not sent by the OIDC assume-role flow.

AZExecute verifies the connection before saving. If AWS IAM propagation has not completed, wait and retry; inspect any reported recovery UUID first. Successful role assumption does not prove every service action is permitted. See the [AWS OIDC role documentation](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-idp_oidc.html) and the [AWS provider OIDC reference](https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/iam_openid_connect_provider).

## Link an application request, group, and integration

Add this to the AWS example above. Include the calling automation identity among the owners when required for application assignment, or run as TenantAdmin. Supply tenant-required application metadata in the request as usual.

```hcl
variable "owner_object_ids" {
  type = set(string)
}

resource "azexecute_application_request" "production" {
  display_name        = "Production API"
  description         = "Production service identity"
  owner_object_ids    = var.owner_object_ids
  wait_for_completion = true
}

resource "azexecute_azgroup" "production" {
  name = "Production platform"
}

resource "azexecute_azgroup_membership" "application" {
  group_id  = azexecute_azgroup.production.group_id
  type      = "application"
  target_id = azexecute_application_request.production.application_entity_id
}

resource "azexecute_azgroup_membership" "aws" {
  group_id  = azexecute_azgroup.production.group_id
  type      = "integration"
  target_id = tostring(azexecute_aws_integration.production.integration_id)
}
```

Waiting allows Terraform to use `application_entity_id` once provisioning reaches `Ready`. It does not grant approval. Human approval must still occur within the configured completion timeout. A timeout during initial creation can taint the retained resource; after approval, inspect a fresh plan and follow the [recovery guidance](troubleshooting.md#deletion-is-waiting-for-entra) before applying so an existing application is not unintentionally replaced. The default group source follows tenant group policy.

The group links make the integration available according to AZExecute access rules. They do not configure an application's secret destinations, Vercel environment variables, or GitHub workflow actions automatically. Configure those application settings separately.

Vercel and GitHub use the same membership pattern:

```hcl
variable "vercel_access_token" {
  type      = string
  sensitive = true
}
variable "github_private_key_pem" {
  type      = string
  sensitive = true
}

resource "azexecute_vercel_integration" "production" {
  name         = "Production Vercel"
  team_id      = "team_example"
  access_token = var.vercel_access_token
}

resource "azexecute_github_integration" "production" {
  name            = "Production GitHub"
  app_id          = "123456"
  installation_id = "789012"
  private_key_pem  = var.github_private_key_pem
}

resource "azexecute_azgroup_membership" "vercel" {
  group_id  = azexecute_azgroup.production.group_id
  type      = "integration"
  target_id = tostring(azexecute_vercel_integration.production.integration_id)
}

resource "azexecute_azgroup_membership" "github" {
  group_id  = azexecute_azgroup.production.group_id
  type      = "integration"
  target_id = tostring(azexecute_github_integration.production.integration_id)
}
```

Vercel requires an existing team and a token with team access. GitHub requires an installed GitHub App and its private key, with repository permissions appropriate to the intended tasks. GitHub Enterprise endpoints are not supported by these resources. See the [Vercel](../resources/vercel_integration.md) and [GitHub](../resources/github_integration.md) references.

## Reuse existing groups and connections

```hcl
data "azexecute_azgroup" "existing" {
  group_id = 12
}

data "azexecute_aws_integration" "existing" {
  integration_id = 42
}

resource "azexecute_azgroup_membership" "existing" {
  group_id  = data.azexecute_azgroup.existing.group_id
  type      = "integration"
  target_id = tostring(data.azexecute_aws_integration.existing.integration_id)
}
```

Data sources leave lifecycle ownership outside this configuration. To manage an existing connection's settings and deletion, declare the corresponding resource and import its numeric AZExecute ID. Do not manage the same integration or membership from more than one state.

## Credentials, recovery, and deletion

Credentials are marked sensitive but remain in state when configured. Protect state with encryption and access controls. Reads and data sources do not expose stored credentials; import can retain them without re-entering them. Supply a replacement token or key to rotate it. Out-of-band credential changes cannot be detected by refresh. Optional secret removal preserves the saved value; an explicit empty string clears an optional secret.

An interrupted create reports a recovery UUID. Inspect or import that UUID before retrying so a completed server write is not duplicated. Access denial and unconfirmed responses retain state. Destroy requires all group memberships to be removed first, and Terraform orders referenced memberships correctly. Destroying an integration does not delete its external IAM role, team, App, or account.
