terraform {
  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.5"
    }
  }
}

# Credentials should normally come from AZEXECUTE_* environment variables or
# Azure's default credential chain. The production endpoint and scope are the
# provider defaults.
provider "azexecute" {}
