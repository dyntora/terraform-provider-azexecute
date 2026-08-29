package client

import "time"

type Capabilities struct {
	APIVersion                     string   `json:"apiVersion"`
	Enabled                        bool     `json:"enabled"`
	AllowApplicationCreation       bool     `json:"allowApplicationCreation"`
	AllowApplicationDeletion       bool     `json:"allowApplicationDeletion"`
	AllowAPIPermissionRequests     bool     `json:"allowApiPermissionRequests"`
	AllowRegistrationConfiguration bool     `json:"allowRegistrationConfiguration"`
	UseApplicationRequestFlow      bool     `json:"useApplicationRequestFlow"`
	UseAPIPermissionRequestFlow    bool     `json:"useApiPermissionRequestFlow"`
	IncludedMetadataFields         []string `json:"includedMetadataFields"`
	RequiredMetadataFields         []string `json:"requiredMetadataFields"`
}

type ApplicationCreate struct {
	ResourceID            string                 `json:"resourceId"`
	DisplayName           string                 `json:"displayName"`
	Description           *string                `json:"description,omitempty"`
	Metadata              ApplicationMetadata    `json:"metadata"`
	APIPermissionRequests []APIPermissionRequest `json:"apiPermissionRequests"`
}

type ApplicationUpdate struct {
	Metadata     ApplicationMetadata        `json:"metadata"`
	Registration *RegistrationConfiguration `json:"registration,omitempty"`
}

type ApplicationMetadata struct {
	BusinessJustification            string     `json:"businessJustification"`
	TechnicalRequirements            *string    `json:"technicalRequirements,omitempty"`
	IntendedAudience                 *string    `json:"intendedAudience,omitempty"`
	DataAccessRequirements           *string    `json:"dataAccessRequirements,omitempty"`
	ComplianceNotes                  *string    `json:"complianceNotes,omitempty"`
	ExpectedGoLiveDate               *time.Time `json:"expectedGoLiveDate,omitempty"`
	ProjectName                      *string    `json:"projectName,omitempty"`
	DepartmentOwner                  *string    `json:"departmentOwner,omitempty"`
	BusinessCriticality              int64      `json:"businessCriticality"`
	RequiresElevatedPermissions      bool       `json:"requiresElevatedPermissions"`
	ElevatedPermissionsJustification *string    `json:"elevatedPermissionsJustification,omitempty"`
	Environment                      *string    `json:"environment,omitempty"`
	ContactEmail                     *string    `json:"contactEmail,omitempty"`
	ContactPhone                     *string    `json:"contactPhone,omitempty"`
}

type APIPermissionRequest struct {
	TargetType                   string          `json:"targetType"`
	TargetApplicationEntityID    *string         `json:"targetApplicationEntityId,omitempty"`
	TargetExternalAPIAppID       *string         `json:"targetExternalApiAppId,omitempty"`
	TargetExternalAPIDisplayName *string         `json:"targetExternalApiDisplayName,omitempty"`
	GrantType                    string          `json:"grantType"`
	Justification                *string         `json:"justification,omitempty"`
	Permissions                  []APIPermission `json:"permissions"`
}

type APIPermission struct {
	ID                   string  `json:"id"`
	DisplayName          *string `json:"displayName,omitempty"`
	Value                *string `json:"value,omitempty"`
	RequiresAdminConsent bool    `json:"requiresAdminConsent"`
}

type PermissionRequestStatus struct {
	ID                    int64  `json:"id"`
	Status                string `json:"status"`
	TargetType            string `json:"targetType"`
	GrantType             string `json:"grantType"`
	RequestedPermissionID string `json:"requestedPermissionId"`
}

type Application struct {
	ResourceID            string                     `json:"resourceId"`
	RequestID             int64                      `json:"requestId"`
	Status                string                     `json:"status"`
	StatusReason          *string                    `json:"statusReason,omitempty"`
	ApplicationEntityID   *string                    `json:"applicationEntityId,omitempty"`
	ApplicationID         *string                    `json:"applicationId,omitempty"`
	ApplicationObjectID   *string                    `json:"applicationObjectId,omitempty"`
	DisplayName           string                     `json:"displayName"`
	Description           *string                    `json:"description,omitempty"`
	Metadata              ApplicationMetadata        `json:"metadata"`
	Registration          *RegistrationConfiguration `json:"registration,omitempty"`
	APIPermissionRequests []PermissionRequestStatus  `json:"apiPermissionRequests"`
	CreatedOn             *time.Time                 `json:"createdOn,omitempty"`
}

type RegistrationConfiguration struct {
	ApplicationID          string                `json:"applicationId"`
	ApplicationObjectID    string                `json:"applicationObjectId"`
	DisplayName            string                `json:"displayName"`
	SignInAudience         string                `json:"signInAudience"`
	IsFallbackPublicClient bool                  `json:"isFallbackPublicClient"`
	CanEdit                bool                  `json:"canEdit"`
	ReadOnlyReason         *string               `json:"readOnlyReason,omitempty"`
	ConcurrencyToken       string                `json:"concurrencyToken"`
	IdentifierUris         []URIValue            `json:"identifierUris"`
	Web                    WebConfiguration      `json:"web"`
	Spa                    RedirectConfiguration `json:"spa"`
	PublicClient           RedirectConfiguration `json:"publicClient"`
	AppRoles               []any                 `json:"appRoles"`
	API                    APIConfiguration      `json:"api"`
}

type URIValue struct {
	Value string `json:"value"`
}
type RedirectURI struct {
	Value string `json:"value"`
}
type RedirectConfiguration struct {
	RedirectUris []RedirectURI `json:"redirectUris"`
}
type WebConfiguration struct {
	HomePageURL               *string       `json:"homePageUrl,omitempty"`
	LogoutURL                 *string       `json:"logoutUrl,omitempty"`
	EnableAccessTokenIssuance bool          `json:"enableAccessTokenIssuance"`
	EnableIDTokenIssuance     bool          `json:"enableIdTokenIssuance"`
	RedirectUris              []RedirectURI `json:"redirectUris"`
}
type APIConfiguration struct {
	RequestedAccessTokenVersion *int64 `json:"requestedAccessTokenVersion,omitempty"`
	Scopes                      []any  `json:"scopes"`
	PreAuthorizedApplications   []any  `json:"preAuthorizedApplications"`
}
