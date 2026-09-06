package config

import "testing"

func TestLocalOnlyBoundary(t *testing.T) {
	t.Setenv("EMISELL_DATABASE_URL", LocalDatabase)
	t.Setenv("EMISELL_ADDRESS", "127.0.0.1:8087")
	t.Setenv("EMISELL_ORIGIN", "http://localhost:4317")
	if _, err := Read(); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{"0.0.0.0:8087", "192.168.1.2:8087", "localhost:8087"} {
		t.Setenv("EMISELL_ADDRESS", address)
		if _, err := Read(); err == nil {
			t.Fatalf("unsafe binding: %s", address)
		}
	}
	t.Setenv("EMISELL_ADDRESS", "127.0.0.1:8087")
	t.Setenv("EMISELL_ORIGIN", "https://evil.example")
	if _, err := Read(); err == nil {
		t.Fatal("external origin accepted")
	}
	t.Setenv("EMISELL_ORIGIN", "http://localhost:4317")
	t.Setenv("EMISELL_DATABASE_URL", "postgres://localhost/existing_business_db")
	if _, err := Read(); err == nil {
		t.Fatal("unrelated database accepted")
	}
}
