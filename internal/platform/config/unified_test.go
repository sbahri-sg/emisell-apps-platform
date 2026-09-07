package config

import "testing"

func TestUnifiedDomainConfiguration(t *testing.T) {
	t.Setenv("EMISELL_ENV", "production")
	t.Setenv("EMISELL_ADMIN_ORIGIN", "")
	t.Setenv("EMISELL_DEVELOPER_ORIGIN", "")
	t.Setenv("EMISELL_DASHBOARD_ORIGIN", "https://dashboard.example.com")
	t.Setenv("EMISELL_STORE_ORIGIN", "https://apps.example.com")
	c, e := ReadPublicOrigins()
	if e != nil || c.Admin != c.Developer || !c.Secure {
		t.Fatal("unified domain failed", e)
	}
	t.Setenv("EMISELL_STORE_ORIGIN", c.Admin)
	if _, e = ReadPublicOrigins(); e == nil {
		t.Fatal("store must remain separate")
	}
}
