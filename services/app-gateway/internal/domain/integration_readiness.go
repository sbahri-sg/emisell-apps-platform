package domain

import "time"

// IntegrationReadiness deliberately contains no URLs, credentials, merchant
// identities, extension configuration, webhook payloads or signing material.
type IntegrationReadiness struct {
	AppID            string                   `json:"appId"`
	OrganizationID   string                   `json:"organizationId"`
	AppName          string                   `json:"appName"`
	AppRevision      int64                    `json:"appRevision"`
	ActiveVersionID  *string                  `json:"activeVersionId"`
	ActiveVersion    *string                  `json:"activeVersion"`
	CheckedAt        time.Time                `json:"checkedAt"`
	Checks           []IntegrationCheck       `json:"checks"`
	Scopes           []IntegrationScope       `json:"scopes"`
	Installations    IntegrationInstallations `json:"installations"`
	ListingStatus    CatalogListingStatus     `json:"listingStatus"`
	ListingRevision  int64                    `json:"listingRevision"`
	EndToEndVerified bool                     `json:"endToEndVerified"`
}

type IntegrationCheck struct {
	Code    string `json:"code"`
	Title   string `json:"title"`
	Status  string `json:"status"` // pass, attention, blocked, or info
	Detail  string `json:"detail"`
	Section string `json:"section"` // Existing Developer Console destination.
}

type IntegrationScope struct {
	Scope        string            `json:"scope"`
	Access       string            `json:"access"`
	Availability ScopeAvailability `json:"availability"`
	Endpoints    []ScopeEndpoint   `json:"endpoints"`
}

type IntegrationInstallations struct {
	Sampled              int  `json:"sampled"`
	HasMore              bool `json:"hasMore"`
	ActiveCurrentVersion int  `json:"activeCurrentVersion"`
	ActiveOtherVersion   int  `json:"activeOtherVersion"`
	Inactive             int  `json:"inactive"`
}
