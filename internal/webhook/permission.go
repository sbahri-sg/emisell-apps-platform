package webhook

import (
	"emisell.app/platform/internal/apppermission"
	"emisell.app/platform/internal/installation/domain"
	"emisell.app/platform/pkg/appmanifest"
	"slices"
)

// Subscription is a trusted server-side record, never browser input. Approved
// denotes authorization of its source release/configuration, not a separate
// administrator approval of each merchant's subscription.
// Topic policies must come from the event producer's validated API contract.
// This evaluator does not register topics, enable scopes, or authorize publishers.
type Subscription struct {
	apppermission.Identity
	Topic, ReleaseVersion string
	Active, Approved      bool
}

// AllowsSubscription is provider-independent. The caller must load a current
// grant under the lifecycle gate both before enqueue and before each delivery.
func AllowsSubscription(expected apppermission.Identity, grant apppermission.Grant, sub Subscription, topic, version string, required []string) bool {
	return topic != "" && topic != "*" && version != "" && sub.Identity == expected &&
		sub.Active && sub.Approved && sub.Topic == topic && sub.ReleaseVersion == version &&
		apppermission.Allows(expected, grant, required)
}

// The historical local fixture has no resource-scope contract. Its signed
// manifest's exact legacy scopes are required; never map them to read_* grants.
func allowsLocalWebhook(tenant, installation, topic string, ins domain.Installation, app appmanifest.Manifest) bool {
	if app.ExecutionProfile != "local-remote" || ins.ID != installation || ins.AppID != app.ID || ins.Version != app.Version || !slices.Contains(app.Subscriptions, topic) {
		return false
	}
	identity := apppermission.Identity{MerchantID: tenant, AppID: app.ID, InstallationID: installation}
	return AllowsSubscription(identity, apppermission.Grant{Identity: identity, Active: ins.Status == "active", GrantedScopes: ins.Scopes, ConsentedScopes: ins.Scopes},
		Subscription{Identity: identity, Topic: topic, ReleaseVersion: app.Version, Active: true, Approved: true}, topic, ins.Version, app.Scopes)
}
