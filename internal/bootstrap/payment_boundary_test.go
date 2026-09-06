package bootstrap_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	apprepo "emisell.app/platform/internal/app/repository"
	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/bootstrap"
	"emisell.app/platform/internal/oauth/appclient"
	"emisell.app/platform/internal/platform/ids"
	"emisell.app/platform/internal/review"
	"emisell.app/platform/pkg/catalogmanifest"
	"emisell.app/platform/pkg/integrationmanifest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Seed historical records directly ONLY in setup's disposable test database.
// Production APIs must not offer a bypass to recreate these reserved releases.
func TestInternalPaymentDistributionBoundary(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, catalogKey, _ := ed25519.GenerateKey(rand.Reader)
	_, integrationKey, _ := ed25519.GenerateKey(rand.Reader)
	cs, is := review.CatalogSigner{Key: catalogKey}, review.IntegrationSigner{Key: integrationKey}
	clientPool, err := pgxpool.NewWithConfig(ctx, f.pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(clientPool.Close)
	f.server.Close()
	f.server = httptest.NewServer(bootstrap.HandlerWithAppClients(f.pool, f.caps, clientPool, origin, slog.New(slog.NewTextHandler(io.Discard, nil)), cs, is, &proofVerifier{}))
	t.Cleanup(f.server.Close)
	dev, admin, other := portalAccount(t, f, "developer", "developer"), portalAccount(t, f, "admin", "administrator"), portalAccount(t, f, "developer", "developer")
	org := pexpect(t, dev, "GET", "/api/v1/developer/session", nil, "", 200)["organization"].(map[string]any)["id"].(string)
	doc := portalDocument()
	doc.Name, doc.Capability, doc.Scopes, doc.Endpoint = "Legacy "+key(), "payment/v1", payScopes, "https://app.example.com/api"
	pexpect(t, dev, "POST", "/api/v1/developer/apps", service.SaveDraft{Document: doc}, key(), 403)
	appID, subID, catalogID := ids.New("app"), ids.New("sub"), ids.New("cat")
	raw := func(v any) []byte {
		b, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO platform_app.drafts(id,organization_id,revision,document) VALUES($1,$2,1,$3)`, appID, org, raw(doc))
	seedSubmission := func(id, app, status string) {
		exec(`INSERT INTO platform_review.submissions(id,app_id,organization_id,submitter_id,draft_revision,version,snapshot,status) VALUES($1,$2,$3,$4,1,'1.0.0',$5,$6)`, id, app, org, dev.user, raw(doc), status)
	}
	seedSubmission(subID, appID, "approved")
	path := "/api/v1/developer/apps/" + appID
	pexpect(t, dev, "GET", path, nil, "", 200)
	pexpect(t, other, "GET", path, nil, "", 404)
	pexpect(t, dev, "PUT", path, service.SaveDraft{Revision: 1, Document: doc}, key(), 403)
	pexpect(t, dev, "PUT", path, service.SaveDraft{Revision: 1, Document: portalDocument()}, key(), 403)
	pexpect(t, dev, "POST", path+"/submissions", map[string]int{"revision": 1}, key(), 403)
	if pexpect(t, dev, "GET", path+"/tooling", nil, "", 200)["valid"] != false {
		t.Fatal("legacy tooling claims publishable")
	}
	pendingSub := ids.New("sub")
	seedSubmission(pendingSub, ids.New("app"), "submitted")
	pexpect(t, admin, "POST", "/api/v1/admin/submissions/"+pendingSub+"/decision", review.Decision{Status: "approved", Feedback: "Cannot approve payment"}, key(), 403)
	pexpect(t, admin, "POST", "/api/v1/admin/submissions/"+pendingSub+"/decision", review.Decision{Status: "rejected", Feedback: "Payment is internal"}, key(), 200)
	pexpect(t, admin, "POST", "/api/v1/admin/submissions/"+subID+"/catalog", map[string]any{}, key(), 403)
	metadata := catalogmanifest.Manifest{Schema: catalogmanifest.Schema, Policy: catalogmanifest.Policy, AppID: appID, DeveloperID: org, Version: doc.Version, Name: doc.Name, Summary: doc.Summary, Description: doc.Description, Capability: doc.Capability, Scopes: doc.Scopes, Runtime: "remote", Pricing: "free", SourceSHA256: strings.Repeat("a", 64)}
	pack, err := cs.Sign(metadata)
	if err != nil || cs.Verify(pack) != nil {
		t.Fatal("legacy catalog cannot verify", err)
	}
	exec(`INSERT INTO platform_app.catalog_releases(id,app_id,organization_id,submission_id,version,package,status) VALUES($1,$2,$3,$4,'1.0.0',$5,'published')`, catalogID, appID, org, subID, raw(pack))
	pexpect(t, dev, "GET", "/api/v1/developer/catalog/"+catalogID, nil, "", 200)
	pexpect(t, admin, "POST", "/api/v1/admin/catalog/"+catalogID+"/status", service.CatalogAction{Status: "published", Revision: 1, Reason: "Cannot republish"}, key(), 403)
	pexpect(t, admin, "GET", "/api/v1/store/apps/"+catalogID, nil, "", 404)
	for _, suffix := range []string{"", "&capability=payment%2Fv1"} {
		v := pexpect(t, admin, "GET", "/api/v1/store/apps?search="+url.QueryEscape(doc.Name)+suffix, nil, "", 200)
		if v["total"] != float64(0) || len(v["apps"].([]any)) != 0 {
			t.Fatal("payment in public count/page", v)
		}
	}
	pexpect(t, admin, "POST", "/api/v1/admin/catalog/"+catalogID+"/status", service.CatalogAction{Status: "suspended", Revision: 1, Reason: "Archive historical payment"}, key(), 200)
	m := integrationmanifest.Manifest{Schema: integrationmanifest.Schema, Policy: integrationmanifest.Policy, SubmissionID: subID, Metadata: metadata, Config: integrationmanifest.Config{Protocol: integrationmanifest.Protocol, Endpoint: doc.Endpoint, CallbackURL: "https://app.example.com/oauth", HealthURL: "https://app.example.com/health"}}
	input := service.IntegrationInput{SubmissionID: subID, Config: m.Config}
	if pexpect(t, dev, "POST", "/api/v1/developer/integration-releases/validate", input, "", 200)["validation"].(map[string]any)["valid"] != false {
		t.Fatal("payment config accepted")
	}
	pexpect(t, dev, "POST", "/api/v1/developer/integration-releases", input, key(), 400)
	seedRelease := func(status string) string {
		v := m
		v.SubmissionID = ids.New("sub")
		v.Metadata.AppID = ids.New("app")
		p, e := is.Sign(v)
		if e != nil || is.Verify(p) != nil {
			t.Fatal("legacy signature", e)
		}
		id := ids.New("intrel")
		var stored any
		if status == "signed" {
			stored = raw(p)
		}
		exec(`INSERT INTO platform_app.integration_releases(id,organization_id,app_id,submission_id,version,manifest,sha256,package,status) VALUES($1,$2,$3,$4,'1.0.0',$5,$6,$7,$8)`, id, org, v.Metadata.AppID, v.SubmissionID, raw(v), p.SHA256, stored, status)
		return id
	}
	releaseID := seedRelease("signed")
	report := pexpect(t, dev, "GET", "/api/v1/developer/integration-releases/"+releaseID, nil, "", 200)["validation"].(map[string]any)
	if report["valid"] != false {
		t.Fatal("legacy payment ready")
	}
	for _, c := range report["checks"].([]any) {
		v := c.(map[string]any)
		if v["code"] == "signature" && v["passed"] != true {
			t.Fatal("signature incorrectly invalidated")
		}
	}
	for _, status := range []string{"submitted", "approved"} {
		id := seedRelease(status)
		action := "approved"
		cleanup := "rejected"
		if status == "approved" {
			action = "signed"
			cleanup = "suspended"
		}
		p := "/api/v1/admin/integration-releases/" + id + "/status"
		pexpect(t, admin, "POST", p, service.IntegrationAction{Status: action, Revision: 1, Reason: "Cannot progress payment"}, key(), 400)
		pexpect(t, admin, "POST", p, service.IntegrationAction{Status: cleanup, Revision: 1, Reason: "Archive payment"}, key(), 200)
	}
	pexpect(t, dev, "POST", "/api/v1/developer/app-clients", map[string]string{"releaseId": releaseID}, key(), 409)
	// A historically verified payment client remains readable, but cannot renew,
	// issue secrets or authenticate. Only explicit revoke clears its stored hash.
	legacyRelease, err := (apprepo.Postgres{Pool: f.pool}).IntegrationGet(ctx, org, releaseID)
	if err != nil {
		t.Fatal(err)
	}
	meta := legacyRelease.Manifest.Metadata
	clientID, secret := ids.New("eac"), "eacs_"+strings.Repeat("a", 43)
	secretHash := fmt.Sprintf("%x", sha256.Sum256([]byte(secret)))
	binding := appclient.Binding{ReleaseID: releaseID, OrganizationID: org, AppID: meta.AppID, Version: meta.Version, Name: meta.Name, Digest: legacyRelease.SHA256, Endpoint: m.Config.Endpoint, RedirectURI: m.Config.CallbackURL}
	exec(`INSERT INTO platform_oauth.app_clients(id,organization_id,release_id,binding,status,revision,challenge_id,challenge,challenge_expires_at,verified_until,last_result,secret_hash,secret_version) VALUES($1,$2,$3,$4,'verified',1,$5,$6,now()+interval '10 minutes',now()+interval '1 hour','verified',$7,1)`, clientID, org, releaseID, raw(binding), ids.New("proof"), strings.Repeat("b", 43), secretHash)
	clientPath := "/api/v1/developer/app-clients/" + clientID
	if clientView(t, pexpect(t, dev, "GET", clientPath, nil, "", 200)).Ready {
		t.Fatal("payment client ready")
	}
	pexpect(t, other, "GET", clientPath, nil, "", 404)
	for _, action := range []string{"challenge", "verify", "rotate_secret"} {
		pexpect(t, dev, "POST", clientPath+"/actions", appclient.Action{Action: action, Revision: 1, Reason: "Payment is reserved"}, key(), 409)
	}
	clientCheck(t, f, clientID, secret, 401)
	var saved string
	if err = f.pool.QueryRow(ctx, `SELECT secret_hash FROM platform_oauth.app_clients WHERE id=$1`, clientID).Scan(&saved); err != nil || saved != secretHash {
		t.Fatal("policy mutated credential", err)
	}
	pexpect(t, dev, "POST", clientPath+"/actions", appclient.Action{Action: "revoke", Revision: 1, Reason: "Explicit historical client revocation"}, key(), 200)
	pexpect(t, dev, "POST", "/api/v1/developer/test-assignments", service.AssignmentInput{ReleaseID: releaseID, MerchantID: f.tenant, Reason: "Cannot distribute payment"}, key(), 409)
	seedAssignment := func(status string) string {
		id := ids.New("testasgn")
		exec(`INSERT INTO platform_app.test_assignments(id,organization_id,release_id,release_sha256,merchant_id,status) SELECT $1,$2,id,sha256,$3,$4 FROM platform_app.integration_releases WHERE id=$5`, id, org, f.tenant, status, releaseID)
		return id
	}
	pending := seedAssignment("requested")
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+pending+"/status", service.AssignmentAction{Status: "approved", Revision: 1, Reason: "Cannot approve payment"}, key(), 409)
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+pending+"/status", service.AssignmentAction{Status: "rejected", Revision: 1, Reason: "Archive payment"}, key(), 200)
	legacy := seedAssignment("approved")
	shipping := approvedTestAssignment(t, dev, admin, f.tenant)
	repo := apprepo.Postgres{Pool: f.pool}
	rows, next, e := repo.AssignmentList(ctx, "", f.tenant, "", 1)
	if e != nil || len(rows) != 1 || rows[0].ID != shipping || next != "" {
		t.Fatal("merchant filtering/pagination", rows, next, e)
	}
	pexpect(t, dev, "GET", "/api/v1/developer/test-assignments/"+legacy, nil, "", 200)
	pexpect(t, other, "GET", "/api/v1/developer/test-assignments/"+legacy, nil, "", 404)
	pexpect(t, admin, "POST", "/api/v1/admin/test-assignments/"+legacy+"/status", service.AssignmentAction{Status: "revoked", Revision: 1, Reason: "End distribution"}, key(), 200)
	pexpect(t, admin, "POST", "/api/v1/admin/integration-releases/"+releaseID+"/status", service.IntegrationAction{Status: "suspended", Revision: 1, Reason: "Archive payment"}, key(), 200)
}
