terraform {
  required_providers {
    azexecute = {
      source = "dyntora/azexecute"
    }
  }
}

data "azexecute_capabilities" "current" {}

output "terraform_application_creation_enabled" {
  value = data.azexecute_capabilities.current.allow_application_creation
}
