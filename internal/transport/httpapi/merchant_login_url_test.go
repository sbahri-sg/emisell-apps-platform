package httpapi

import (
	"net/url"
	"strings"
	"testing"
)

func TestMerchantLoginSingleEntry(t *testing.T) {
	request := strings.Repeat("A", 52)
	target, err := url.Parse(merchantLoginURL("http://localhost:3000", request))
	if err != nil || target.Path != "/auth/login" || target.Host != "localhost:3000" {
		t.Fatal("wrong merchant entry", target, err)
	}
	callback, err := url.Parse(target.Query().Get("returnTo"))
	if err != nil || callback.IsAbs() || callback.Host != "" || callback.Path != "/api/app-platform/sso" || callback.Query().Get("request") != request {
		t.Fatal("wrong callback", callback, err)
	}
}
