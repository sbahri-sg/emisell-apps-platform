// Package referenceapp is a LOCAL TEST FIXTURE, not a production OAuth provider.
// Its DB access is confined to reference_remote; it consumes public app contracts.
package referenceapp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/platform/localfiles"
	"emisell.app/platform/internal/platform/secretbox"
	"emisell.app/platform/internal/runtime/simulator"
	"emisell.app/platform/pkg/appapi"
	"emisell.app/platform/pkg/sdk/events"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//go:embed schema.sql
var schema string

//go:embed callbacks.sql
var callbacksSchema string

func Init(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, schema)
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, callbacksSchema)
	return err
}

type Server struct {
	Pool   *pgxpool.Pool
	Config localfiles.RemoteConfig
	Box    *secretbox.Box
}
type authorization struct{ Tenant, Installation, State, Challenge, Redirect string }
type grant struct {
	ID, Tenant, Installation, Hook string
	Local                          bool
}

func hash(v string) string { s := sha256.Sum256([]byte(v)); return hex.EncodeToString(s[:]) }
func random() string       { return rand.Text() + rand.Text() }
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, err error) {
	status := 503
	switch {
	case errors.Is(err, fault.Invalid):
		status = 400
	case errors.Is(err, fault.Unauthenticated):
		status = 401
	case errors.Is(err, fault.Forbidden):
		status = 403
	case errors.Is(err, fault.NotFound):
		status = 404
	case errors.Is(err, fault.Conflict):
		status = 409
	}
	write(w, status, map[string]string{"error": http.StatusText(status)})
}
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(&struct{}{}) != io.EOF {
		return fault.Invalid
	}
	return nil
}
func lock(ctx context.Context, tx pgx.Tx, key string) error {
	_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", "reference-app:"+key)
	return err
}
func (s Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{"mode": "local-reference-only"})
	})
	mux.HandleFunc("GET /oauth/authorize", s.authorize)
	mux.HandleFunc("POST /oauth/authorize", s.approve)
	mux.HandleFunc("POST /oauth/token", s.token)
	mux.HandleFunc("POST /v1/installations/revoke", s.revoke)
	mux.HandleFunc("GET /v1/connection", func(w http.ResponseWriter, r *http.Request) {
		g, err := s.authenticate(r)
		if err != nil {
			fail(w, err)
			return
		}
		write(w, 200, map[string]string{"tenantId": g.Tenant, "installationId": g.Installation})
	})
	mux.HandleFunc("POST /v1/capabilities/invoke", s.invoke)
	mux.HandleFunc("POST /v1/webhooks", s.webhook)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := strings.Cut(r.Host, ":")
		if host != "localhost" && host != "127.0.0.1" {
			fail(w, fault.Forbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		// Keep the same-origin form's Origin header; suppress the referrer on
		// the cross-origin callback so authorization parameters cannot leak.
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		callback, _ := url.Parse(s.Config.CallbackURL)
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self' "+callback.Scheme+"://"+callback.Host+"; frame-ancestors 'none'; base-uri 'none'")
		r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		recorder := &responseStatus{ResponseWriter: w, status: 200}
		request := r.WithContext(ctx)
		mux.ServeHTTP(recorder, request)
		slog.Info("reference request", "method", r.Method, "route", request.Pattern, "status", recorder.status)
	})
}

type responseStatus struct {
	http.ResponseWriter
	status int
}

func (w *responseStatus) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

