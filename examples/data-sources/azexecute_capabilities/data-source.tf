terraform {
  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.8"
    }
  }
}

data "azexecute_capabilities" "current" {}

output "terraform_application_creation_enabled" {
  value = data.azexecute_capabilities.current.allow_application_creation
}

output "required_metadata_fields" {
  value = data.azexecute_capabilities.current.required_metadata_fields
}
