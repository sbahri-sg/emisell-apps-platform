// Package oauth implements the platform-side connection lifecycle. It never exposes tokens to the browser.
package oauth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/installation/domain"
	installation "emisell.app/platform/internal/installation/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/internal/platform/localhttp"
	"emisell.app/platform/internal/platform/secretbox"
	"emisell.app/platform/pkg/appmanifest"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

type State struct {
	Hash, Tenant, Installation, Actor, Session, Key string
	Material                                        []byte
	Expires                                         time.Time
}
type Repository interface {
	Begin(context.Context, State) (State, error)
	Consume(context.Context, string, string, string) (State, error)
	Load(context.Context, string, string) (string, []byte, error)
	Save(context.Context, string, string, string, string, []byte) error
	Invalidate(context.Context, string, string) error
}
type Gate interface {
	WithInstallation(context.Context, string, string, func(domain.Installation) error) error
}
type Registry interface {
	Get(context.Context, string) (appmanifest.Manifest, error)
}
type Service struct {
	Repo   Repository
	Gate   Gate
	Apps   Registry
	Auth   identity.Authorizer
	Config localfiles.RemoteConfig
	Box    secretbox.Box
	HTTP   *http.Client
}
type material struct{ State, Verifier string }
type credentials struct {
	Token         oauth2.Token `json:"token"`
	WebhookSecret string       `json:"webhookSecret"`
}
type Connected struct{ AccessToken, WebhookSecret string }

func Hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func New(repo Repository, gate Gate, apps Registry, auth identity.Authorizer, cfg localfiles.RemoteConfig) (*Service, error) {
	h, err := localhttp.Client(cfg.Origin)
	if err != nil {
		return nil, err
	}
	u, err := url.Parse(cfg.AuthorizationURL)
	if err != nil || u.Scheme != "http" || u.Hostname() != "localhost" || u.Path != "/oauth/authorize" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return nil, errors.New("invalid local authorization URL")
	}
	origin, _ := url.Parse(cfg.Origin)
	if u.Port() != origin.Port() || cfg.CallbackURL != "http://localhost:4317/api/v1/oauth/callback" || cfg.ClientID == "" || len(cfg.ClientSecret) < 32 {
		return nil, errors.New("invalid local OAuth configuration")
	}
	box, err := secretbox.New(cfg.EncryptionKey)
	if err != nil {
		return nil, err
	}
	return &Service{Repo: repo, Gate: gate, Apps: apps, Auth: auth, Config: cfg, Box: box, HTTP: h}, nil
}
func (s *Service) config() oauth2.Config {
	return oauth2.Config{ClientID: s.Config.ClientID, ClientSecret: s.Config.ClientSecret, RedirectURL: s.Config.CallbackURL, Scopes: []string{"orders.read", "payments.read", "payments.write"}, Endpoint: oauth2.Endpoint{AuthURL: s.Config.AuthorizationURL, TokenURL: s.Config.Origin + "/oauth/token", AuthStyle: oauth2.AuthStyleInHeader}}
}
func (s *Service) context(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, s.HTTP)
}
func (s *Service) check(ctx context.Context, ins domain.Installation) error {
	if ins.Status != "pending" && ins.Status != "active" {
		return fault.Conflict
	}
	app, err := s.Apps.Get(ctx, ins.AppID)
	if err != nil {
		return err
	}
	if app.ExecutionProfile != "local-remote" || app.Version != ins.Version || !slices.Equal(app.Scopes, ins.Scopes) {
		return fault.Forbidden
	}
	return nil
}
func (s *Service) Begin(ctx context.Context, user, session, tenant, insID, key string) (string, error) {
	if !installation.KeyPattern.MatchString(key) {
		return "", fault.Invalid
	}
	if err := s.Auth.Authorize(ctx, user, tenant); err != nil {
		return "", err
	}
	var authorize string
	err := s.Gate.WithInstallation(ctx, tenant, insID, func(ins domain.Installation) error {
		if err := s.check(ctx, ins); err != nil {
			return err
		}
		m := material{State: rand.Text() + rand.Text(), Verifier: oauth2.GenerateVerifier()}
		raw, _ := json.Marshal(m)
		record, err := s.Repo.Begin(ctx, State{Hash: Hash(m.State), Tenant: tenant, Installation: insID, Actor: user, Session: Hash(session), Key: key, Material: s.Box.Seal("state:"+tenant+":"+insID, raw), Expires: time.Now().Add(5 * time.Minute)})
		if err != nil {
			return err
		}
		raw, err = s.Box.Open("state:"+tenant+":"+insID, record.Material)
		if err != nil {
			return fault.Unavailable
		}
		if json.Unmarshal(raw, &m) != nil {
			return fault.Unavailable
		}
		cfg := s.config()
		authorize = cfg.AuthCodeURL(m.State, oauth2.S256ChallengeOption(m.Verifier), oauth2.SetAuthURLParam("tenant_id", tenant), oauth2.SetAuthURLParam("installation_id", insID))
		return nil
	})
	return authorize, err
}
func (s *Service) Callback(ctx context.Context, user, session, state, code string) (string, error) {
	if len(state) > 128 || len(state) < 32 || len(code) > 256 {
		return "", fault.Invalid
	}
	record, err := s.Repo.Consume(ctx, Hash(state), user, Hash(session))
	if err != nil {
		return "", err
	}
	if err = s.Auth.Authorize(ctx, user, record.Tenant); err != nil {
		return "", err
	}
	err = s.Gate.WithInstallation(ctx, record.Tenant, record.Installation, func(ins domain.Installation) error {
		if err := s.check(ctx, ins); err != nil {
			return err
		}
		if code == "" {
			return fault.Invalid
		}
		raw, err := s.Box.Open("state:"+record.Tenant+":"+record.Installation, record.Material)
		if err != nil {
			return fault.Unavailable
		}
		var m material
		if json.Unmarshal(raw, &m) != nil {
			return fault.Unavailable
		}
		cfg := s.config()
		token, err := cfg.Exchange(s.context(ctx), code, oauth2.VerifierOption(m.Verifier))
		if err != nil {
			return fault.Unavailable
		}
		cred, err := validateToken(token, record.Tenant, record.Installation)
		if err != nil {
			return err
		}
		return s.save(ctx, record.Tenant, record.Installation, user, cred)
	})
	return record.Tenant, err
}
func validateToken(t *oauth2.Token, tenant, ins string) (credentials, error) {
	var c credentials
	if t == nil || !strings.EqualFold(t.TokenType, "Bearer") || len(t.AccessToken) < 32 || len(t.RefreshToken) < 32 || t.Expiry.Before(time.Now()) || t.Expiry.After(time.Now().Add(10*time.Minute)) || t.Extra("tenant_id") != tenant || t.Extra("installation_id") != ins || t.Extra("scope") != "orders.read payments.read payments.write" {
		return c, fault.Unavailable
	}
	hook, _ := t.Extra("webhook_secret").(string)
	if len(hook) < 32 || len(hook) > 128 {
		return c, fault.Unavailable
	}
	return credentials{Token: *t, WebhookSecret: hook}, nil
}
func (s *Service) save(ctx context.Context, tenant, ins, actor string, c credentials) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return s.Repo.Save(ctx, tenant, ins, actor, "connected", s.Box.Seal("connection:"+tenant+":"+ins, raw))
}
func (s *Service) Status(ctx context.Context, tenant, ins string) (string, error) {
	status, _, err := s.Repo.Load(ctx, tenant, ins)
	return status, err
}
func (s *Service) Disconnect(ctx context.Context, tenant, ins string) error {
	return s.Repo.Save(ctx, tenant, ins, "system-remote", "needs_connection", nil)
}

