// Package integrationmanifest attests immutable remote integration configuration,
// not remotely hosted application code and never an installation grant.
package integrationmanifest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"

	"emisell.app/platform/pkg/accessscope"
	"emisell.app/platform/pkg/catalogmanifest"
)

const Schema = "emisell.integration-release/v1"
const Policy = "integration-configuration/v1"
const Protocol = "emisell.capability-http/v1"

type Config struct {
	Protocol    string `json:"protocol"`
	Endpoint    string `json:"endpoint"`
	CallbackURL string `json:"callbackUrl"`
	HealthURL   string `json:"healthUrl"`
}
type Manifest struct {
	Schema       string                   `json:"schema"`
	Policy       string                   `json:"policy"`
	SubmissionID string                   `json:"submissionId"`
	Metadata     catalogmanifest.Manifest `json:"metadata"`
	Config       Config                   `json:"config"`
}
type Package struct {
	Manifest  Manifest `json:"manifest"`
	SHA256    string   `json:"sha256"`
	KeyID     string   `json:"keyId"`
	Signature []byte   `json:"signature"`
}
type Check struct {
	Code    string `json:"code"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}
type Report struct {
	Valid       bool     `json:"valid"`
	Installable bool     `json:"installable"`
	Checks      []Check  `json:"checks"`
	Blockers    []string `json:"blockers"`
}

var hostPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)

// Static allow-list only. No DNS lookup, health probe or outbound request is
// performed here. A future runtime MUST enforce resolved-IP/redirect/egress policy.
func endpoint(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(raw, "#") || u.Opaque != "" || (u.Port() != "" && u.Port() != "443") {
		return nil, false
	}
	h := u.Hostname()
	if h != strings.ToLower(h) || len(h) > 253 || net.ParseIP(h) != nil || !strings.Contains(h, ".") {
		return nil, false
	}
	for _, label := range strings.Split(h, ".") {
		if !hostPattern.MatchString(label) {
			return nil, false
		}
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".test", ".invalid", ".onion", ".home", ".lan"} {
		if strings.HasSuffix(h, suffix) {
			return nil, false
		}
	}
	if strings.Trim(h, "0123456789.") == "" {
		return nil, false
	}
	return u, true
}

func Inspect(m Manifest) Report {
	r := Report{Valid: true, Checks: []Check{}, Blockers: []string{
		"Runtime Aplikasi Integrasi umum belum tersedia; release ini tidak masuk registry executable.",
		"Status app-client dan bukti kendali endpoint diperiksa terpisah. OAuth token exchange dan uji kontrak runtime end-to-end belum tersedia.",
		"Kode di server developer tidak dipindai. DNS, TLS, health, redirect, dan egress belum diverifikasi secara langsung.",
	}}
	check := func(code string, ok bool, message string) {
		r.Checks = append(r.Checks, Check{code, ok, message})
		r.Valid = r.Valid && ok
	}
	check("manifest", m.Schema == Schema && m.Policy == Policy && idPattern.MatchString(m.SubmissionID) && m.Metadata.Validate() == nil, "Manifest, versi, capability dan snapshot metadata harus sesuai kontrak.")
	check("protocol", m.Config.Protocol == Protocol, "Profil HTTP capability v1; belum merupakan bukti kompatibilitas server developer.")
	base, ok := endpoint(m.Config.Endpoint)
	callback, callbackOK := endpoint(m.Config.CallbackURL)
	health, healthOK := endpoint(m.Config.HealthURL)
	check("https_endpoints", ok && callbackOK && healthOK, "URL wajib HTTPS, hostname DNS, tanpa credentials/query/fragment dan hanya port 443.")
	same := ok && callbackOK && healthOK && base.Hostname() == callback.Hostname() && base.Hostname() == health.Hostname()
	check("endpoint_origin", same, "Callback dan health harus satu origin dengan endpoint integrasi pada policy v1.")
	grantable := map[string]bool{}
	for _, s := range accessscope.Reference().Scopes {
		grantable[s.Handle] = s.Grantable
	}
	ready := true
	if d := m.Metadata.AccessScopes; d != nil {
		for _, handle := range d.Required {
			ready = ready && grantable[handle]
		}
		for _, handle := range d.Optional {
			if !grantable[handle] {
				r.Blockers = append(r.Blockers, "Scope opsional "+handle+" belum grantable; tidak diberikan akses.")
			}
		}
	}
	check("required_scopes", ready, "Semua resource scope wajib harus grantable; scope Plan tidak bisa menjadi izin aktif.")
	return r
}
func Canonical(m Manifest) ([]byte, error) {
	if !Inspect(m).Valid {
		return nil, errors.New("invalid integration configuration")
	}
	return json.Marshal(m)
}
func Digest(raw []byte) string              { v := sha256.Sum256(raw); return hex.EncodeToString(v[:]) }
func KeyID(public ed25519.PublicKey) string { return "integration-" + Digest(public) }
func Sign(m Manifest, key ed25519.PrivateKey) (Package, error) {
	raw, err := Canonical(m)
	if err != nil {
		return Package{}, err
	}
	if len(key) != ed25519.PrivateKeySize {
		return Package{}, errors.New("integration signer unavailable")
	}
	return Package{m, Digest(raw), KeyID(key.Public().(ed25519.PublicKey)), ed25519.Sign(key, append([]byte(Policy+"\n"), raw...))}, nil
}
func Verify(p Package, public ed25519.PublicKey) error {
	raw, err := Canonical(p.Manifest)
	if err != nil {
		return err
	}
	if len(public) != ed25519.PublicKeySize || p.KeyID != KeyID(public) || p.SHA256 != Digest(raw) || !ed25519.Verify(public, append([]byte(Policy+"\n"), raw...), p.Signature) {
		return errors.New("integration signature verification failed")
	}
	return nil
}
