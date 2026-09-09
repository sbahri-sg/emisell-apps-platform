package bootstrap_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	apprepo "emisell.app/platform/internal/app/repository"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
	"github.com/jackc/pgx/v5"
)

func TestApplicationCredentialLifecycle(t *testing.T) {
	t.Setenv("EMISELL_APP_CREDENTIAL_KEY", base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{42}, 32)))
	f := setup(t)
	dev := portalAccount(t, f, "developer", "developer")
	other := portalAccount(t, f, "developer", "developer")
	admin := portalAccount(t, f, "admin", "administrator")
	body := map[string]any{"revision": 0, "document": portalDocument()}
	created := pexpect(t, dev, "POST", "/api/v1/developer/apps", body, "identity-create-1", 200)
	app := created["app"].(map[string]any)["id"].(string)
	path := "/api/v1/developer/apps/" + app + "/credentials"
	get := func() map[string]any {
		return pexpect(t, dev, "GET", path, nil, "", 200)["credential"].(map[string]any)
	}
	c := get()
	id := c["clientId"].(string)
	if !strings.HasPrefix(id, "eai_") || c["version"] != float64(1) {
		t.Fatal("identity not created with app")
	}
	if _, ok := c["secret"]; ok {
		t.Fatal("secret exposed by GET")
	}
	if _, ok := c["Ciphertext"]; ok {
		t.Fatal("ciphertext exposed")
	}
	replay := pexpect(t, dev, "POST", "/api/v1/developer/apps", body, "identity-create-1", 200)
	if replay["app"].(map[string]any)["id"] != app || get()["clientId"] != id {
		t.Fatal("create replay changed identity")
	}
	reveal := pexpect(t, dev, "POST", path+"/reveal", map[string]int{"version": 1}, "", 200)
	secret := reveal["secret"].(string)
	if !strings.HasPrefix(secret, "ecs_") {
		t.Fatal("secret missing")
	}
	var cipher []byte
	var hash string
	if err := f.pool.QueryRow(context.Background(), `SELECT secret_ciphertext,secret_hash FROM platform_oauth.application_credentials WHERE client_id=$1`, id).Scan(&cipher, &hash); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(cipher, []byte(secret)) || hash == secret {
		t.Fatal("secret stored in plaintext")
	}
	for _, method := range []string{"GET", "POST"} {
		p := path
		if method == "POST" {
			p += "/reveal"
		}
		pexpect(t, other, method, p, map[string]int{"version": 1}, "", 404)
	}
	pexpect(t, other, "POST", path+"/rotate", map[string]int{"version": 1}, "other-rotate-key", 404)
	adminStatus, _, _, _ := admin.call("POST", strings.Replace(path, "/developer/", "/admin/", 1)+"/reveal", map[string]int{"version": 1}, "", origin)
	if adminStatus != 404 {
		t.Fatal("admin reveal route must not exist")
	}
	status, _, _, err := dev.call("POST", path+"/reveal", map[string]int{"version": 1}, "", "https://untrusted.invalid")
	if err != nil || status != 403 {
		t.Fatal("origin boundary missing")
	}
	authenticate := func(s string, want int) {
		r, _ := http.NewRequest("POST", f.server.URL+"/api/v1/app/client-check", strings.NewReader("{}"))
		r.Header.Set("Content-Type", "application/json")
		r.SetBasicAuth(id, s)
		response, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("client check status %d want %d", response.StatusCode, want)
		}
		if want == 200 {
			var v map[string]any
			if err = json.NewDecoder(response.Body).Decode(&v); err != nil {
				t.Fatal(err)
			}
			if v["oauthEnabled"] != false || v["resourceGatewayAllowed"] != false || v["installable"] != false {
				t.Fatal("identity bypassed grant gate")
			}
		}
	}
	authenticate(secret, 200)
	// Editing a draft never reissues its identity or secret.
	doc := portalDocument()
	doc.Name = "Updated application"
	doc.Version = "2.0.0"
	pexpect(t, dev, "PUT", "/api/v1/developer/apps/"+app, map[string]any{"revision": 1, "document": doc}, "identity-update-1", 200)
	if get()["clientId"] != id || get()["version"] != float64(1) {
		t.Fatal("draft update changed credentials")
	}
	var wg sync.WaitGroup
	results := make(chan int, 2)
	for _, key := range []string{"rotate-concurrent-a", "rotate-concurrent-b"} {
		wg.Add(1)
		go func(k string) {
			defer wg.Done()
			s, _, _, e := dev.call("POST", path+"/rotate", map[string]int{"version": 1}, k, developerOrigin)
			if e != nil {
				results <- 0
			} else {
				results <- s
			}
		}(key)
	}
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for s := range results {
		if s == 200 {
			wins++
		}
		if s == 409 {
			conflicts++
		}
	}
	if wins != 1 || conflicts != 1 {
		t.Fatal("concurrent rotation not serialized")
	}
	authenticate(secret, 401)
	newSecret := pexpect(t, dev, "POST", path+"/reveal", map[string]int{"version": 2}, "", 200)["secret"].(string)
	if secret == newSecret {
		t.Fatal("rotation did not change secret")
	}
	authenticate(newSecret, 200)
	pexpect(t, dev, "POST", path+"/rotate", map[string]int{"version": 2}, "repeat-rotation-key", 200)
	pexpect(t, dev, "POST", path+"/rotate", map[string]int{"version": 2}, "repeat-rotation-key", 200)
	if get()["version"] != float64(3) || get()["clientId"] != id {
		t.Fatal("replay changed stable identity")
	}
	pexpect(t, dev, "POST", path+"/reveal", map[string]int{"version": 2}, "", 409)
	var count int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM platform_oauth.application_credential_audit WHERE client_id=$1 AND action='rotated'`, id).Scan(&count)
	if count != 2 {
		t.Fatal("rotation audit duplicated")
	}
	// Secret responses must not be cached.
	r, _ := http.NewRequest("POST", f.server.URL+path+"/reveal", strings.NewReader(`{"version":3}`))
	r.AddCookie(dev.cookie)
	r.Header.Set("Origin", developerOrigin)
	r.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatal("secret response is cacheable")
	}
}

func TestApplicationCredentialCreationRollback(t *testing.T) {
	f := setup(t)
	repo := apprepo.Postgres{Pool: f.pool, OnDraftCreated: func(context.Context, pgx.Tx, string, string, string) error { return fault.Unavailable }}
	org := "rollback-" + f.user
	_, err := repo.SaveDraft(context.Background(), org, f.user, "", "rollback-create-key", "request-hash", service.SaveDraft{Document: portalDocument()})
	if err != fault.Unavailable {
		t.Fatal("expected credential failure")
	}
	var count int
	if err = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM platform_app.drafts WHERE organization_id=$1`, org).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed credential creation left an app")
	}
}
