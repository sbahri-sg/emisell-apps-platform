package domain

import "time"

type AdminAccount struct {
	OrganizationID string
	UserID         string
	Email          string
	DisplayName    string
	PasswordHash   string
	DisabledAt     *time.Time
}
