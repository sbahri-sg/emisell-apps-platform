package kurir

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/pkg/appmanifest"
)

func TestProviderReadinessNeverUsesAnotherProviderOrChangesCheckout(t *testing.T) {
	const ready = `{"data":{"code":"rajaongkir","built_in":false,"installed":true,"available":true,"active":false}}`
	for _, tc := range []struct {
		name, body string
		want       error
	}{
		{"installed but not selected", ready, nil},
		{"selected", strings.Replace(ready, `"active":false`, `"active":true`, 1), nil},
		{"another provider", strings.Replace(ready, `rajaongkir`, `kiriminaja`, 1), fault.Unavailable},
		{"missing credential", strings.Replace(ready, `"installed":true`, `"installed":false`, 1), fault.Conflict},
		{"unavailable", strings.Replace(ready, `"available":true`, `"available":false`, 1), fault.Conflict},
		{"missing state", `{"data":{"code":"rajaongkir","installed":true}}`, fault.Unavailable},
		{"builtin", strings.Replace(ready, `"built_in":false`, `"built_in":true`, 1), fault.Unavailable},
		{"no data", `{}`, fault.Unavailable},
		{"extra JSON", ready + ready, fault.Unavailable},
		{"oversized", strings.Repeat(" ", 33<<10) + ready, fault.Unavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int64
			c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.Method != "GET" || r.URL.Path != "/api/v1/integrations/providers/rajaongkir" || r.URL.RawQuery != "" || r.Header.Get("X-Emisell-Merchant-ID") != "merchant_1" || r.Header.Get("key") != FixtureKey || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
					t.Error("provider readiness crossed its boundary")
				}
				w.Header().Set(FixtureHeader, appmanifest.KurirFixtureProfile)
				_, _ = w.Write([]byte(tc.body))
			})
			err := c.ReadyProvider(context.Background(), "merchant_1", "ins_1", appmanifest.ShippingProviderBinding{Engine: "api-kurir", ProviderCode: "rajaongkir"})
			if !errors.Is(err, tc.want) || hits.Load() != 1 {
				t.Fatal("wrong readiness or hidden retry", err, hits.Load())
			}
		})
	}
}

func TestInvalidProviderBindingNeverReachesEngine(t *testing.T) {
	var hits atomic.Int64
	c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) { hits.Add(1) })
	for _, binding := range []appmanifest.ShippingProviderBinding{{}, {Engine: "other", ProviderCode: "rajaongkir"}, {Engine: "api-kurir", ProviderCode: "../rajaongkir"}, {Engine: "api-kurir", ProviderCode: "biteship"}, {Engine: "api-kurir", ProviderCode: "emisell-kurir"}} {
		if c.ReadyProvider(context.Background(), "merchant", "ins", binding) == nil {
			t.Fatal("accepted invalid binding")
		}
	}
	m, _ := appmanifest.ShippingProviderFixture("rajaongkir")
	for _, ids := range [][2]string{{"bad\r\nmerchant", "ins"}, {"merchant", ""}} {
		if c.ReadyProvider(context.Background(), ids[0], ids[1], *m.ShippingProvider) == nil {
			t.Fatal("accepted invalid identity")
		}
	}
	if hits.Load() != 0 {
		t.Fatal("invalid request reached engine")
	}
}

func TestEmisellBuiltInReadinessIsExactAndDoesNotProvisionCredentials(t *testing.T) {
	const ready = `{"data":{"code":"emisell","built_in":true,"installed":true,"available":true,"active":false}}`
	for _, tc := range []struct {
		name, body string
		want       error
	}{
		{"built-in not selected", ready, nil},
		{"built-in selected", strings.Replace(ready, `"active":false`, `"active":true`, 1), nil},
		{"not built-in", strings.Replace(ready, `"built_in":true`, `"built_in":false`, 1), fault.Unavailable},
		{"missing built-in flag", `{"data":{"code":"emisell","installed":true,"available":true}}`, fault.Unavailable},
		{"other provider", strings.Replace(ready, `"emisell"`, `"rajaongkir"`, 1), fault.Unavailable},
		{"unavailable", strings.Replace(ready, `"available":true`, `"available":false`, 1), fault.Conflict},
		{"not installed", strings.Replace(ready, `"installed":true`, `"installed":false`, 1), fault.Conflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var hits atomic.Int64
			c := fixtureClient(t, func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				if r.Method != "GET" || r.URL.Path != "/api/v1/integrations/providers/emisell" || r.URL.RawQuery != "" || r.ContentLength > 0 || r.Header.Get("X-Emisell-Merchant-ID") != "merchant_1" || r.Header.Get("key") != FixtureKey {
					t.Error("built-in readiness changed the upstream contract")
				}
				w.Header().Set(FixtureHeader, appmanifest.KurirFixtureProfile)
				_, _ = w.Write([]byte(tc.body))
			})
			err := c.ReadyProvider(context.Background(), "merchant_1", "ins_1", appmanifest.ShippingProviderBinding{Engine: "api-kurir", ProviderCode: "emisell"})
			if !errors.Is(err, tc.want) || hits.Load() != 1 {
				t.Fatal("wrong built-in readiness or hidden retry", err, hits.Load())
			}
		})
	}
}
