package appapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"
)

func SignWebhook(secret, tenant, installation, delivery, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("v1\n" + timestamp + "\n" + tenant + "\n" + installation + "\n" + delivery + "\n"))
	mac.Write(body)
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}
func VerifyWebhook(secret, tenant, installation, delivery, timestamp, signature string, body []byte, now time.Time) bool {
	unix, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || unix < now.Add(-5*time.Minute).Unix() || unix > now.Add(30*time.Second).Unix() {
		return false
	}
	return hmac.Equal([]byte(signature), []byte(SignWebhook(secret, tenant, installation, delivery, timestamp, body)))
}
