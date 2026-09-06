// Package appclient registers confidential app identities and endpoint evidence.
// It is not a token issuer and never grants tenant/resource access.
package appclient

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"

	"emisell.app/platform/internal/developer"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
)

const ProofTTL = 24 * time.Hour
const ChallengeTTL = 10 * time.Minute
const ProofSchema = "emisell.endpoint-proof/v1"

type Binding struct {
	ReleaseID      string `json:"releaseId"`
	OrganizationID string `json:"organizationId"`
	AppID          string `json:"appId"`
	Version        string `json:"version"`
	Name           string `json:"name"`
	Digest         string `json:"digest"`
	Endpoint       string `json:"endpoint"`
	RedirectURI    string `json:"redirectUri"`
}
type Proof struct {
	Schema        string `json:"schema"`
	ClientID      string `json:"clientId"`
	ReleaseSHA256 string `json:"releaseSha256"`
	Challenge     string `json:"challenge"`
}
type Client struct {
	ID                 string     `json:"id"`
	Binding            Binding    `json:"binding"`
	Status             string     `json:"status"`
	Revision           int        `json:"revision"`
	ChallengeID        string     `json:"challengeId"`
	Challenge          string     `json:"challenge"`
	ChallengeExpiresAt time.Time  `json:"challengeExpiresAt"`
	VerifiedUntil      *time.Time `json:"verifiedUntil"`
	LastAttemptAt      *time.Time `json:"lastAttemptAt"`
	LastResult         string     `json:"lastResult"`
	SecretHash         string     `json:"-"`
	SecretVersion      int        `json:"secretVersion"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}
type Audit struct {
	ID         string    `json:"id"`
	ActorID    string    `json:"actorId"`
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurredAt"`
}
type View struct {
	Client       Client   `json:"client"`
	ProofURL     string   `json:"proofUrl"`
	Expected     Proof    `json:"expected"`
	Ready        bool     `json:"clientReady"`
	Installable  bool     `json:"installable"`
	OAuthEnabled bool     `json:"oauthEnabled"`
	Blockers     []string `json:"blockers"`
}
type Input struct {
	ReleaseID string `json:"releaseId"`
}
type Action struct {
	Action   string `json:"action"`
	Revision int    `json:"revision"`
	Reason   string `json:"reason"`
}
type Mutation struct {
	OrganizationID, ActorID, Key, Hash, ID, Action, Reason string
	Create                                                 *Client
	Apply                                                  func(Client) (Client, error)
}
type Repository interface {
	List(context.Context, string) ([]Client, error)
	Get(context.Context, string, string) (Client, error)
	History(context.Context, string) ([]Audit, error)
	Mutate(context.Context, Mutation) (Client, bool, error)
	WithClient(context.Context, string, func(Client) error) error
	FinishProbe(context.Context, string, int, bool) (Client, error)
}
type Releases interface {
	WithBinding(context.Context, string, string, func(Binding) error) error
}
type Verifier interface {
	Verify(context.Context, string, Proof) error
}
type Service struct {
	Repo       Repository
	Releases   Releases
	Developers developer.Service
	Verifier   Verifier
}

// WithReady retains release and client locks through a short dependent commit.
// Portal ownership is checked before entering the application callback.
func (s Service) WithReady(ctx context.Context, p identity.PortalPrincipal, id string, fn func(Client) error) error {
	if s.Repo == nil || s.Releases == nil {
		return fault.Unavailable
	}
	org, err := s.scope(ctx, p)
	if err != nil {
		return err
	}
	v, err := s.Repo.Get(ctx, org, id)
	if err != nil {
		return err
	}
	return s.Releases.WithBinding(ctx, v.Binding.OrganizationID, v.Binding.ReleaseID, func(b Binding) error {
		return s.WithBoundReady(ctx, id, b, fn)
	})
}

// WithBoundReady is an internal composition port, not portal authorization.
// The caller must already hold the authoritative signed release lock for b
// (and merchant assignment where applicable), then acquire launch/installation
// locks only inside fn. Never pass a binding supplied by an HTTP caller.
func (s Service) WithBoundReady(ctx context.Context, id string, b Binding, fn func(Client) error) error {
	if s.Repo == nil {
		return fault.Unavailable
	}
	if fn == nil || id == "" || b.ReleaseID == "" || b.OrganizationID == "" || b.AppID == "" || b.Digest == "" {
		return fault.Invalid
	}
	return s.Repo.WithClient(ctx, id, func(c Client) error {
		if c.ID != id || c.Binding != b || !current(c) || c.SecretHash == "" {
			return fault.Conflict
		}
		return fn(c)
	})
}

var requestKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)
var clientID = regexp.MustCompile(`^eac_[A-Z2-7]{26}$`)

func hash(s string) string     { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func fingerprint(v any) string { b, _ := json.Marshal(v); return hash(string(b)) }
func random() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
func (s Service) scope(ctx context.Context, p identity.PortalPrincipal) (string, error) {
	if p.ID != "" && p.Surface == "admin" {
		return "", nil
	}
	o, err := s.Developers.Organization(ctx, p)
	return o.ID, err
}
func proof(v Client) Proof { return Proof{ProofSchema, v.ID, v.Binding.Digest, v.Challenge} }
func proofURL(v Client) string {
	u, _ := url.Parse(v.Binding.Endpoint)
	if u == nil {
		return ""
	}
	return "https://" + u.Host + "/.well-known/emisell-app-verification/" + v.ChallengeID
}
func current(v Client) bool {
	return v.Status == "verified" && v.VerifiedUntil != nil && time.Now().Before(*v.VerifiedUntil)
}
func (s Service) view(ctx context.Context, v Client) View {
	out := View{Client: v, ProofURL: proofURL(v), Expected: proof(v), Blockers: []string{"Runtime umum dan OAuth authorization-code/token exchange belum tersedia. Client ini bukan grant tenant atau resource."}}
	if !current(v) {
		out.Blockers = append(out.Blockers, "Bukti kendali endpoint belum valid, kedaluwarsa, atau client dicabut.")
	}
	if v.SecretHash == "" {
		out.Blockers = append(out.Blockers, "Client secret belum diterbitkan atau sudah dicabut.")
	}
	err := s.Releases.WithBinding(ctx, v.Binding.OrganizationID, v.Binding.ReleaseID, func(b Binding) error {
		if b != v.Binding {
			return fault.Conflict
		}
		return nil
	})
	if err != nil {
		out.Blockers = append(out.Blockers, "Release tidak lagi signed/valid atau verifikasi release tidak tersedia.")
	}
	out.Ready = current(v) && v.SecretHash != "" && err == nil
	if v.LastResult == "checking" && v.LastAttemptAt != nil && time.Since(*v.LastAttemptAt) > time.Minute {
		out.Client.LastResult = "interrupted"
	}
	return out
}
func (s Service) List(ctx context.Context, p identity.PortalPrincipal) ([]View, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return nil, err
	}
	rows, err := s.Repo.List(ctx, org)
	if err != nil {
		return nil, err
	}
	out := []View{}
	for _, v := range rows {
		out = append(out, s.view(ctx, v))
	}
	return out, nil
}
func (s Service) Get(ctx context.Context, p identity.PortalPrincipal, id string) (View, []Audit, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return View{}, nil, err
	}
	v, err := s.Repo.Get(ctx, org, id)
	if err != nil {
		return View{}, nil, err
	}
	h, err := s.Repo.History(ctx, id)
	return s.view(ctx, v), h, err
}
func (s Service) Create(ctx context.Context, p identity.PortalPrincipal, key string, in Input) (View, error) {
	o, err := s.Developers.Organization(ctx, p)
	if err != nil {
		return View{}, err
	}
	if !requestKey.MatchString(key) {
		return View{}, fault.Invalid
	}
	var out Client
	err = s.Releases.WithBinding(ctx, o.ID, in.ReleaseID, func(b Binding) error {
		v := Client{ID: ids.New("eac"), Binding: b, Status: "pending", Revision: 1, ChallengeID: ids.New("proof"), Challenge: random(), ChallengeExpiresAt: time.Now().Add(ChallengeTTL), LastResult: "not_checked"}
		var e error
		out, _, e = s.Repo.Mutate(ctx, Mutation{OrganizationID: o.ID, ActorID: p.ID, Key: key, Hash: fingerprint([]any{"create", in}), Create: &v, Action: "registered", Reason: "Registrasi client dari konfigurasi signed."})
		return e
	})
	if err != nil {
		return View{}, err
	}
	return s.view(ctx, out), nil
}
func (s Service) Act(ctx context.Context, p identity.PortalPrincipal, id, key string, in Action) (View, string, error) {
	org, err := s.scope(ctx, p)
	if err != nil {
		return View{}, "", err
	}
	if p.Surface == "admin" && (p.Role != "administrator" || in.Action != "revoke") {
		return View{}, "", fault.Forbidden
	}
	if !requestKey.MatchString(key) || in.Revision < 1 || strings.TrimSpace(in.Reason) == "" || len(in.Reason) > 2000 || strings.ContainsFunc(in.Reason, unicode.IsControl) {
		return View{}, "", fault.Invalid
	}
	if in.Action != "challenge" && in.Action != "verify" && in.Action != "rotate_secret" && in.Action != "revoke" {
		return View{}, "", fault.Invalid
	}
	v, err := s.Repo.Get(ctx, org, id)
	if err != nil {
		return View{}, "", err
	}
	var out Client
	var changed bool
	var secret string
	mut := Mutation{OrganizationID: org, ActorID: p.ID, Key: key, Hash: fingerprint([]any{id, in}), ID: id, Action: in.Action, Reason: in.Reason}
	mut.Apply = func(c Client) (Client, error) {
		if c.Revision != in.Revision {
			return c, fault.Conflict
		}
		if in.Action == "revoke" {
			c.Status = "revoked"
			c.SecretHash = ""
			c.VerifiedUntil = nil
			c.Revision++
			return c, nil
		}
		if c.Status == "revoked" {
			return c, fault.Conflict
		}
		switch in.Action {
		case "challenge":
			c.ChallengeID = ids.New("proof")
			c.Challenge = random()
			c.ChallengeExpiresAt = time.Now().Add(ChallengeTTL)
			c.Status = "pending"
			c.VerifiedUntil = nil
			c.SecretHash = ""
			c.LastResult = "not_checked"
		case "verify":
			if s.Verifier == nil {
				return c, fault.Unavailable
			}
			if c.Status != "pending" || !time.Now().Before(c.ChallengeExpiresAt) || (c.LastAttemptAt != nil && time.Since(*c.LastAttemptAt) < time.Minute) {
				return c, fault.Conflict
			}
			now := time.Now()
			c.LastAttemptAt = &now
			c.LastResult = "checking"
		case "rotate_secret":
			if !current(c) {
				return c, fault.Conflict
			}
			secret = "eacs_" + random()
			c.SecretHash = hash(secret)
			c.SecretVersion++
		}
		c.Revision++
		return c, nil
	}
	apply := func() error { var e error; out, changed, e = s.Repo.Mutate(ctx, mut); return e }
	if in.Action == "revoke" {
		err = apply()
	} else {
		err = s.Releases.WithBinding(ctx, v.Binding.OrganizationID, v.Binding.ReleaseID, func(b Binding) error {
			if b != v.Binding {
				return fault.Conflict
			}
			return apply()
		})
	}
	if err != nil {
		return View{}, "", err
	}
	if !changed {
		secret = ""
	}
	if in.Action == "verify" && changed {
		// No database lock is held during outbound I/O. Revision and release are
		// rechecked at completion, so revoke/renew/suspend wins over stale probes.
		probeErr := s.Verifier.Verify(ctx, proofURL(out), proof(out))
		err = s.Releases.WithBinding(ctx, out.Binding.OrganizationID, out.Binding.ReleaseID, func(b Binding) error {
			if b != out.Binding {
				return fault.Conflict
			}
			var e error
			out, e = s.Repo.FinishProbe(ctx, out.ID, out.Revision, probeErr == nil)
			return e
		})
		if err != nil {
			return View{}, "", err
		}
	}
	return s.view(ctx, out), secret, nil
}

// Authenticate is a bounded self-check, not an OAuth grant or token exchange.
func (s Service) Authenticate(ctx context.Context, id, secret string) (Binding, error) {
	if !clientID.MatchString(id) || !strings.HasPrefix(secret, "eacs_") || len(secret) != 48 {
		return Binding{}, fault.Unauthenticated
	}
	v, err := s.Repo.Get(ctx, "", id)
	if err != nil {
		return Binding{}, fault.Unauthenticated
	}
	var binding Binding
	err = s.Releases.WithBinding(ctx, v.Binding.OrganizationID, v.Binding.ReleaseID, func(b Binding) error {
		if b != v.Binding {
			return fault.Unauthenticated
		}
		return s.Repo.WithClient(ctx, id, func(c Client) error {
			if !current(c) || c.Binding != b || c.SecretHash == "" || subtle.ConstantTimeCompare([]byte(c.SecretHash), []byte(hash(secret))) != 1 {
				return fault.Unauthenticated
			}
			binding = b
			return nil
		})
	})
	if err != nil {
		return Binding{}, fault.Unauthenticated
	}
	return binding, nil
}
