variable "github_private_key_pem" {
  type      = string
  sensitive = true
}

resource "azexecute_github_integration" "production" {
  name            = "Production GitHub"
  app_id          = "123456"
  installation_id = "789012"
  private_key_pem = var.github_private_key_pem
}
