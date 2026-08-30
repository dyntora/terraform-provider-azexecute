terraform {
  required_version = ">= 1.8"

  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.8"
    }
  }
}

# Change the old azexecute_application resource to the approval-aware resource.
resource "azexecute_application_request" "this" {
  display_name = "platform-deployment-production"
}

# Provider 0.5 migrates the state without deleting or recreating the remote app.
moved {
  from = azexecute_application.this
  to   = azexecute_application_request.this
}
