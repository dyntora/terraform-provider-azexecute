package client

import "encoding/json"

// ASP.NET validates non-null collection properties before the controller runs.
// A nil Go slice must therefore be sent as an empty array, not JSON null.
func (r ApplicationCreate) MarshalJSON() ([]byte, error) {
	type wire ApplicationCreate
	r.APIPermissionRequests = nonNilSlice(r.APIPermissionRequests)
	return json.Marshal(wire(r))
}

func (r RegistrationConfiguration) MarshalJSON() ([]byte, error) {
	type wire RegistrationConfiguration
	r.IdentifierUris = nonNilSlice(r.IdentifierUris)
	r.AppRoles = nonNilSlice(r.AppRoles)
	r.Web.RedirectUris = nonNilSlice(r.Web.RedirectUris)
	r.Spa.RedirectUris = nonNilSlice(r.Spa.RedirectUris)
	r.PublicClient.RedirectUris = nonNilSlice(r.PublicClient.RedirectUris)
	r.API.Scopes = nonNilSlice(r.API.Scopes)
	r.API.PreAuthorizedApplications = nonNilSlice(r.API.PreAuthorizedApplications)
	return json.Marshal(wire(r))
}

func nonNilSlice[T any](value []T) []T {
	if value == nil {
		return []T{}
	}
	return value
}
