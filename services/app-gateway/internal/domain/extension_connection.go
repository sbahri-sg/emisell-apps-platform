package domain

import "time"

// Separate from AppCredential (OAuth) and from public version configuration.
// One machine identity and one encrypted provider credential per installation/extension.
type ExtensionConnection struct {
	ID               string    `json:"id"`
	AppID            string    `json:"appId"`
	InstallationID   string    `json:"installationId"`
	ExtensionID      string    `json:"extensionId"`
	RuntimeName      string    `json:"runtimeName"`
	Scopes           []string  `json:"scopes"`
	Status           string    `json:"status"`
	Revision         int64     `json:"revision"`
	RuntimeExpiresAt time.Time `json:"runtimeExpiresAt"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
	TokenHash        string    `json:"-"`
	Ciphertext       []byte    `json:"-"`
	KeyVersion       int       `json:"-"`
}
