package config

import "testing"

func TestProductionConfigRequiresExplicitDatabase(t *testing.T) {
	t.Setenv("EMISELL_ENV", "production")
	t.Setenv("EMISELL_ADMIN_ORIGIN", "https://admin.example.com")
	t.Setenv("EMISELL_DEVELOPER_ORIGIN", "https://developer.example.com")
	t.Setenv("EMISELL_STORE_ORIGIN", "https://apps.example.com")
	for _, bad := range []string{"", LocalDatabase, "postgres://emisell:short@postgres/emisell"} {
		t.Setenv("EMISELL_DATABASE_URL", bad)
		if _, err := Read(); err == nil {
			t.Fatal("unsafe production database accepted")
		}
	}
	t.Setenv("EMISELL_DATABASE_URL", "postgres://emisell:random-testing-password@postgres/emisell")
	cfg, err := Read()
	if err != nil || cfg.Address != "0.0.0.0:8087" || cfg.RPCAddress != "127.0.0.1:8088" || cfg.Origin != "https://admin.example.com" {
		t.Fatalf("production config failed: %v", err)
	}
}
