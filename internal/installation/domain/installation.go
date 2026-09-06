package domain

import (
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/pkg/appmanifest"
	"slices"
	"time"
)

type Installation struct {
	IntentID         string    `json:"intentId,omitempty"`
	ID               string    `json:"id"`
	AppID            string    `json:"appId"`
	Version          string    `json:"version"`
	Status           string    `json:"status"`
	Scopes           []string  `json:"scopes"`
	Capabilities     []string  `json:"capabilities"`
	InstalledAt      time.Time `json:"installedAt"`
	ExecutionProfile string    `json:"executionProfile,omitempty"`
	ConnectionStatus string    `json:"connectionStatus,omitempty"`
}
type Action struct {
	Type    string   `json:"type"`
	AppID   string   `json:"appId"`
	Version string   `json:"version,omitempty"`
	Grants  []string `json:"grants,omitempty"`
}

// Transition is pure business logic; storage locks, SQL and HTTP stay outside.
func Transition(current *Installation, app appmanifest.Manifest, action Action) (*Installation, bool, error) {
	switch action.Type {
	case "install":
		grants := slices.Clone(action.Grants)
		slices.Sort(grants)
		grants = slices.Compact(grants)
		required := slices.Clone(app.Scopes)
		slices.Sort(required)
		if action.Version != app.Version || !slices.Equal(grants, required) {
			return nil, false, fault.Invalid
		}
		if current != nil && current.Status != "uninstalled" {
			return current, false, nil
		}
		return &Installation{ID: ids.New("ins"), AppID: app.ID, Version: app.Version, Status: "pending", Scopes: slices.Clone(app.Scopes), Capabilities: slices.Clone(app.Capabilities), InstalledAt: time.Now().UTC()}, true, nil
	case "activate":
		if current == nil || current.Status == "uninstalled" || current.Status == "disabling" {
			return nil, false, fault.NotFound
		}
		if current.Version != app.Version || !slices.Equal(current.Scopes, app.Scopes) {
			return nil, false, fault.Forbidden
		}
		if current.Status == "active" {
			return current, false, nil
		}
		next := *current
		next.Status = "active"
		return &next, true, nil
	case "uninstall":
		if current == nil {
			return &Installation{AppID: app.ID, Version: app.Version, Status: "uninstalled", Scopes: []string{}, Capabilities: []string{}}, false, nil
		}
		if current.Status == "uninstalled" || current.Status == "disabling" {
			return current, false, nil
		}
		next := *current
		next.Status = "uninstalled"
		if app.ExecutionProfile == "local-remote" {
			next.Status = "disabling"
		}
		next.Scopes = []string{}
		next.Capabilities = []string{}
		return &next, true, nil
	case "complete_uninstall":
		if current == nil || current.Status != "disabling" {
			return nil, false, fault.Conflict
		}
		next := *current
		next.Status = "uninstalled"
		return &next, true, nil
	default:
		return nil, false, fault.Invalid
	}
}
