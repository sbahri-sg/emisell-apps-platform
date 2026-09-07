// Package webhookconfig defines app-specific webhook configuration, not grants.
package webhookconfig

import (
	"emisell.app/platform/internal/platform/fault"
	"net"
	"net/url"
	"regexp"
	"strings"
)

type Topic struct {
	Name          string `json:"name"`
	RequiredScope string `json:"requiredScope"`
	DeliveryReady bool   `json:"deliveryReady"`
}

// Proposed v1 resource topics. All remain non-deliverable until their Core
// producer, payload contract, reviewed routing and deployment are verified.
func Topics() []Topic {
	return []Topic{
		{Name: "products.created", RequiredScope: "read_products"},
		{Name: "products.updated", RequiredScope: "read_products"},
		{Name: "orders.created", RequiredScope: "read_orders"},
		{Name: "orders.updated", RequiredScope: "read_orders"},
		{Name: "fulfillments.created", RequiredScope: "read_fulfillments"},
		{Name: "fulfillments.updated", RequiredScope: "read_fulfillments"},
	}
}

func RequiredScope(topic string) (string, error) {
	for _, t := range Topics() {
		if t.Name == topic {
			return t.RequiredScope, nil
		}
	}
	return "", fault.Invalid
}

var hostPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

// Syntax validation only: does not contact a developer-supplied URL. A future
// delivery adapter must independently resolve/pin public IPs and block redirects.
func ValidEndpoint(raw string) bool {
	u, e := url.Parse(raw)
	if e != nil || len(raw) > 2048 || u.Scheme != "https" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#") || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	h := u.Hostname()
	if len(h) > 253 || !hostPattern.MatchString(h) || net.ParseIP(h) != nil {
		return false
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".test", ".invalid", ".example", ".onion", ".home", ".lan"} {
		if strings.HasSuffix(h, suffix) {
			return false
		}
	}
	return h != "localhost"
}
