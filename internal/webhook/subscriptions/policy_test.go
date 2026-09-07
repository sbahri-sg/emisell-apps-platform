package subscriptions

import "testing"

func TestEndpointPolicy(t *testing.T) {
	for _, u := range []string{"https://hooks.emisell.com/events", "https://example.com:443/webhook"} {
		if !ValidEndpoint(u) {
			t.Errorf("rejected %s", u)
		}
	}
	for _, u := range []string{"http://example.com", "https://localhost", "https://127.0.0.1", "https://[::1]/", "https://user:secret@example.com", "https://example.com?token=secret", "https://example.com#x", "https://example.com:8080", "https://a.internal", "https://example.com.", "https://example.com?"} {
		if ValidEndpoint(u) {
			t.Errorf("accepted %s", u)
		}
	}
	for _, topic := range Topics() {
		if topic.DeliveryReady || topic.RequiredScope == "" {
			t.Fatal("authoring topics must not enable delivery")
		}
	}
	if _, err := requiredScope("orders.*"); err == nil {
		t.Fatal("wildcard topic accepted")
	}
}
