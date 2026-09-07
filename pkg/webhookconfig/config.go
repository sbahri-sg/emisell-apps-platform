package webhookconfig

import (
	"emisell.app/platform/pkg/accessscope"
	"errors"
	"slices"
)

// APIVersion versions payload contracts independently of the app's semver.
// This proposed contract is not yet enabled for production delivery.
const APIVersion = "2026-09"

type Subscription struct {
	Topics []string `json:"topics"`
	URI    string   `json:"uri"`
}
type Config struct {
	APIVersion    string         `json:"apiVersion"`
	Subscriptions []Subscription `json:"subscriptions"`
}

// Validate validates authoring only. Optional declarations do not grant access.
func (c Config) Validate(scopes *accessscope.Declaration) error {
	invalid := errors.New("invalid app webhook configuration")
	if c.APIVersion != APIVersion || c.Subscriptions == nil || len(c.Subscriptions) > 20 {
		return invalid
	}
	if len(c.Subscriptions) > 0 && (scopes == nil || scopes.Validate() != nil) {
		return invalid
	}
	seen := map[string]bool{}
	total := 0
	for _, sub := range c.Subscriptions {
		if !ValidEndpoint(sub.URI) || len(sub.Topics) == 0 {
			return invalid
		}
		for _, topic := range sub.Topics {
			total++
			scope, err := RequiredScope(topic)
			if err != nil || total > 50 || seen[topic+"\x00"+sub.URI] {
				return invalid
			}
			if !slices.Contains(scopes.Required, scope) && !slices.Contains(scopes.Optional, scope) {
				return invalid
			}
			seen[topic+"\x00"+sub.URI] = true
		}
	}
	return nil
}
