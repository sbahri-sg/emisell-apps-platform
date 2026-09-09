package contact

import "testing"

func TestEmail(t *testing.T) {
	for _, email := range []string{"api@example.com", "support+apps@example.co.id"} {
		if !ValidEmail(email) {
			t.Errorf("valid email rejected: %s", email)
		}
	}
	for _, email := range []string{"", "Name <api@example.com>", "api@example.com\r\nBcc:someone@example.com", " a@example.com", "a@example.com,b@example.com", "invalid"} {
		if ValidEmail(email) {
			t.Error("invalid email accepted")
		}
	}
}
