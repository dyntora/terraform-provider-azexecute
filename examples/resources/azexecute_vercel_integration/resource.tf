variable "vercel_access_token" {
  type      = string
  sensitive = true
}

resource "azexecute_vercel_integration" "production" {
  name         = "Production Vercel"
  team_id      = "team_example"
  team_slug    = "platform"
  access_token = var.vercel_access_token
}