// Access must run inside the installation lifecycle gate, including token rotation.
func (s *Service) Access(ctx context.Context, tenant, ins string) (Connected, error) {
	status, raw, err := s.Repo.Load(ctx, tenant, ins)
	if err != nil {
		return Connected{}, err
	}
	if status != "connected" {
		return Connected{}, fault.Conflict
	}
	raw, err = s.Box.Open("connection:"+tenant+":"+ins, raw)
	if err != nil {
		return Connected{}, fault.Unavailable
	}
	var c credentials
	if json.Unmarshal(raw, &c) != nil {
		return Connected{}, fault.Unavailable
	}
	if c.Token.Expiry.Before(time.Now().Add(15 * time.Second)) {
		cfg := s.config()
		expired := c.Token
		expired.Expiry = time.Now().Add(-time.Minute)
		token, refreshErr := cfg.TokenSource(s.context(ctx), &expired).Token()
		if refreshErr == nil {
			c, refreshErr = validateToken(token, tenant, ins)
		}
		if refreshErr != nil {
			if err = s.Repo.Save(ctx, tenant, ins, "system-refresh", "needs_connection", nil); err != nil {
				return Connected{}, err
			}
			return Connected{}, fault.Unavailable
		}
		if err = s.save(ctx, tenant, ins, "system-refresh", c); err != nil {
			return Connected{}, err
		}
	}
	return Connected{AccessToken: c.Token.AccessToken, WebhookSecret: c.WebhookSecret}, nil
}
func (s *Service) Ready(ctx context.Context, tenant, ins string) error {
	c, err := s.Access(ctx, tenant, ins)
	if err != nil {
		return err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.Config.Origin+"/v1/connection", nil)
	req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return fault.Unavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 {
		if err = s.Disconnect(ctx, tenant, ins); err != nil {
			return err
		}
		return fault.Conflict
	}
	if resp.StatusCode != 200 {
		return fault.Unavailable
	}
	var binding struct {
		Tenant       string `json:"tenantId"`
		Installation string `json:"installationId"`
	}
	if json.NewDecoder(resp.Body).Decode(&binding) != nil || binding.Tenant != tenant || binding.Installation != ins {
		return fault.Unavailable
	}
	return nil
}
func (s *Service) Revoke(ctx context.Context, tenant, ins string) error {
	// Fail closed locally before attempting remote cleanup; retry never needs the old token.
	if err := s.Repo.Invalidate(ctx, tenant, ins); err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]string{"tenantId": tenant, "installationId": ins})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, s.Config.Origin+"/v1/installations/revoke", bytes.NewReader(raw))
	req.SetBasicAuth(s.Config.ClientID, s.Config.ClientSecret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.HTTP.Do(req)
	if err != nil {
		return fault.Unavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fault.Unavailable
	}
	return nil
}