var consent = template.Must(template.New("consent").Parse(`<!doctype html><html lang="id"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Hubungkan Remote Pay</title><style>body{margin:0;background:#f3f5fa;color:#172038;font:16px system-ui;display:grid;min-height:100vh;place-items:center}main{background:white;border:1px solid #e1e6f0;border-radius:24px;padding:40px;max-width:460px;box-shadow:0 20px 80px #27305612}small{color:#6951dc;font-weight:700}h1{font-size:30px}p,li{line-height:1.7;color:#52607a}code{overflow-wrap:anywhere}button{border:0;border-radius:12px;padding:14px 18px;background:#6551d9;color:white;font:inherit;cursor:pointer}.secondary{background:#edf0f7;color:#334155;margin-left:8px}.notice{padding:14px;background:#f1efff;border-radius:12px}</style><main><small>REMOTE PAY · LOCAL REFERENCE</small><h1>Hubungkan aplikasi</h1><p>Workspace <code>{{.Tenant}}</code><br>Installation <code>{{.Installation}}</code></p><ul><li>Membaca data pesanan</li><li>Membaca dan membuat pembayaran simulasi</li><li>Menerima event capability bertanda tangan</li></ul><p class="notice">Ini aplikasi contoh di komputer Anda. Tidak ada akun provider asli, pembayaran nyata, atau data produksi.</p><form method="post" action="/oauth/authorize"><input type="hidden" name="code" value="{{.Code}}"><input type="hidden" name="form" value="{{.Form}}"><button name="decision" value="allow">Izinkan koneksi lokal</button><button class="secondary" name="decision" value="deny">Batalkan</button></form></main></html>`))

