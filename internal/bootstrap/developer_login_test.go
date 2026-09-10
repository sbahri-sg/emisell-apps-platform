package bootstrap_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/identity/merchantlogin"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
)

func TestDeveloperMerchantLoginRepository(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	repo := merchantlogin.Repository{Pool: f.pool}
	subject := ids.New("coreuser")
	a := merchantlogin.Assertion{Subject: subject, Email: subject + "@example.com", Name: "Test Merchant", Stores: []merchantlogin.Store{{ID: "store1", CommonID: "test-store", Name: "Test Store"}}}
	start := func() (string, string, string) {
		request, proof, err := repo.Start(ctx, developerOrigin)
		if err != nil {
			t.Fatal(err)
		}
		a.Request = request
		returned, code, err := repo.Approve(ctx, a)
		if err != nil || returned != developerOrigin {
			t.Fatal("approval failed", err)
		}
		if _, _, err = repo.Approve(ctx, a); !errors.Is(err, fault.Conflict) {
			t.Fatal("approval replay allowed")
		}
		return request, proof, code
	}
	request, proof, code := start()
	if _, err := repo.Finish(ctx, request, merchantlogin.Token(), code, developerOrigin); !errors.Is(err, fault.Unauthenticated) {
		t.Fatal("foreign browser accepted")
	}
	if _, err := repo.Finish(ctx, request, proof, code, origin); !errors.Is(err, fault.Unauthenticated) {
		t.Fatal("foreign portal accepted")
	}
	token, err := repo.Finish(ctx, request, proof, code, developerOrigin)
	if err != nil {
		t.Fatal(err)
	}
	portals := identity.Portals{Repo: identityrepo.Repository{Pool: f.pool}}
	user, err := portals.Authenticate(ctx, "developer", token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = portals.Authenticate(ctx, "admin", token); err == nil {
		t.Fatal("developer became admin")
	}
	if _, err = repo.Finish(ctx, request, proof, code, developerOrigin); !errors.Is(err, fault.Unauthenticated) {
		t.Fatal("code replay allowed")
	}
	profile, err := repo.Profile(ctx, user)
	if err != nil || profile.Email != strings.ToLower(a.Email) || len(profile.Stores) != 1 {
		t.Fatal("profile missing")
	}
	request, proof, code = start()
	again, err := repo.Finish(ctx, request, proof, code, developerOrigin)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := portals.Authenticate(ctx, "developer", again)
	if err != nil || repeated.ID != user.ID {
		t.Fatal("repeat login duplicated account")
	}
	old := portalAccount(t, f, "developer", "developer")
	a.Subject = ids.New("anothercore")
	a.Email = old.email
	request, proof, code = start()
	linked, err := repo.Finish(ctx, request, proof, code, developerOrigin)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := portals.Authenticate(ctx, "developer", linked)
	if err != nil || owner.ID == old.user {
		t.Fatal("email must not transfer ownership")
	}
	request, proof, code = start()
	if _, err = f.pool.Exec(ctx, `UPDATE platform_identity.developer_login_requests SET expires_at=now()-interval '1 second' WHERE id=$1`, request); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Finish(ctx, request, proof, code, developerOrigin); !errors.Is(err, fault.Unauthenticated) {
		t.Fatal("expired code accepted")
	}
}

func TestDeveloperMerchantLoginHTTP(t *testing.T) {
	for _, portalOrigin := range []string{developerOrigin, origin} {
		t.Run(portalOrigin, func(t *testing.T) { testDeveloperMerchantLoginHTTP(t, portalOrigin) })
	}
}

func testDeveloperMerchantLoginHTTP(t *testing.T, portalOrigin string) {
	f := setup(t)
	ctx := context.Background()
	dev := portalAccount(t, f, "developer", "developer")
	created := pexpect(t, dev, "POST", "/api/v1/developer/apps", map[string]any{"revision": 0, "document": portalDocument()}, "developer-login-existing-app", 200)
	appID := created["app"].(map[string]any)["id"].(string)
	keys := identity.PlatformKeys{Repo: identityrepo.Repository{Pool: f.pool}}
	adminFixture := portalAccount(t, f, "admin", "administrator")
	admin := identity.PortalPrincipal{ID: adminFixture.user, Surface: "admin", Role: "administrator"}
	key, secret, err := keys.Create(ctx, admin, ids.New("request"), identity.CreatePlatformKey{Name: "Login test"})
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{CheckRedirect: func(r *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	call := func(method, path string, body any, source, bearer string, cookie *http.Cookie) *http.Response {
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, f.server.URL+path, bytes.NewReader(raw))
		req.Host = strings.TrimPrefix(portalOrigin, "http://")
		req.Header.Set("Content-Type", "application/json")
		if source != "" {
			req.Header.Set("Origin", source)
		}
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { res.Body.Close() })
		return res
	}
	start := call("POST", "/api/v1/developer-login/start", map[string]any{}, portalOrigin, "", nil)
	if start.StatusCode != 200 {
		t.Fatal("authenticated start", start.StatusCode)
	}
	var data map[string]string
	if err = json.NewDecoder(start.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	target, _ := url.Parse(data["authorizeUrl"])
	if target.Path != "/auth/login" {
		t.Fatal("must use the existing merchant login", target.Path)
	}
	resume, _ := url.Parse(target.Query().Get("returnTo"))
	if resume.Path != "/api/app-platform/sso" || resume.IsAbs() {
		t.Fatal("invalid SSO callback", resume)
	}
	request := resume.Query().Get("request")
	cookies := start.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatal("browser proof cookie missing")
	}
	payload := merchantlogin.Assertion{Request: request, Subject: ids.New("core"), Email: "linked-" + dev.email, Name: "Owner", Stores: []merchantlogin.Store{{ID: "one", CommonID: "test-one", Name: "Test"}}}
	if res := call("POST", "/api/v1/developer-login/approve", payload, "", "", nil); res.StatusCode != 401 {
		t.Fatal("anonymous approval", res.StatusCode)
	}
	if res := call("POST", "/api/v1/developer-login/approve", payload, developerOrigin, secret, nil); res.StatusCode != 403 {
		t.Fatal("browser assertion accepted", res.StatusCode)
	}
	approved := call("POST", "/api/v1/developer-login/approve", payload, "", secret, nil)
	if approved.StatusCode != 200 {
		t.Fatal("Core assertion", approved.StatusCode)
	}
	if err = json.NewDecoder(approved.Body).Decode(&data); err != nil {
		t.Fatal(err)
	}
	exchange, _ := url.Parse(data["exchangeUrl"])
	if res := call("GET", exchange.RequestURI(), nil, "", "", nil); res.StatusCode != 401 {
		t.Fatal("callback without browser proof", res.StatusCode)
	}
	finished := call("GET", exchange.RequestURI(), nil, "", "", cookies[0])
	if finished.StatusCode != 303 || finished.Header.Get("Location") != portalOrigin+"/development" {
		t.Fatal("callback failed", finished.StatusCode)
	}
	for _, c := range finished.Cookies() {
		if c.Name == "emisell_developer_session" || c.Name == "emisell_portal_session" {
			dev.cookie = c
		}
	}
	pexpect(t, dev, "GET", "/api/v1/developer/apps/"+appID, nil, "", 404)
	account := pexpect(t, dev, "GET", "/api/v1/developer/account", nil, "", 200)
	if account["profile"].(map[string]any)["email"] != strings.ToLower(payload.Email) {
		t.Fatal("Core profile missing")
	}
	pexpect(t, dev, "GET", "/api/v1/developer/activity", nil, "", 200)
	status, _, renewed, err := dev.call("POST", "/api/v1/developer/activity", map[string]any{}, "", portalOrigin)
	if err != nil || status != 200 || len(renewed) != 1 || renewed[0].MaxAge != 3600 {
		t.Fatal("activity cookie lifetime", status, err)
	}
	replay := call("GET", exchange.RequestURI(), nil, "", "", cookies[0])
	if replay.Header.Get("Location") != portalOrigin+"/development?login=failed" {
		t.Fatal("HTTP callback replay")
	}
	if _, err = keys.Revoke(ctx, admin, key.ID); err != nil {
		t.Fatal(err)
	}
	revoked := call("POST", "/api/v1/developer-login/approve", payload, "", secret, nil)
	if revoked.StatusCode != 401 {
		t.Fatal("revoked Core key accepted")
	}
	body, _ := io.ReadAll(revoked.Body)
	if strings.Contains(string(body), secret) {
		t.Fatal("secret in error")
	}
}

func TestDeveloperMerchantLoginIdleExpiry(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	ctx := context.Background()
	repo := merchantlogin.Repository{Pool: f.pool}
	first, err := repo.ActivityExpiry(ctx, dev.cookie.Value)
	if err != nil {
		t.Fatal(err)
	}
	pexpect(t, dev, "GET", "/api/v1/developer/session", nil, "", 200)
	second, err := repo.ActivityExpiry(ctx, dev.cookie.Value)
	if err != nil || !first.Equal(second) {
		t.Fatal("passive read extended session", err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE platform_identity.portal_sessions SET last_active_at=now()-interval '59 minutes',expires_at=now()+interval '1 minute' WHERE token_hash=$1`, merchantlogin.Hash(dev.cookie.Value)); err != nil {
		t.Fatal(err)
	}
	pexpect(t, dev, "POST", "/api/v1/developer/activity", map[string]any{}, "", 200)
	if _, err = f.pool.Exec(ctx, `UPDATE platform_identity.portal_sessions SET last_active_at=now()-interval '1 hour' WHERE token_hash=$1`, merchantlogin.Hash(dev.cookie.Value)); err != nil {
		t.Fatal(err)
	}
	pexpect(t, dev, "GET", "/api/v1/developer/session", nil, "", 401)
	pexpect(t, dev, "POST", "/api/v1/developer/activity", map[string]any{}, "", 401)
	if _, err = repo.Activity(ctx, dev.cookie.Value); !errors.Is(err, fault.Unauthenticated) {
		t.Fatal("expired session revived", err)
	}
}
