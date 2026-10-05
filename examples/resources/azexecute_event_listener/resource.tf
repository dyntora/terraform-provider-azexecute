terraform {
  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.10"
    }
  }
}

provider "azexecute" {}

# Use the provisioned application's AZExecute entity UUID and an existing task ID.
resource "azexecute_event_listener" "renewal" {
  name                  = "Deploy renewed secret"
  application_entity_id = "11111111-2222-4333-8444-555555555555"
  event_type            = "ApplicationCredentialRenewed"
  credential_type       = "Secret"
  automation_task_id    = 42
  parameters = {
    DeploymentInput = "{{ renewed_secret }}"
    ApplicationName = "{{ application_name }}"
  }
}
