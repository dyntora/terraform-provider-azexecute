terraform {
  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.8"
    }
  }
}

provider "azexecute" {}

resource "azexecute_application_request" "example" {
  display_name = "platform-production"
}

locals {
  owner_object_ids = toset([
    "11111111-2222-4333-8444-555555555555",
    "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee",
  ])
}

resource "azexecute_application_owner" "example" {
  for_each = local.owner_object_ids

  application_resource_id = azexecute_application_request.example.id
  owner_object_id         = each.value
}
