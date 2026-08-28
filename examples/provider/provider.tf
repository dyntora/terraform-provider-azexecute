terraform {
  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.1"
    }
  }
}

provider "azexecute" {
  tenant_id = var.tenant_id
  client_id = var.client_id
  scope     = var.azexecute_scope
}

variable "tenant_id" { type = string }
variable "client_id" { type = string }
variable "azexecute_scope" { type = string }
