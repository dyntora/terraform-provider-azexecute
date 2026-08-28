data "azexecute_application" "existing" {
  # Stable AZExecute Terraform resource UUID, not an Entra application ID.
  id = "11111111-2222-4333-8444-555555555555"
}

output "application_status" {
  value = data.azexecute_application.existing.status
}

output "application_client_id" {
  value = data.azexecute_application.existing.application_id
}
