resource "azexecute_aws_integration" "production" {
  name           = "Production secrets"
  service        = "secrets_manager"
  account_id     = "123456789012"
  role_arn       = "arn:aws:iam::123456789012:role/azexecute"
  default_region = "eu-west-1"
  global         = false
}
