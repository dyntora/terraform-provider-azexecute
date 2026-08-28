#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

fail() {
  echo "release documentation check failed: $*" >&2
  exit 1
}

version="$(tr -d '[:space:]' < VERSION)"
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "VERSION must contain a semantic version"
minor_version="${version%.*}"

if [[ "${GITHUB_REF_TYPE:-}" == "tag" ]]; then
  [[ "${GITHUB_REF_NAME:-}" == "v$version" ]] || fail "tag ${GITHUB_REF_NAME:-<missing>} does not match VERSION v$version"
fi

required_files=(
  README.md
  CHANGELOG.md
  docs/index.md
  docs/resources/application.md
  docs/resources/application_request.md
  docs/data-sources/application.md
  docs/data-sources/capabilities.md
  docs/guides/getting-started.md
  docs/guides/authentication.md
  docs/guides/azure-devops.md
  docs/guides/approval-workflows.md
  docs/guides/migration-v0.5.md
  docs/guides/troubleshooting.md
  examples/authentication/azure-devops.yml
  examples/authentication/github-actions.yml
  examples/migration/moved.tf
  examples/resources/azexecute_application/resource.tf
  examples/resources/azexecute_application_request/resource.tf
)

for file in "${required_files[@]}"; do
  [[ -s "$file" ]] || fail "required release file is missing or empty: $file"
done

for doc in docs/index.md docs/resources/*.md docs/data-sources/*.md docs/guides/*.md; do
  head -n 1 "$doc" | grep -qx -- '---' || fail "$doc has no Terraform Registry front matter"
  grep -q '^page_title:' "$doc" || fail "$doc has no page_title"
  grep -q '^description:' "$doc" || fail "$doc has no description"
done

provider_fields=(
  endpoint tenant_id client_id client_secret client_certificate_path
  client_certificate_password send_certificate_chain access_token use_oidc
  oidc_token oidc_token_file_path oidc_audience use_managed_identity scope
  request_timeout_seconds
)

resource_fields=(
  id display_name description business_justification technical_requirements
  intended_audience data_access_requirements compliance_notes
  expected_go_live_date project_name department_owner business_criticality
  requires_elevated_permissions elevated_permissions_justification environment
  contact_email contact_phone configure_registration sign_in_audience
  is_fallback_public_client identifier_uris web_home_page_url web_logout_url
  web_enable_access_token_issuance web_enable_id_token_issuance
  web_redirect_uris spa_redirect_uris public_client_redirect_uris
  requested_access_token_version status status_reason request_id
  application_entity_id application_id application_object_id
  api_permission_request target_type target_application_entity_id
  target_external_api_app_id target_external_api_display_name grant_type
  justification permission value requires_admin_consent
)

capability_fields=(
  id api_version enabled allow_application_creation allow_application_deletion
  allow_api_permission_requests allow_registration_configuration
  use_application_request_flow use_api_permission_request_flow
  included_metadata_fields required_metadata_fields
)

application_data_fields=(
  id display_name description status status_reason request_id
  application_entity_id application_id application_object_id
  business_justification
)

for field in "${provider_fields[@]}"; do
  grep -Fq "\`$field\`" docs/index.md || fail "provider argument $field is missing from docs/index.md"
done

for doc in docs/resources/application.md docs/resources/application_request.md; do
  for field in "${resource_fields[@]}"; do
    grep -Fq "\`$field\`" "$doc" || fail "resource field $field is missing from $doc"
  done
done

for field in poll_interval_seconds create_timeout_minutes; do
  grep -Fq "\`$field\`" docs/resources/application.md || fail "synchronous resource field $field is undocumented"
done

for field in "${capability_fields[@]}"; do
  grep -Fq "\`$field\`" docs/data-sources/capabilities.md || fail "capability field $field is undocumented"
done

for field in "${application_data_fields[@]}"; do
  grep -Fq "\`$field\`" docs/data-sources/application.md || fail "application data field $field is undocumented"
done

grep -Fq "version = \"~> $minor_version\"" docs/index.md || fail "provider example does not use $minor_version"
grep -Fq "version = \"~> $minor_version\"" examples/provider/provider.tf || fail "provider example does not use $minor_version"
grep -Fq 'NewApplicationRequestResource' internal/provider/provider.go || fail "approval-aware resource is not registered"
grep -Fq '"_application_request"' internal/provider/application_request_resource.go || fail "approval-aware resource type name is missing"
grep -Fq 'azexecute_application_request' docs/resources/application_request.md || fail "approval-aware resource documentation is missing"

if grep -R -n -E 'version = "~> 0\.[0-4]|Provider `0\.[0-4]`|Provider version 0\.[0-4]|migration-v0\.[0-4]' README.md docs examples; then
  fail "stale pre-0.5 release references remain"
fi

request_lines="$(wc -l < docs/resources/application_request.md)"
application_lines="$(wc -l < docs/resources/application.md)"
[[ "$request_lines" -ge 200 ]] || fail "application request reference is unexpectedly small ($request_lines lines)"
[[ "$application_lines" -ge 200 ]] || fail "application reference is unexpectedly small ($application_lines lines)"

echo "release documentation checks passed for v$version"
