package bootstrap_test

import (
	"context"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/identity"
	"emisell.app/platform/internal/identity/merchantlogin"
	identityrepo "emisell.app/platform/internal/identity/postgres"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/review"
	"encoding/json"
	"net/http"
	"reflect"
	"sync"
	"testing"
)

const developerOrigin = "http://localhost:4319"

func portalAccount(t *testing.T, f *fixture, surface, role string) *fixture {
	t.Helper()
	p := identity.PortalPrincipal{ID: ids.New("portal"), Surface: surface, Role: role}
	p.Email = p.ID + "@portal.invalid"
	password := ids.New("password")
	if surface == "developer" {
		repo := merchantlogin.Repository{Pool: f.pool}
		ctx := context.Background()
		request, proof, err := repo.Start(ctx, developerOrigin)
		if err != nil {
			t.Fatal(err)
		}
		_, code, err := repo.Approve(ctx, merchantlogin.Assertion{Request: request, Subject: p.ID, Email: p.Email, Name: "Test Developer", Stores: []merchantlogin.Store{{ID: "store1", CommonID: "test-store", Name: "Test Store"}}})
		if err != nil {
			t.Fatal(err)
		}
		token, err := repo.Finish(ctx, request, proof, code, developerOrigin)
		if err != nil {
			t.Fatal(err)
		}
		user, err := (identity.Portals{Repo: identityrepo.Repository{Pool: f.pool}}).Authenticate(ctx, "developer", token)
		if err != nil {
			t.Fatal(err)
		}
		client := *f
		client.user = user.ID
		client.email = p.Email
		client.password = password
		client.cookie = &http.Cookie{Name: "emisell_developer_session", Value: token, Path: "/api/v1/developer", HttpOnly: true, SameSite: http.SameSiteStrictMode}
		return &client
	}
	if err := (identityrepo.Repository{Pool: f.pool}).SeedPortal(context.Background(), p, password); err != nil {
		t.Fatal(err)
	}
	client := *f
	client.cookie = nil
	client.user = p.ID
	client.email = p.Email
	client.password = password
	source := origin
	if surface == "developer" {
		source = developerOrigin
	}
	status, _, cookies, err := client.call("POST", "/api/v1/"+surface+"/login", map[string]string{"email": p.Email, "password": password}, "", source)
	if err != nil || status != 200 || len(cookies) != 1 {
		t.Fatalf("portal login %d %v", status, err)
	}
	client.cookie = cookies[0]
	if !client.cookie.HttpOnly || client.cookie.SameSite != http.SameSiteStrictMode || client.cookie.Path != "/api/v1/"+surface {
		t.Fatal("cookie isolation missing")
	}
	return &client
}
func pexpect(t *testing.T, f *fixture, method, path string, body any, k string, want int) map[string]any {
	t.Helper()
	source := origin
	if f.cookie != nil && f.cookie.Name == "emisell_developer_session" {
		source = developerOrigin
	}
	status, data, _, err := f.call(method, path, body, k, source)
	if err != nil || status != want {
		t.Fatalf("%s %s got %d want %d: %v %v", method, path, status, want, data, err)
	}
	return data
}
func portalDocument() service.AppDocument {
	return service.AppDocument{Name: "Portal shipping test", Summary: "Test shipping app", Description: "A reference submission, not an executable release.", Version: "1.0.0", Capability: "shipping/v1", Scopes: shippingScopes, Endpoint: "https://example.invalid/emisell/v1"}
}
func TestPortalDraftReviewIsolationAndPersistence(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "reviewer")
	operator := portalAccount(t, f, "admin", "operator")
	// Legacy sessions cannot become portal sessions, even when cookies are renamed.
	pexpect(t, f, "GET", "/api/v1/admin/session", nil, "", 401)
	owner := *f
	c := *f.cookie
	c.Name = "emisell_admin_session"
	owner.cookie = &c
	pexpect(t, &owner, "GET", "/api/v1/admin/session", nil, "", 401)
	pexpect(t, admin, "GET", "/api/v1/session", nil, "", 401)
	for _, method := range []string{"GET", "POST"} {
		status, _, _, _ := admin.call(method, "/api/v1/admin/session", nil, "", developerOrigin)
		if status != 403 {
			t.Fatal("cross-origin admin access", status)
		}
	}
	wrong := *dev
	cross := *dev.cookie
	cross.Name = "emisell_admin_session"
	wrong.cookie = &cross
	pexpect(t, &wrong, "GET", "/api/v1/admin/session", nil, "", 401)
	// Portal credentials are not accepted by the merchant login endpoint.
	pexpect(t, admin, "POST", "/api/v1/login", map[string]string{"email": admin.email, "password": admin.password}, "", 401)
	body := service.SaveDraft{Document: portalDocument()}
	k := key()
	created := pexpect(t, dev, "POST", "/api/v1/developer/apps", body, k, 200)
	if !reflect.DeepEqual(created, pexpect(t, dev, "POST", "/api/v1/developer/apps", body, k, 200)) {
		t.Fatal("draft replay changed")
	}
	app := created["app"].(map[string]any)
	id := app["id"].(string)
	path := "/api/v1/developer/apps/" + id
	changed := body
	changed.Document.Name = "Changed name"
	pexpect(t, dev, "POST", "/api/v1/developer/apps", changed, k, 409)
	pexpect(t, dev, "POST", "/api/v1/developer/apps", body, "", 400)
	pexpect(t, other, "GET", path, nil, "", 404)
	pexpect(t, other, "PUT", path, service.SaveDraft{Revision: 1, Document: body.Document}, key(), 404)
	pexpect(t, other, "POST", path+"/submissions", map[string]int{"revision": 1}, key(), 404)
	submitKey := key()
	submitted := pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 1}, submitKey, 200)
	sub := submitted["submission"].(map[string]any)
	subID := sub["id"].(string)
	if !reflect.DeepEqual(submitted, pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 1}, submitKey, 200)) {
		t.Fatal("submit replay")
	}
	pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 1}, key(), 409)
	pexpect(t, other, "GET", "/api/v1/developer/submissions/"+subID, nil, "", 404)
	otherList := pexpect(t, other, "GET", "/api/v1/developer/submissions", nil, "", 200)["submissions"].([]any)
	if len(otherList) != 0 {
		t.Fatal("cross developer list leak")
	}
	// Saving a new draft does not mutate the frozen review snapshot.
	body.Revision = 1
	body.Document.Summary = "Revised draft summary"
	pexpect(t, dev, "PUT", path, body, key(), 200)
	pexpect(t, dev, "PUT", path, body, key(), 409)
	detail := pexpect(t, admin, "GET", "/api/v1/admin/submissions/"+subID, nil, "", 200)
	if detail["submission"].(map[string]any)["snapshot"].(map[string]any)["summary"] == body.Document.Summary {
		t.Fatal("snapshot mutated")
	}
	if !reflect.DeepEqual(submitted, pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 1}, submitKey, 200)) {
		t.Fatal("retry after draft edit")
	}
	decision := review.Decision{Status: "changes_requested", Feedback: "Lengkapi dokumentasi integrasi."}
	decisionPath := "/api/v1/admin/submissions/" + subID + "/decision"
	pexpect(t, operator, "POST", decisionPath, decision, key(), 403)
	pexpect(t, admin, "POST", decisionPath, review.Decision{Status: "approved"}, key(), 400)
	decisionKey := key()
	decided := pexpect(t, admin, "POST", decisionPath, decision, decisionKey, 200)
	if !reflect.DeepEqual(decided, pexpect(t, admin, "POST", decisionPath, decision, decisionKey, 200)) {
		t.Fatal("decision replay")
	}
	pexpect(t, admin, "POST", decisionPath, review.Decision{Status: "approved", Feedback: "OK"}, decisionKey, 409)
	pexpect(t, admin, "POST", decisionPath, decision, key(), 409)
	detail = pexpect(t, dev, "GET", "/api/v1/developer/submissions/"+subID, nil, "", 200)
	if len(detail["history"].([]any)) != 2 {
		t.Fatal("audit duplicate or missing")
	}
	if detail["submission"].(map[string]any)["feedback"] != decision.Feedback {
		t.Fatal("feedback missing")
	}
	resubmit := pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 2}, key(), 200)["submission"].(map[string]any)
	newID := resubmit["id"].(string)
	pexpect(t, admin, "POST", "/api/v1/admin/submissions/"+newID+"/decision", review.Decision{Status: "approved", Feedback: "Metadata diperiksa; belum dipublikasikan."}, key(), 200)
	body.Revision = 2
	pexpect(t, dev, "PUT", path, body, key(), 200)
	pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 3}, key(), 409)
	body.Revision = 3
	body.Document.Version = "1.0.1"
	pexpect(t, dev, "PUT", path, body, key(), 200)
	pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 4}, key(), 200)
	// No draft/review write is allowed to create an executable/public registry release.
	var count int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM platform_app.releases WHERE app_id=$1`, id).Scan(&count); err != nil || count != 0 {
		t.Fatal("review leaked into registry", count, err)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE platform_review.submissions SET snapshot='{}' WHERE id=$1`, subID); err == nil {
		t.Fatal("database allowed immutable snapshot mutation")
	}
	// Password login is retired; logout still invalidates the merchant-backed session.
	previous := *dev.cookie
	status, _, _, err := dev.call("POST", "/api/v1/developer/login", map[string]string{"email": dev.email, "password": dev.password}, "", developerOrigin)
	if status != 404 || err != nil {
		t.Fatal("legacy login enabled", status, err)
	}
	pexpect(t, dev, "GET", path, nil, "", 200)
	stale := *dev
	stale.cookie = &previous
	pexpect(t, dev, "POST", "/api/v1/developer/logout", map[string]any{}, "", 200)
	pexpect(t, &stale, "GET", "/api/v1/developer/session", nil, "", 401)
	pexpect(t, dev, "GET", "/api/v1/developer/session", nil, "", 401)
	pexpect(t, admin, "GET", "/api/v1/admin/session", nil, "", 200)
	// Disabled roles and revoked developer membership take effect on existing sessions.
	if _, err = f.pool.Exec(context.Background(), `UPDATE platform_identity.portal_accounts SET enabled=false WHERE id=$1`, admin.user); err != nil {
		t.Fatal(err)
	}
	pexpect(t, admin, "GET", "/api/v1/admin/session", nil, "", 401)
	if _, err = f.pool.Exec(context.Background(), `DELETE FROM platform_developer.memberships WHERE account_id=$1`, other.user); err != nil {
		t.Fatal(err)
	}
	pexpect(t, other, "GET", "/api/v1/developer/apps", nil, "", 403)
}
func TestPortalConcurrentDecisionAndSubmit(t *testing.T) {
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "reviewer")
	second := portalAccount(t, f, "admin", "administrator")
	app := pexpect(t, dev, "POST", "/api/v1/developer/apps", service.SaveDraft{Document: portalDocument()}, key(), 200)["app"].(map[string]any)
	path := "/api/v1/developer/apps/" + app["id"].(string) + "/submissions"
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	submitKey := key()
	for range 2 {
		wg.Go(func() {
			status, _, _, _ := dev.call("POST", path, map[string]int{"revision": 1}, submitKey, developerOrigin)
			codes <- status
		})
	}
	wg.Wait()
	close(codes)
	for code := range codes {
		if code != 200 {
			t.Fatal("concurrent replay", code)
		}
	}
	submissions := pexpect(t, dev, "GET", "/api/v1/developer/submissions", nil, "", 200)["submissions"].([]any)
	if len(submissions) != 1 {
		t.Fatal("duplicate submission")
	}
	id := submissions[0].(map[string]any)["id"].(string)
	codes = make(chan int, 2)
	for _, actor := range []*fixture{admin, second} {
		wg.Go(func() {
			status, _, _, _ := actor.call("POST", "/api/v1/admin/submissions/"+id+"/decision", review.Decision{Status: "approved", Feedback: "Reviewed"}, key(), origin)
			codes <- status
		})
	}
	wg.Wait()
	close(codes)
	successes, conflicts := 0, 0
	for code := range codes {
		if code == 200 {
			successes++
		}
		if code == 409 {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("concurrent decision", successes, conflicts)
	}
	detail := pexpect(t, dev, "GET", "/api/v1/developer/submissions/"+id, nil, "", 200)
	if len(detail["history"].([]any)) != 2 {
		raw, _ := json.Marshal(detail)
		t.Fatalf("audit %s", raw)
	}
}
