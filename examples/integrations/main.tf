terraform {
  required_version = ">= 1.8"
  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.13.6"
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
  private_key_pem = var.github_private_key_pem
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
