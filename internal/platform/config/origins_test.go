package config

import "testing"

func TestPublicOrigins(t *testing.T) {
	for _, key := range []string{"EMISELL_ADMIN_ORIGIN", "EMISELL_DEVELOPER_ORIGIN", "EMISELL_STORE_ORIGIN", "EMISELL_ENV"} {
		t.Setenv(key, "")
	}
	local, err := ReadPublicOrigins()
	if err != nil || local.Secure || local.Admin != "http://localhost:4317" {
		t.Fatal("local defaults changed")
	}
	t.Setenv("EMISELL_ENV", "production")
	if _, err := ReadPublicOrigins(); err == nil {
		t.Fatal("production accepted missing origins")
	}
	t.Setenv("EMISELL_ADMIN_ORIGIN", "https://admin.example.com")
	t.Setenv("EMISELL_DEVELOPER_ORIGIN", "https://developer.example.com")
	t.Setenv("EMISELL_STORE_ORIGIN", "https://apps.example.com")
	valid, err := ReadPublicOrigins()
	if err != nil || !valid.Secure || !valid.AllowsHost("admin.example.com") || valid.AllowsHost("evil.example.com") {
		t.Fatalf("invalid allowlist: %v", err)
	}
	for _, bad := range []string{"", "http://admin.example.com", "https://127.0.0.1", "https://admin.localhost", "https://admin.example.com/", "https://admin.example.com?", "https://admin.example.com?x=1", "https://user@admin.example.com", "https://admin.example.com:443", "https://developer.example.com", "https://*.example.com", "https://ADMIN.example.com"} {
		t.Setenv("EMISELL_ADMIN_ORIGIN", bad)
		if _, err := ReadPublicOrigins(); err == nil {
			t.Fatalf("accepted unsafe origin %q", bad)
		}
	}
}