func (s Server) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	a := authorization{Tenant: q.Get("tenant_id"), Installation: q.Get("installation_id"), State: q.Get("state"), Challenge: q.Get("code_challenge"), Redirect: q.Get("redirect_uri")}
	challenge, err := base64.RawURLEncoding.DecodeString(a.Challenge)
	if err != nil || len(challenge) != 32 || q.Get("code_challenge_method") != "S256" || q.Get("response_type") != "code" || q.Get("client_id") != s.Config.ClientID || a.Redirect != s.Config.CallbackURL || q.Get("scope") != "orders.read payments.read payments.write" || !events.Token.MatchString(a.Tenant) || !events.Token.MatchString(a.Installation) || len(a.State) < 32 || len(a.State) > 128 {
		fail(w, fault.Invalid)
		return
	}
	code, form := random(), random()
	raw, _ := json.Marshal(a)
	_, err = s.Pool.Exec(r.Context(), "INSERT INTO reference_remote.codes(hash,form_hash,request) VALUES($1,$2,$3)", hash(code), hash(form), raw)
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = consent.Execute(w, map[string]string{"Tenant": a.Tenant, "Installation": a.Installation, "Code": code, "Form": form})
}
func (s Server) approve(w http.ResponseWriter, r *http.Request) {
	authURL, _ := url.Parse(s.Config.AuthorizationURL)
	if r.Header.Get("Origin") != authURL.Scheme+"://"+authURL.Host || r.ParseForm() != nil {
		fail(w, fault.Forbidden)
		return
	}
	allow := r.PostForm.Get("decision") == "allow"
	if !allow && r.PostForm.Get("decision") != "deny" {
		fail(w, fault.Invalid)
		return
	}
	var a authorization
	err := s.Pool.QueryRow(r.Context(), `UPDATE reference_remote.codes SET approved=$3,used=$4 WHERE hash=$1 AND form_hash=$2 AND NOT approved AND NOT used AND expires_at>now() RETURNING request`, hash(r.PostForm.Get("code")), hash(r.PostForm.Get("form")), allow, !allow).Scan(&a)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.Invalid
	}
	if err != nil {
		fail(w, err)
		return
	}
	target, _ := url.Parse(a.Redirect)
	q := url.Values{"state": {a.State}}
	if allow {
		q.Set("code", r.PostForm.Get("code"))
	} else {
		q.Set("error", "access_denied")
	}
	target.RawQuery = q.Encode()
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}
func (s Server) client(r *http.Request) bool {
	u, p, ok := r.BasicAuth()
	return ok && subtle.ConstantTimeCompare([]byte(u), []byte(s.Config.ClientID)) == 1 && subtle.ConstantTimeCompare([]byte(p), []byte(s.Config.ClientSecret)) == 1
}
func (s Server) token(w http.ResponseWriter, r *http.Request) {
	if !s.client(r) {
		write(w, 401, map[string]string{"error": "invalid_client"})
		return
	}
	if r.ParseForm() != nil {
		write(w, 400, map[string]string{"error": "invalid_request"})
		return
	}
	result, err := s.exchange(r.Context(), r.PostForm)
	if err != nil {
		write(w, 400, map[string]string{"error": "invalid_grant"})
		return
	}
	write(w, 200, result)
}
func (s Server) exchange(ctx context.Context, f url.Values) (map[string]any, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// All fixture grants are serialized, including refresh replay and revocation.
	if err = lock(ctx, tx, "grants"); err != nil {
		return nil, err
	}
	var g grant
	switch f.Get("grant_type") {
	case "authorization_code":
		var a authorization
		err = tx.QueryRow(ctx, "SELECT request FROM reference_remote.codes WHERE hash=$1 AND approved AND NOT used AND expires_at>now() FOR UPDATE", hash(f.Get("code"))).Scan(&a)
		if err != nil {
			return nil, fault.Invalid
		}
		verifier := f.Get("code_verifier")
		challenge := sha256.Sum256([]byte(verifier))
		if len(verifier) < 43 || len(verifier) > 128 || base64.RawURLEncoding.EncodeToString(challenge[:]) != a.Challenge || f.Get("redirect_uri") != a.Redirect {
			return nil, fault.Invalid
		}
		_, err = tx.Exec(ctx, "UPDATE reference_remote.codes SET used=true WHERE hash=$1", hash(f.Get("code")))
		if err != nil {
			return nil, err
		}
		_, err = tx.Exec(ctx, "UPDATE reference_remote.grants SET revoked=true WHERE tenant_id=$1 AND installation_id=$2", a.Tenant, a.Installation)
		if err != nil {
			return nil, err
		}
		g = grant{ID: ids.New("grant"), Tenant: a.Tenant, Installation: a.Installation, Hook: random()}
	case "refresh_token":
		var sealed []byte
		err = tx.QueryRow(ctx, "SELECT id,tenant_id,installation_id,hook FROM reference_remote.grants WHERE refresh_hash=$1 AND NOT revoked AND expires_at>now() FOR UPDATE", hash(f.Get("refresh_token"))).Scan(&g.ID, &g.Tenant, &g.Installation, &sealed)
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, "UPDATE reference_remote.grants SET revoked=true WHERE id IN (SELECT grant_id FROM reference_remote.refresh_history WHERE hash=$1)", hash(f.Get("refresh_token")))
			if err != nil {
				return nil, err
			}
			if err = tx.Commit(ctx); err != nil {
				return nil, err
			}
			return nil, fault.Invalid
		}
		if err != nil {
			return nil, err
		}
		raw, err := s.Box.Open("hook:"+g.Tenant+":"+g.Installation, sealed)
		if err != nil {
			return nil, err
		}
		g.Hook = string(raw)
		_, err = tx.Exec(ctx, "INSERT INTO reference_remote.refresh_history(hash,grant_id) VALUES($1,$2)", hash(f.Get("refresh_token")), g.ID)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fault.Invalid
	}
	access, refresh := random(), random()
	sealed := s.Box.Seal("hook:"+g.Tenant+":"+g.Installation, []byte(g.Hook))
	_, err = tx.Exec(ctx, `INSERT INTO reference_remote.grants(id,tenant_id,installation_id,access_hash,refresh_hash,hook,access_expires) VALUES($1,$2,$3,$4,$5,$6,now()+interval '5 minutes') ON CONFLICT(id) DO UPDATE SET access_hash=excluded.access_hash,refresh_hash=excluded.refresh_hash,access_expires=excluded.access_expires`, g.ID, g.Tenant, g.Installation, hash(access), hash(refresh), sealed)
	if err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, "INSERT INTO reference_remote.audit(tenant_id,installation_id,action) VALUES($1,$2,$3)", g.Tenant, g.Installation, f.Get("grant_type"))
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"access_token": access, "refresh_token": refresh, "token_type": "Bearer", "expires_in": 300, "scope": "orders.read payments.read payments.write", "tenant_id": g.Tenant, "installation_id": g.Installation, "webhook_secret": g.Hook}, nil
}
func (s Server) authenticate(r *http.Request) (grant, error) {
	var g grant
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || len(token) > 128 {
		return g, fault.Unauthenticated
	}
	err := s.Pool.QueryRow(r.Context(), "SELECT id,tenant_id,installation_id FROM reference_remote.grants WHERE access_hash=$1 AND NOT revoked AND access_expires>now() AND expires_at>now()", hash(token)).Scan(&g.ID, &g.Tenant, &g.Installation)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.Unauthenticated
	}
	return g, err
}
func (s Server) revoke(w http.ResponseWriter, r *http.Request) {
	if !s.client(r) {
		fail(w, fault.Unauthenticated)
		return
	}
	var body struct {
		Tenant       string `json:"tenantId"`
		Installation string `json:"installationId"`
	}
	if decode(r, &body) != nil || !events.Token.MatchString(body.Tenant) || !events.Token.MatchString(body.Installation) {
		fail(w, fault.Invalid)
		return
	}
	tx, err := s.Pool.Begin(r.Context())
	if err != nil {
		fail(w, err)
		return
	}
	defer tx.Rollback(r.Context())
	if err = lock(r.Context(), tx, "grants"); err == nil {
		_, err = tx.Exec(r.Context(), "UPDATE reference_remote.grants SET revoked=true WHERE tenant_id=$1 AND installation_id=$2", body.Tenant, body.Installation)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), "UPDATE reference_remote.codes SET used=true WHERE request->>'Tenant'=$1 AND request->>'Installation'=$2", body.Tenant, body.Installation)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), "INSERT INTO reference_remote.audit(tenant_id,installation_id,action) VALUES($1,$2,'revoked')", body.Tenant, body.Installation)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		fail(w, err)
		return
	}
	write(w, 200, map[string]bool{"revoked": true})
}
func (s Server) invoke(w http.ResponseWriter, r *http.Request) {
	g, err := s.authenticate(r)
	if err != nil {
		fail(w, err)
		return
	}
	var request appapi.Invocation
	if decode(r, &request) != nil || request.TenantID != g.Tenant || request.InstallationID != g.Installation || request.Capability != "payment/v1" || !events.Token.MatchString(request.IdempotencyKey) {
		fail(w, fault.Forbidden)
		return
	}
	result, err := s.execute(r.Context(), g, request)
	if err != nil {
		fail(w, err)
		return
	}
	write(w, 200, result)
}
func (s Server) execute(ctx context.Context, g grant, in appapi.Invocation) (appapi.Response, error) {
	var result appapi.Response
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	// Grant lock prevents an accepted invocation from racing completed revocation.
	if err = lock(ctx, tx, "grants"); err != nil {
		return result, err
	}
	var valid bool
	err = tx.QueryRow(ctx, "SELECT NOT revoked AND ($2 OR access_expires>now()) AND expires_at>now() FROM reference_remote.grants WHERE id=$1", g.ID, g.Local).Scan(&valid)
	if err != nil {
		return result, err
	}
	if !valid {
		return result, fault.Unauthenticated
	}
	raw, _ := json.Marshal(in)
	requestHash := hash(string(raw))
	var oldHash string
	err = tx.QueryRow(ctx, "SELECT hash,response FROM reference_remote.idempotency WHERE tenant_id=$1 AND installation_id=$2 AND key=$3", g.Tenant, g.Installation, in.IdempotencyKey).Scan(&oldHash, &result)
	if err == nil {
		if requestHash != oldHash {
			return result, fault.Conflict
		}
		return result, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return result, err
	}
	var current *appapi.Resource
	if in.Request.ResourceID != "" {
		var c appapi.Resource
		err = tx.QueryRow(ctx, "SELECT data FROM reference_remote.resources WHERE tenant_id=$1 AND installation_id=$2 AND id=$3", g.Tenant, g.Installation, in.Request.ResourceID).Scan(&c)
		if errors.Is(err, pgx.ErrNoRows) {
			err = fault.NotFound
		}
		if err != nil {
			return result, err
		}
		current = &c
	}
	resource, rates, err := (simulator.Runtime{}).Execute(in.Capability, in.Request, current)
	if err != nil {
		return result, err
	}
	result = appapi.Response{Capability: in.Capability, Operation: in.Request.Operation, InstallationID: g.Installation, Simulation: true, Resource: resource, Rates: rates}
	if resource != nil {
		raw, _ = json.Marshal(resource)
		_, err = tx.Exec(ctx, "INSERT INTO reference_remote.resources(tenant_id,installation_id,id,data) VALUES($1,$2,$3,$4) ON CONFLICT(tenant_id,installation_id,id) DO UPDATE SET data=excluded.data", g.Tenant, g.Installation, resource.ID, raw)
		if err != nil {
			return result, err
		}
		if current == nil || resource.Status != current.Status {
			update := appapi.PaymentUpdate{Type: appapi.PaymentUpdateType, Resource: *resource, OccurredAt: time.Now().UTC()}
			raw, _ = json.Marshal(update)
			_, err = tx.Exec(ctx, "INSERT INTO reference_remote.callbacks(id,tenant_id,installation_id,owner_key,body) VALUES($1,$2,$3,$4,$5)", ids.New("callback"), g.Tenant, g.Installation, hash(s.Config.EncryptionKey), raw)
			if err != nil {
				return result, err
			}
		}
	}
	raw, _ = json.Marshal(result)
	_, err = tx.Exec(ctx, "INSERT INTO reference_remote.idempotency(tenant_id,installation_id,key,hash,response) VALUES($1,$2,$3,$4,$5)", g.Tenant, g.Installation, in.IdempotencyKey, requestHash, raw)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
func (s Server) webhook(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		fail(w, fault.Invalid)
		return
	}
	tenant, ins, delivery := r.Header.Get("X-Emisell-Tenant"), r.Header.Get("X-Emisell-Installation"), r.Header.Get("X-Emisell-Delivery")
	e, err := events.Decode(raw)
	if err != nil || e.TenantID != tenant || e.Subject != ins || e.Type != "emisell.capability.invoked.v1" || !events.Token.MatchString(delivery) {
		fail(w, fault.Invalid)
		return
	}
	err = s.receive(r.Context(), tenant, ins, delivery, raw, r.Header.Get("X-Emisell-Timestamp"), r.Header.Get("X-Emisell-Signature"), e.ID)
	if err != nil {
		fail(w, err)
		return
	}
	write(w, 200, map[string]bool{"received": true})
}
func (s Server) receive(ctx context.Context, tenant, ins, delivery string, raw []byte, timestamp, signature, eventID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = lock(ctx, tx, "grants"); err != nil {
		return err
	}
	var sealed []byte
	err = tx.QueryRow(ctx, "SELECT hook FROM reference_remote.grants WHERE tenant_id=$1 AND installation_id=$2 AND NOT revoked AND expires_at>now()", tenant, ins).Scan(&sealed)
	if errors.Is(err, pgx.ErrNoRows) {
		return fault.Unauthenticated
	}
	if err != nil {
		return err
	}
	key, err := s.Box.Open("hook:"+tenant+":"+ins, sealed)
	if err != nil {
		return err
	}
	if !appapi.VerifyWebhook(string(key), tenant, ins, delivery, timestamp, signature, raw, time.Now()) {
		return fault.Unauthenticated
	}
	bodyHash := hash(string(raw))
	var prior string
	err = tx.QueryRow(ctx, "SELECT body_hash FROM reference_remote.webhook_inbox WHERE tenant_id=$1 AND installation_id=$2 AND (delivery_id=$3 OR event_id=$4)", tenant, ins, delivery, eventID).Scan(&prior)
	if err == nil {
		if prior != bodyHash {
			return fault.Conflict
		}
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO reference_remote.webhook_inbox(tenant_id,installation_id,delivery_id,event_id,body_hash) VALUES($1,$2,$3,$4,$5)", tenant, ins, delivery, eventID, bodyHash)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "INSERT INTO reference_remote.audit(tenant_id,installation_id,action) VALUES($1,$2,'webhook_received')", tenant, ins)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
