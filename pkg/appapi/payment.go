package appapi

import (
	"crypto/hmac"
	"strconv"
	"time"
)

// PaymentUpdate is an app-to-platform snapshot, not a platform event envelope.
// Tenant/installation/delivery are authenticated in the signature headers.
type PaymentUpdate struct {
	Type       string    `json:"type"`
	Resource   Resource  `json:"resource"`
	OccurredAt time.Time `json:"occurredAt"`
}

const PaymentUpdateType = "payment.status.v1"

// Direction separation prevents reflecting an outbound platform webhook back
// into the callback endpoint, even though the installation key is shared.
func SignCallback(secret, tenant, installation, delivery, timestamp string, body []byte) string {
	return SignWebhook(secret, tenant, installation, delivery, "app-to-platform/payment.status.v1\n"+timestamp, body)
}
func VerifyCallback(secret, tenant, installation, delivery, timestamp, signature string, body []byte, now time.Time) bool {
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || unix < now.Add(-5*time.Minute).Unix() || unix > now.Add(30*time.Second).Unix() {
		return false
	}
	return hmac.Equal([]byte(signature), []byte(SignCallback(secret, tenant, installation, delivery, timestamp, body)))
}
