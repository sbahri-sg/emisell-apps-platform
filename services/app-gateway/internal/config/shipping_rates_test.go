package config

import "testing"

func TestShippingRatesRemainOffWithoutExplicitEnablement(t *testing.T) {
	t.Setenv("API_KURIR_RATES_ENABLED", "false")
	t.Setenv("API_KURIR_BASE_URL", "http://unsafe.example.test")
	t.Setenv("API_KURIR_SERVICE_KEY", "short")
	client, err := LoadShippingRates("development")
	if err != nil || client != nil {
		t.Fatalf("disabled bridge initialized: client=%v err=%v", client, err)
	}
}

func TestShippingRatesRequireCompleteSafeServerConfiguration(t *testing.T) {
	const key = "api-kurir-sandbox-service-key-000001"
	for _, test := range []struct {
		name        string
		enabled     string
		environment string
		origin      string
		key         string
		timeout     string
		wantError   bool
	}{
		{name: "development loopback HTTP", enabled: "true", environment: "development", origin: "http://127.0.0.1:13000", key: key, timeout: "5s"},
		{name: "production HTTPS", enabled: "true", environment: "production", origin: "https://api-kurir.example.test", key: key, timeout: "1s"},
		{name: "production rejects HTTP", enabled: "true", environment: "production", origin: "http://api-kurir.example.test", key: key, timeout: "5s", wantError: true},
		{name: "missing origin", enabled: "true", environment: "development", key: key, timeout: "5s", wantError: true},
		{name: "missing key", enabled: "true", environment: "development", origin: "http://127.0.0.1:13000", timeout: "5s", wantError: true},
		{name: "invalid flag", enabled: "sometimes", environment: "development", wantError: true},
		{name: "invalid timeout", enabled: "true", environment: "development", origin: "http://127.0.0.1:13000", key: key, timeout: "forever", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("API_KURIR_RATES_ENABLED", test.enabled)
			t.Setenv("API_KURIR_BASE_URL", test.origin)
			t.Setenv("API_KURIR_SERVICE_KEY", test.key)
			t.Setenv("API_KURIR_RATES_TIMEOUT", test.timeout)
			client, err := LoadShippingRates(test.environment)
			if test.wantError && err == nil {
				t.Fatalf("unsafe configuration accepted: %#v", test)
			}
			if !test.wantError && (err != nil || client == nil) {
				t.Fatalf("safe configuration rejected: client=%v err=%v", client, err)
			}
		})
	}
}
