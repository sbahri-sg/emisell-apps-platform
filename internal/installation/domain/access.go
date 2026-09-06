package domain

import "time"

const EmbeddedPilotApp = "embedded-local-demo"
const EmbeddedPilotPolicy = "embedded-local-demo/v1"

// Access is the installation aggregate's consent provenance and native grant.
// Resource gateway grants are deliberately NOT represented by fixture scopes.
type Access struct {
	ReviewedUILaunch    *UIBinding // ephemeral decision; never persisted
	LocalEmbeddedAccess bool       // ephemeral decision; never persisted
	Owner               IntentOwner
	IntentID            string
	Release             IntentRelease
	ConsentDigest       string
	Installation        Installation
	GrantState          string
	GrantedScopes       []string
	ConsumedAt          time.Time
}

type AppToken struct {
	ID        string
	Secret    string // ephemeral; never persisted or included in an audit/event
	ExpiresAt time.Time
	Revoked   bool
}

type AccessResult struct {
	Access   Access
	Token    AppToken
	Replayed bool
}

// InstalledApp is a read-only merchant summary, not lifecycle action authority.
type InstalledApp struct {
	ID, AppID, Name, DeveloperID, Version, Status, GrantState, ExecutionProfile string
	InstalledAt                                                                 time.Time
}
