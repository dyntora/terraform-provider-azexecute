variable "group_id" { type = number }
variable "application_entity_id" { type = string }

resource "azexecute_azgroup_membership" "application" {
  group_id  = var.group_id
  type      = "application"
  target_id = var.application_entity_id
}
