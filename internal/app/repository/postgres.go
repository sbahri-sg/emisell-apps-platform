package repository

import (
	"context"
	"crypto/ed25519"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/pkg/appmanifest"
	"encoding/base64"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct{ Pool *pgxpool.Pool }

// SeedRemote is opt-in and does not rewrite existing signed releases.
func (p Postgres) SeedRemote(ctx context.Context) error {
	m := appmanifest.Manifest{Schema: "emisell.app/v1", ID: "remote-pay", Name: "Remote Pay", Version: "1.0.0", DeveloperID: "emisell-local", Runtime: "Remote app", ExecutionProfile: "local-remote", Compatibility: "platform/v1", ArtifactSHA256: appmanifest.RemoteArtifactDigest(), Category: "Pembayaran", Summary: "Reference app terpisah dengan OAuth dan webhook.", Description: "Aplikasi contoh di localhost. Hubungkan melalui OAuth sebelum aktivasi. Semua pembayaran simulasi; tidak memindahkan uang.", Icon: "payment", Color: "blue", Scopes: []string{"orders.read", "payments.read", "payments.write"}, Capabilities: []string{"payment/v1"}, Subscriptions: []string{"emisell.capability.invoked.v1"}}
	raw, err := json.Marshal(m)
	if err != nil {
		return err
	}
	key := appmanifest.LocalFixtureKey()
	_, err = p.Pool.Exec(ctx, "INSERT INTO platform_app.releases(app_id,version,manifest,signature,public_key) VALUES($1,$2,$3::json,$4,$5) ON CONFLICT DO NOTHING", m.ID, m.Version, string(raw), base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw)), base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)))
	return err
}

func (p Postgres) List(ctx context.Context) ([]service.Release, error) {
	rows, err := p.Pool.Query(ctx, "SELECT manifest::text, signature FROM platform_app.releases ORDER BY app_id, version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []service.Release{}
	for rows.Next() {
		var r service.Release
		if err = rows.Scan(&r.Raw, &r.Signature); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// Explicit CLI-only bootstrap; release rows remain immutable thereafter.
func (p Postgres) SeedLocal(ctx context.Context) error {
	key := appmanifest.LocalFixtureKey()
	for _, spec := range []struct{ id, name, cap, category, icon, color string }{
		{"emisell-pay", "Emisell Pay", "payment/v1", "Pembayaran", "payment", "violet"},
		{"emisell-pay-alt", "Emisell Pay Alternate", "payment/v1", "Pembayaran", "payment", "green"},
		{"parcel", "Parcel", "shipping/v1", "Pengiriman", "shipping", "blue"},
		{"parcel-alt", "Parcel Alternate", "shipping/v1", "Pengiriman", "shipping", "orange"},
	} {
		scopes := []string{"orders.read", "payments.read", "payments.write"}
		if spec.cap == "shipping/v1" {
			scopes = []string{"orders.read", "shipping.read", "shipping.write"}
		}
		m := appmanifest.Manifest{Schema: "emisell.app/v1", ID: spec.id, Name: spec.name, Version: "1.0.0", DeveloperID: "emisell-local", Runtime: "Remote app", ExecutionProfile: "local-simulator", Compatibility: "platform/v1", ArtifactSHA256: appmanifest.ArtifactDigest(), Category: spec.category, Summary: "Reference app untuk " + spec.cap + ".", Description: "Installation dan izin disimpan di PostgreSQL. Eksekusi capability menggunakan simulator lokal, tanpa transaksi atau pengiriman asli.", Icon: spec.icon, Color: spec.color, Scopes: scopes, Capabilities: []string{spec.cap}}
		raw, err := json.Marshal(m)
		if err != nil {
			return err
		}
		_, err = p.Pool.Exec(ctx, "INSERT INTO platform_app.releases(app_id,version,manifest,signature,public_key) VALUES($1,$2,$3::json,$4,$5) ON CONFLICT DO NOTHING", m.ID, m.Version, string(raw), base64.StdEncoding.EncodeToString(ed25519.Sign(key, raw)), base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey)))
		if err != nil {
			return err
		}
	}
	return nil
}
