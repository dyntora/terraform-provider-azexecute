---
page_title: "AZExecute Provider: Application Deployment Validation"
subcategory: "Guides"
description: |-
  Verified lifecycle behavior and remaining live-tenant release checks.
---

# Application deployment validation

Version 0.12.4 has local regression coverage for the application lifecycle,
including role and scope removal. These checks have not been run against a live
Entra tenant; the live-tenant scenarios below remain to be verified.

## Verified locally

The actual Terraform CLI exercises both application resource types against a
local API fixture. This verifies Terraform's plan/state consistency rules, not
Microsoft Graph behavior:

| Scenario | Expected result |
| --- | --- |
| Metadata supplied by another Terraform resource | Unknown plan values resolve at apply; known omissions still fail policy validation |
| Approval/provisioning followed by Ready | Waiting records the resulting registration and identifiers |
| Submit asynchronously, then enable waiting in a later apply | Generated identifier URIs survive approval and the subsequent update |
| Roles, scopes, pre-authorized clients, redirects and owners | Configuration survives apply and subsequent plans |
| Explicitly clear roles, scopes and pre-authorized clients | One in-place apply removes the definitions and the following plan is empty |
| Hidden elevated-permission metadata | Values persist with the updated API; incompatible APIs fail with upgrade guidance |
| Description/name/metadata edits | In-place update retains resource, client and object IDs |
| Owner and app-role drift | Apply restores configured values; subsequent plan is empty |
| UUID casing, trimmed justification, empty optional strings and URI sets | Equivalent representations do not cause inconsistent-state errors |
| API discards metadata after an update | Useful field-level error; corrected API allows in-place recovery |
| Import with API-permission request blocks | Original submission is recovered without recreating the application |
| Destroy followed by another destroy | One deletion; subsequent destroy is a no-op |
| Destroy encounters unconfirmed directory deletion | Cleanup retries the same resource and completes within the same Terraform run |
| Replace a tainted resource while deletion is unconfirmed | Apply waits for cleanup, creates the replacement, and the next plan is empty |

Additional provider tests cover cancellation/timeouts, rejection, stale registration
reads, state upgrades, owner operations, capability restrictions, malformed API
responses, routing 404s, directory failures and authentication assertion sources.
Backend tests cover metadata persistence, tenant policy, provisioning/recreation,
cleanup and endpoint contracts. Graph SDK tests cover automatic disable, edit and
removal of permission definitions, replacement IDs reusing existing values,
propagation retries, concurrency conflicts and interrupted-update recovery.
Optional PostgreSQL integration tests are outside this local run.

A shared JSON fixture is checked against both the Go client and the C# API DTOs
for metadata, registration settings, ownership, original permission submissions
and downstream permission status. The release and pull-request workflows run the
Terraform CLI suite, `go vet`, formatting checks and Go race tests on Linux.

The final local run passed the provider suite (including Terraform CLI 1.16.0),
`go vet`, module integrity, formatting and release-documentation checks. The
backend suite passed 5,184 tests, with two optional PostgreSQL integration tests
skipped. Local Windows race testing was unavailable because no C compiler was
installed; the Linux release workflow must pass its race test before publishing.

Application and individual-owner refresh retain state on unqualified 404s and
malformed responses. Delete requires confirmed HTTP 204 completion. Owner reads
use the parent's policy, access and recovery checks, so inaccessible applications
and live orphaned registrations are not silently treated as missing owners.

## Pipeline setup

Use `azexecute_application_request` with `wait_for_completion = true` when later
resources need its `application_id` in the same apply. Supply required metadata
and human owners inline at creation. Use a stable `for_each` key; changing that
key changes Terraform's resource address unless accompanied by a `moved` block.

For long human approval workflows, submit asynchronously and run a later pipeline
after approval. A static access token or OIDC assertion cannot renew itself;
waiting must fit the pipeline and credential lifetime, or use a renewable
credential source. Keep a shared, locked backend and use a stable automation
identity with the required AZExecute role and application access.

## Deliberate limits

- Metadata, registration settings and ownership can be corrected in place once
  the application is Ready. Pending/rejected requests cannot be freely rewritten
  across the approval boundary.
- Changing `api_permission_request` still plans replacement. These are original
  request blocks, not an authoritative ongoing permission-grant resource. Review
  replacement plans; use `lifecycle { prevent_destroy = true }` where accidental
  replacement must be blocked.
- `Ready` confirms application provisioning, not completion of every downstream
  permission approval or admin-consent operation.
- Import takes an existing AZExecute Terraform resource UUID. It does not adopt
  an arbitrary Entra client/object ID. The updated import requires the API's
  original permission-submission response; older APIs produce upgrade guidance.
- Destruction requires applicable tenant policy and access. Directory failures
  retain state. Explicit pending-deletion and operation-in-progress responses
  are retried for up to five minutes; timeout or other failures require
  retry/recovery and are not hidden as success.
- A failed create can be tainted by Terraform even when the application exists.
  After fixing the underlying failure, inspect the plan and reconcile/import the
  existing resource before approving replacement. Metadata errors alone are not
  a reason to recreate an app with a new client ID.

## Live-tenant release check

With the released API/worker/provider pair, run the customer-style configuration
under the actual pipeline identity: create, wait for Ready, confirm owners and
registration in Entra, confirm downstream permission outcomes separately, run a
no-change plan, edit metadata and a redirect, edit and remove enabled roles and
scopes, repair deliberate test drift, import
the same resource into an isolated recovery state, and destroy the test resource.
Use only one active state owner at a time. Repeat the approval flow in a tenant
that requires approval. Confirm permission-denied and deletion-propagation errors
retain the resource and provide actionable diagnostics.

Terraform's consistency contract is documented in
[HashiCorp's plan modification guidance](https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification).
