package appapi

import (
	"strconv"
	"testing"
	"time"
)

func TestCallbackDirectionBindingAndExpiry(t *testing.T) {
	now := time.Now()
	stamp := strconv.FormatInt(now.Unix(), 10)
	raw := []byte(`{"status":"captured"}`)
	sig := SignCallback("installation-secret", "tenant", "ins", "delivery", stamp, raw)
	if !VerifyCallback("installation-secret", "tenant", "ins", "delivery", stamp, sig, raw, now) {
		t.Fatal("valid signature rejected")
	}
	for _, v := range []struct {
		tenant, ins, delivery, stamp, signature string
		body                                    []byte
	}{
		{"foreign", "ins", "delivery", stamp, sig, raw},
		{"tenant", "foreign", "delivery", stamp, sig, raw},
		{"tenant", "ins", "other", stamp, sig, raw},
		{"tenant", "ins", "delivery", stamp, sig, []byte(`{}`)},
		{"tenant", "ins", "delivery", stamp, SignWebhook("installation-secret", "tenant", "ins", "delivery", stamp, raw), raw},
		{"tenant", "ins", "delivery", "invalid", sig, raw},
	} {
		if VerifyCallback("installation-secret", v.tenant, v.ins, v.delivery, v.stamp, v.signature, v.body, now) {
			t.Fatal("forgery accepted")
		}
	}
	if VerifyCallback("installation-secret", "tenant", "ins", "delivery", stamp, sig, raw, now.Add(6*time.Minute)) || VerifyCallback("installation-secret", "tenant", "ins", "delivery", stamp, sig, raw, now.Add(-time.Minute)) {
		t.Fatal("timestamp accepted outside tolerance")
	}
}
