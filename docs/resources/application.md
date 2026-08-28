---
page_title: "azexecute_application Resource"
description: |-
  Creates a governed application registration with metadata and permission requests.
---

# azexecute_application

Creates an AZExecute application request and waits for approval/provisioning. App names, descriptions, and permission-request blocks are replacement-only in v0.1; metadata and supported registration fields update in place.

Important fields:

- `display_name` and `business_justification` are required.
- metadata requirements configured by the tenant are enforced by the API.
- `api_permission_request` targets either `ExternalApi` by Entra application ID or `InternalApplication` by AZExecute application entity ID.
- `grant_type` is `AppRole`, `DelegatedScope`, or `AuthorizedClient`.
- set `configure_registration = true` to manage account type, identifier URIs, redirect URIs, token issuance, fallback public client, and requested access-token version.
- `create_timeout_minutes` controls how long the provider waits for approval and provisioning.

The exported `application_id` is the Entra client ID; `application_object_id` is the Entra object ID. Import uses the stable AZExecute Terraform resource UUID, not either Entra ID:

```shell
terraform import azexecute_application.example 11111111-2222-4333-8444-555555555555
```
