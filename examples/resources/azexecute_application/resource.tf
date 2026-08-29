terraform {
  required_providers {
    azexecute = {
      source  = "dyntora/azexecute"
      version = "~> 0.6"
    }
  }
}

resource "azexecute_application" "deployment" {
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

  api_permission_request {
    target_type                      = "ExternalApi"
    target_external_api_app_id       = "00000003-0000-0000-c000-000000000000"
    target_external_api_display_name = "Microsoft Graph"
    grant_type                       = "AppRole"
    justification                    = "Reads the approved directory inventory during deployment."

    permission {
      id                     = "7ab1d382-f21e-4acd-a863-ba3e13f7da61"
      display_name           = "Directory.Read.All"
      value                  = "Directory.Read.All"
      requires_admin_consent = true
    }
  }
}

output "client_id" { value = azexecute_application.deployment.application_id }
