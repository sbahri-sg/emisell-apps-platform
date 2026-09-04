package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

//go:embed page.html style.css
var assets embed.FS
var pageTemplate = template.Must(template.ParseFS(assets, "page.html"))
var opaqueID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
var requestID = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

const cookieName = "emisell_product_reader_session"

type reader struct {
	config configuration
	store  *store
	client *http.Client
}
type product struct {
	ID, Name, SKU, Price string
	Stock                *int64
	IsPublished          bool
}
type productPage struct {
	Data []product
	Meta struct {
		NextCursor *string `json:"nextCursor"`
	}
}
type view struct {
	Session                             session
	Products                            []product
	NextCursor, Error, ConnectedAppsURL string
	Connected                           bool
}

func randomString() string {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		panic("secure random unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(data)
}
func newReader(config configuration, storage *store) *reader {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &reader{config: config, store: storage, client: &http.Client{Transport: transport, Timeout: 5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

func (a *reader) handler() http.Handler {
	consent, _ := url.Parse(a.config.ConsentURL)
	consentOrigin := consent.Scheme + "://" + consent.Host
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.home)
	mux.HandleFunc("POST /connect", a.connect)
	mux.HandleFunc("GET /products", a.products)
	mux.HandleFunc("GET /api/products", a.productsJSON)
	mux.HandleFunc("GET /api/products/{productId}", a.productsJSON)
	mux.HandleFunc("GET /style.css", func(w http.ResponseWriter, r *http.Request) {
		data, _ := assets.ReadFile("style.css")
		w.Header().Set("Content-Type", "text/css")
		_, _ = w.Write(data)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		// no-referrer makes native form POST Origin=null in browsers, breaking our
		// exact-origin CSRF check. same-origin retains it without cross-origin referrers.
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		// Browsers also apply form-action to the POST /connect redirect chain.
		// Permit only the operator-configured consent origin, never arbitrary targets.
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; form-action 'self' "+consentOrigin+"; frame-ancestors 'none'; base-uri 'none'")
		public, _ := url.Parse(a.config.PublicURL)
		if r.Host != public.Host {
			http.Error(w, "Invalid host", 400)
			return
		}
		if r.Method == http.MethodHead {
			// HEAD probes must not consume a single-use OAuth callback or change sessions.
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		mux.ServeHTTP(w, r)
		if a.config.LocalDevelopment {
			// Route templates only: never log query strings, merchant IDs, or headers.
			log.Printf("Product Reader local request: %s", r.Pattern)
		}
	})
}

func (a *reader) browserSession(w http.ResponseWriter, r *http.Request, create bool) (string, session, bool) {
	if cookie, err := r.Cookie(cookieName); err == nil && len(cookie.Value) == 43 {
		if item, ok := a.store.get(cookie.Value); ok {
			return cookie.Value, item, true
		}
	}
	if !create {
		return "", session{}, false
	}
	id := randomString()
	item := session{CSRF: randomString(), ExpiresAt: time.Now().Add(12 * time.Hour)}
	if a.store.change(id, func(s *session) error { *s = item; return nil }) != nil {
		return "", session{}, false
	}
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: id, Path: "/", HttpOnly: true, Secure: strings.HasPrefix(a.config.PublicURL, "https://"), SameSite: http.SameSiteLaxMode, MaxAge: 12 * 3600})
	return id, item, true
}

func (a *reader) render(w http.ResponseWriter, status int, data view) {
	data.ConnectedAppsURL = a.config.ConnectedAppsURL
	data.Connected = data.Session.Token != "" && data.Session.TokenExpiresAt.After(time.Now())
	// Never give templates credentials, tokens, state or verifier values.
	data.Session.Token = ""
	data.Session.StateHash = ""
	data.Session.Verifier = ""
	data.Session.PendingRequest = ""
	var body bytes.Buffer
	if pageTemplate.Execute(&body, data) != nil {
		http.Error(w, "Cannot render page", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = body.WriteTo(w)
}

func (a *reader) home(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Has("code") || query.Has("error") || query.Has("state") {
		a.callback(w, r)
		return
	}
	id, item, ok := a.browserSession(w, r, true)
	if !ok {
		http.Error(w, "Cannot create example session", 503)
		return
	}
	if query.Has("emisell_test_install_request") {
		if len(query) != 1 || len(query["emisell_test_install_request"]) != 1 || !requestID.MatchString(query.Get("emisell_test_install_request")) {
			http.Error(w, "Invalid development install link", 400)
			return
		}
		if a.store.change(id, func(s *session) error {
			s.PendingRequest = query.Get("emisell_test_install_request")
			s.Notice = "Link development siap. Lanjutkan untuk meninjau izin di Emisell."
			return nil
		}) != nil {
			http.Error(w, "Cannot save install request", 503)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if len(query) != 0 {
		http.Error(w, "Unknown query", 400)
		return
	}
	a.render(w, 200, view{Session: item})
}

func (a *reader) connect(w http.ResponseWriter, r *http.Request) {
	id, item, ok := a.browserSession(w, r, false)
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if !ok || r.Header.Get("Origin") != a.config.PublicURL || r.ParseForm() != nil || len(r.PostForm["csrf"]) != 1 || subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(item.CSRF)) != 1 {
		http.Error(w, "Invalid browser request", 403)
		return
	}
	if !requestID.MatchString(item.PendingRequest) {
		a.render(w, 400, view{Session: item, Error: "Buka link instalasi development dari Developer Console terlebih dahulu."})
		return
	}
	state, verifier := randomString(), randomString()
	if a.store.change(id, func(s *session) error {
		s.StateHash = digest(state)
		s.Verifier = verifier
		s.StateExpiresAt = time.Now().Add(10 * time.Minute)
		s.Notice = ""
		return nil
	}) != nil {
		http.Error(w, "Cannot save authorization state", 503)
		return
	}
	challenge := sha256.Sum256([]byte(verifier))
	consent, _ := url.Parse(a.config.ConsentURL)
	consent.RawQuery = url.Values{"client_id": {a.config.ClientID}, "redirect_uri": {a.config.PublicURL + "/"}, "state": {state},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(challenge[:])}, "code_challenge_method": {"S256"},
		"scope": {"read_products"}, "test_install_request": {item.PendingRequest}}.Encode()
	http.Redirect(w, r, consent.String(), http.StatusSeeOther)
}

func (a *reader) callback(w http.ResponseWriter, r *http.Request) {
	id, _, ok := a.browserSession(w, r, false)
	query := r.URL.Query()
	valid := ok && len(query["state"]) == 1 && len(query.Get("state")) == 43 && query.Has("code") != query.Has("error")
	for key, values := range query {
		if (key != "state" && key != "code" && key != "error") || len(values) != 1 || len(values[0]) > 512 {
			valid = false
		}
	}
	if !valid {
		http.Error(w, "Invalid OAuth callback", 400)
		return
	}
	var pending session
	err := a.store.change(id, func(s *session) error {
		if s.StateHash == "" || !s.StateExpiresAt.After(time.Now()) || subtle.ConstantTimeCompare([]byte(s.StateHash), []byte(digest(query.Get("state")))) != 1 {
			return errors.New("invalid state")
		}
		pending = *s
		s.StateHash = ""
		s.Verifier = ""
		s.StateExpiresAt = time.Time{}
		s.PendingRequest = ""
		s.Notice = "Instalasi belum selesai. Jika gagal, minta link development baru."
		return nil
	})
	if err != nil {
		http.Error(w, "OAuth state expired, already used, or belongs to another browser", 400)
		return
	}
	if query.Has("error") {
		_ = a.store.change(id, func(s *session) error {
			s.Notice = "Instalasi dibatalkan. Tidak ada token baru yang disimpan."
			return nil
		})
		http.Redirect(w, r, "/", 303)
		return
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {query.Get("code")}, "redirect_uri": {a.config.PublicURL + "/"}, "code_verifier": {pending.Verifier}}
	req, _ := http.NewRequestWithContext(r.Context(), "POST", a.config.GatewayURL+"/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(a.config.ClientID, a.config.ClientSecret)
	status, body, err := a.send(req)
	var token struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int64  `json:"expires_in"`
		Scope        string `json:"scope"`
		Installation struct{ ID, AppID, MerchantID, MerchantName, Status string }
	}
	if err != nil || status != 200 || json.Unmarshal(body, &token) != nil || token.TokenType != "Bearer" || !strings.HasPrefix(token.AccessToken, "es_at_") ||
		token.Scope != "read_products" || token.ExpiresIn < 1 || token.ExpiresIn > 86400 || token.Installation.AppID != a.config.AppID ||
		!opaqueID.MatchString(token.Installation.ID) || !opaqueID.MatchString(token.Installation.MerchantID) || token.Installation.Status != "active" {
		http.Redirect(w, r, "/", 303)
		return
	}
	if a.store.change(id, func(s *session) error {
		s.Token = token.AccessToken
		s.TokenExpiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
		s.InstallationID = token.Installation.ID
		s.MerchantID = token.Installation.MerchantID
		s.MerchantName = token.Installation.MerchantName
		s.Notice = "Instalasi terhubung. Token hanya tersimpan di backend contoh app."
		return nil
	}) != nil {
		http.Error(w, "Cannot store installation token", 503)
		return
	}
	http.Redirect(w, r, "/products", 303)
}

func (a *reader) send(req *http.Request) (int, []byte, error) {
	req.Header.Set("Accept", "application/json")
	res, err := a.client.Do(req)
	if err != nil {
		return 0, nil, errors.New("gateway unavailable")
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4*1024*1024+1))
	if err != nil || len(body) > 4*1024*1024 {
		return 0, nil, errors.New("invalid gateway response")
	}
	return res.StatusCode, body, nil
}

func (a *reader) read(w http.ResponseWriter, r *http.Request) (session, int, []byte) {
	id, item, ok := a.browserSession(w, r, false)
	if !ok || item.Token == "" || !item.TokenExpiresAt.After(time.Now()) {
		return item, 401, nil
	}
	query := r.URL.Query()
	for key, values := range query {
		if key != "cursor" || len(values) != 1 || len(values[0]) == 0 || len(values[0]) > 1024 {
			return item, 400, nil
		}
	}
	path := "/v1/products"
	if productID := r.PathValue("productId"); productID != "" {
		if !opaqueID.MatchString(productID) || len(query) > 0 {
			return item, 400, nil
		}
		path += "/" + productID
	} else {
		query.Set("limit", "10")
		path += "?" + query.Encode()
	}
	req, _ := http.NewRequestWithContext(r.Context(), "GET", a.config.GatewayURL+path, nil)
	req.Header.Set("Authorization", "Bearer "+item.Token)
	status, body, err := a.send(req)
	if err != nil {
		return item, 503, nil
	}
	if status == 401 {
		_ = a.store.change(id, func(s *session) error {
			s.Token = ""
			s.TokenExpiresAt = time.Time{}
			s.Notice = "Akses instalasi berakhir atau dicabut. Mulai instalasi baru untuk menghubungkan kembali."
			return nil
		})
		item.Token = ""
		item.Notice = "" // Do not render a stale "connected" notice on the revocation response.
	}
	return item, status, body
}

func readError(status int) string {
	switch status {
	case 401:
		return "Akses instalasi berakhir atau dicabut. Produk tidak dapat dibaca."
	case 403:
		return "Izin read_products tidak diberikan."
	case 404:
		return "Produk tidak ditemukan untuk instalasi ini."
	case 400:
		return "Parameter pembacaan tidak valid."
	case 429:
		return "Batas pembacaan tercapai. Coba kembali nanti."
	default:
		return "Resource belum tersedia. Hubungi operator pilot; jangan mengganti Merchant ID."
	}
}

func (a *reader) products(w http.ResponseWriter, r *http.Request) {
	item, status, body := a.read(w, r)
	data := view{Session: item}
	var page productPage
	if status == 200 && json.Unmarshal(body, &page) == nil && page.Data != nil {
		data.Products = page.Data
		if page.Meta.NextCursor != nil {
			data.NextCursor = *page.Meta.NextCursor
		}
	} else {
		if status == 200 {
			status = 503
		}
		data.Error = readError(status)
	}
	a.render(w, status, data)
}

func (a *reader) productsJSON(w http.ResponseWriter, r *http.Request) {
	_, status, body := a.read(w, r)
	w.Header().Set("Content-Type", "application/json")
	if status != 200 {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": readError(status)})
		return
	}
	var payload any
	if json.Unmarshal(body, &payload) != nil {
		http.Error(w, "Invalid resource response", 503)
		return
	}
	_, _ = w.Write(body)
}
