package bootstrap_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func merchantRPC(t *testing.T, address, secret, method string, body map[string]any, want int) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(body)
	r, _ := http.NewRequest("POST", address+method, strings.NewReader(string(raw)))
	r.Header.Set("Authorization", "Bearer "+secret)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Connect-Protocol-Version", "1")
	res, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != want {
		t.Fatalf("%s: status %d, want %d", method, res.StatusCode, want)
	}
	var out map[string]any
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestMerchantOnlyJSONLifecycleAndLegacyReplay(t *testing.T) {
	f := setup(t)
	_, _, secret, address := lifecycleClient(t, f)
	const intents = "/emisell.installation.v1.InstallIntentService/"
	const installs = "/emisell.installation.v1.InstallationService/"
	prepareKey := key()
	in := map[string]any{"merchantId": f.tenant, "coreActorId": "staff", "idempotencyKey": prepareKey, "appId": "emisell-pay", "version": "1.0.0"}
	p := merchantRPC(t, address, secret, intents+"Prepare", in, 200)["intent"].(map[string]any)
	if p["merchantId"] != f.tenant {
		t.Fatal("missing canonical response identity")
	}
	delete(in, "merchantId")
	in["tenantId"] = f.tenant
	replay := merchantRPC(t, address, secret, intents+"Prepare", in, 200)["intent"].(map[string]any)
	if replay["id"] != p["id"] || replay["tenantId"] != f.tenant {
		t.Fatal("legacy replay changed identity or receipt")
	}
	in["merchantId"] = f.other
	bad := merchantRPC(t, address, secret, intents+"Prepare", in, 400)
	if bad["code"] != "invalid_argument" {
		t.Fatal("alias conflict not rejected")
	}
	merchantRPC(t, address, secret, intents+"Get", map[string]any{"merchantId": f.other, "coreActorId": "staff", "intentId": p["id"]}, 404)
	merchantRPC(t, address, secret, intents+"Get", map[string]any{"merchantId": f.tenant, "coreActorId": "staff", "intentId": p["id"]}, 200)
	merchantRPC(t, address, secret, intents+"Decide", map[string]any{"merchantId": f.tenant, "coreActorId": "staff", "idempotencyKey": key(), "intentId": p["id"], "consentDigest": p["consentDigest"], "decision": "CONSENT_DECISION_CONSENT"}, 200)
	consumed := merchantRPC(t, address, secret, installs+"Consume", map[string]any{"merchantId": f.tenant, "coreActorId": "staff", "idempotencyKey": key(), "intentId": p["id"], "consentDigest": p["consentDigest"]}, 200)
	i := consumed["result"].(map[string]any)["installation"].(map[string]any)
	if i["merchantId"] != f.tenant {
		t.Fatal("installation identity")
	}
	target := func() map[string]any {
		return map[string]any{"target": map[string]any{"merchantId": f.tenant, "coreActorId": "staff", "installationId": i["installationId"], "idempotencyKey": key()}}
	}
	merchantRPC(t, address, secret, installs+"Activate", target(), 200)
	merchantRPC(t, address, secret, installs+"GetInstallation", target(), 200)
	merchantRPC(t, address, secret, "/emisell.payment.v1.PaymentService/Create", map[string]any{"merchantId": f.tenant, "idempotencyKey": key(), "reference": "merchant-contract", "amountMinor": "1000", "currency": "IDR"}, 200)
	issued := merchantRPC(t, address, secret, installs+"IssueToken", target(), 200)["result"].(map[string]any)
	token := issued["appToken"].(string)
	check := func(merchant, legacy string, want int) {
		t.Helper()
		r, _ := http.NewRequest("GET", f.server.URL+"/api/v1/app/installation-access", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("X-Emisell-Merchant-ID", merchant)
		if legacy != "" {
			r.Header.Set("X-Emisell-Tenant-ID", legacy)
		}
		r.Header.Set("X-Emisell-App-ID", "emisell-pay")
		r.Header.Set("X-Emisell-Installation-ID", i["installationId"].(string))
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != want {
			t.Fatalf("self-check status %d want %d", res.StatusCode, want)
		}
		body, _ := io.ReadAll(res.Body)
		if want == 200 && merchant != "" {
			var v map[string]any
			_ = json.Unmarshal(body, &v)
			if v["merchantId"] != f.tenant || v["tenantId"] != nil {
				t.Fatal("canonical self-check exposes second ID")
			}
		}
	}
	check(f.tenant, "", 200)
	check("", f.tenant, 200)
	check(f.tenant, f.other, 400)
	check(f.other, "", 401)
	merchantRPC(t, address, secret, installs+"Uninstall", target(), 200)
	check(f.tenant, "", 401)
}
