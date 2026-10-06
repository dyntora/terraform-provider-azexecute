package client

import (
	"net/http"
	"regexp"
	"strings"
)

var fieldWordBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)
var fieldAcronymBoundary = regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`)

// Keep indices so repeated nested properties still identify the failing item.
func terraformFieldName(field string) string {
	field = strings.TrimPrefix(field, "$.")
	field = fieldAcronymBoundary.ReplaceAllString(field, "${1}_${2}")
	field = strings.ToLower(fieldWordBoundary.ReplaceAllString(field, "${1}_${2}"))
	field = strings.TrimPrefix(field, "metadata.")
	for _, mapping := range [][2]string{
		{"registration.api.pre_authorized_applications", "pre_authorized_applications"},
		{"registration.api.scopes", "exposed_scopes"},
		{"registration.api.requested_access_token_version", "requested_access_token_version"},
		{"registration.web.", "web_"},
		{"registration.spa.", "spa_"},
		{"registration.public_client.", "public_client_"},
		{"registration.", ""},
		{"api_permission_requests", "api_permission_request"},
	} {
		if strings.HasPrefix(field, mapping[0]) {
			return mapping[1] + strings.TrimPrefix(field, mapping[0])
		}
	}
	return field
}

func defaultErrorDetail(status int) string {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return "The API rejected the supplied configuration. Review the field errors below; if none are provided, ask an administrator to inspect the request reference."
	case http.StatusUnauthorized:
		return "AZExecute could not authenticate the provider. Check the provider tenant, token audience and configured credentials."
	case http.StatusForbidden:
		return "Access was denied. Check the identity's AZExecute role and the tenant's Terraform policy."
	case http.StatusConflict:
		return "The resource conflicts with the current state. Run terraform plan to refresh it and review the requested changes."
	case http.StatusTooManyRequests:
		return "AZExecute is rate limiting requests. Wait before running Terraform again."
	default:
		if status >= 500 {
			return "AZExecute or a connected service could not complete the operation. Refresh the resource state before retrying; a write may have completed."
		}
		return http.StatusText(status)
	}
}
