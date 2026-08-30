terraform {
  required_version = ">= 1.8"

  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.7"
    }
  }
}

resource "azexecute_application_request" "deployment" {
  display_name           = "platform-deployment-production"
  description            = "Deployment identity managed through AZExecute"
  business_justification = "Deploys the production platform from the approved CI pipeline."
  project_name           = "Platform"
  department_owner       = "Engineering"
  environment            = "Production"
  business_criticality   = 4
  contact_email          = "platform@example.com"
  owner_object_ids       = ["11111111-2222-4333-8444-555555555555"]

  configure_registration         = true
  sign_in_audience               = "AzureADMyOrg"
  web_redirect_uris              = ["https://platform.example.com/signin-oidc"]
  web_enable_id_token_issuance   = true
  requested_access_token_version = 2
}

output "request_status" {
  value = azexecute_application_request.deployment.status
}

output "client_id" {
  value = azexecute_application_request.deployment.application_id
}
