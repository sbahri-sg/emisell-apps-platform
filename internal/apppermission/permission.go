// Package apppermission evaluates trusted installation grants independently of
// an application's provider or business domain. It does not authenticate callers.
package apppermission

import "slices"

type Identity struct {
	MerchantID, AppID, InstallationID string
}

type Grant struct {
	Identity
	Active, Revoked                bool
	ConsentedScopes, GrantedScopes []string
}

// Allows requires every server-defined scope in both consent and current grant.
// Callers must load a fresh grant from a trusted source, authenticate the caller,
// and resolve required scopes from server policy, never browser input. Empty
// requirements are rejected: metadata/reconciliation is a separate operation.
func Allows(expected Identity, grant Grant, required []string) bool {
	if expected.MerchantID == "" || expected.AppID == "" || expected.InstallationID == "" || expected != grant.Identity || !grant.Active || grant.Revoked || len(required) == 0 {
		return false
	}
	for _, scope := range required {
		if scope == "" || scope == "*" || !slices.Contains(grant.ConsentedScopes, scope) || !slices.Contains(grant.GrantedScopes, scope) {
			return false
		}
	}
	return true
}
