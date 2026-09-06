package appapi

import (
	"strconv"
	"testing"
	"time"
)

func TestWebhookSignatureBindsContextAndRejectsReplayWindow(t *testing.T) {
	now := time.Unix(1800000000, 0)
	timestamp := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"id":"event_1"}`)
	secret := "fixture-secret-at-least-32-characters"
	signature := SignWebhook(secret, "tenant_a", "ins_a", "delivery_a", timestamp, body)
	if !VerifyWebhook(secret, "tenant_a", "ins_a", "delivery_a", timestamp, signature, body, now) {
		t.Fatal("valid signature rejected")
	}
	for _, args := range [][4]string{{"other", "ins_a", "delivery_a", timestamp}, {"tenant_a", "other", "delivery_a", timestamp}, {"tenant_a", "ins_a", "other", timestamp}, {"tenant_a", "ins_a", "delivery_a", "1800000001"}} {
		if VerifyWebhook(secret, args[0], args[1], args[2], args[3], signature, body, now) {
			t.Fatal("context substitution accepted")
		}
	}
	if VerifyWebhook(secret, "tenant_a", "ins_a", "delivery_a", timestamp, signature, append(body, ' '), now) {
		t.Fatal("body tampering accepted")
	}
	if VerifyWebhook(secret, "tenant_a", "ins_a", "delivery_a", timestamp, signature, body, now.Add(301*time.Second)) {
		t.Fatal("stale replay accepted")
	}
	if VerifyWebhook(secret, "tenant_a", "ins_a", "delivery_a", timestamp, signature, body, now.Add(-31*time.Second)) {
		t.Fatal("future timestamp accepted")
	}
}
