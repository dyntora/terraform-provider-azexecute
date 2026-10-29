data "azexecute_group_capabilities" "tenant" {}

resource "azexecute_azgroup" "team" {
  name                    = "Platform"
  description             = "Platform team resources"
  source                  = "auto"
  delete_entra_on_destroy = false
}

output "group_id" {
  value = azexecute_azgroup.team.group_id
}
