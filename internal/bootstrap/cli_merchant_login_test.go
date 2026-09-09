package bootstrap_test

import (
	"context"
	"emisell.app/platform/internal/identity/merchantlogin"
	"testing"
)

func TestCLILoginProofConsentExpiryAndReplay(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	repo := merchantlogin.Repository{Pool: f.pool}
	subject := "cli-owner-" + key()
	start := func() (string, string, string) {
		request, verifier, err := repo.StartCLI(ctx, "http://localhost:4317")
		if err != nil {
			t.Fatal(err)
		}
		token, err := repo.PollCLI(ctx, request, verifier, "http://localhost:4317")
		if err != nil || token != "" {
			t.Fatal("unapproved login", err)
		}
		_, code, err := repo.Approve(ctx, merchantlogin.Assertion{Request: request, Subject: subject, Name: "CLI Owner", Email: "cli@example.invalid", Stores: []merchantlogin.Store{{ID: f.tenant, CommonID: "my-store", Name: "Store"}}})
		if err != nil {
			t.Fatal(err)
		}
		return request, verifier, code
	}
	request, verifier, code := start()
	if _, err := repo.PollCLI(ctx, request, merchantlogin.Token(), "http://localhost:4317"); err == nil {
		t.Fatal("wrong proof accepted")
	}
	if _, err := repo.PollCLI(ctx, request, verifier, "http://localhost:4319"); err == nil {
		t.Fatal("wrong origin accepted")
	}
	if token, err := repo.PollCLI(ctx, request, verifier, "http://localhost:4317"); err != nil || token != "" {
		t.Fatal("Core approval auto-authorized CLI")
	}
	if _, err := repo.Finish(ctx, request, verifier, code, "http://localhost:4317"); err == nil {
		t.Fatal("browser consumed CLI request")
	}
	if _, err := repo.CLIConfirmation(ctx, request, merchantlogin.Token(), "http://localhost:4317", true); err == nil {
		t.Fatal("wrong code confirmed")
	}
	if _, err := repo.CLIConfirmation(ctx, request, code, "http://localhost:4317", true); err != nil {
		t.Fatal(err)
	}
	token, err := repo.PollCLI(ctx, request, verifier, "http://localhost:4317")
	if err != nil || len(token) != 52 {
		t.Fatal("login failed", err)
	}
	if _, err := repo.PollCLI(ctx, request, verifier, "http://localhost:4317"); err == nil {
		t.Fatal("replayed CLI login")
	}
	request, verifier, code = start()
	_, _ = f.pool.Exec(ctx, `UPDATE platform_identity.portal_accounts SET enabled=false WHERE id=(SELECT account_id FROM platform_identity.developer_core_links WHERE core_subject=$1)`, subject)
	_, _ = repo.CLIConfirmation(ctx, request, code, "http://localhost:4317", true)
	if _, err := repo.PollCLI(ctx, request, verifier, "http://localhost:4317"); err == nil {
		t.Fatal("disabled owner authenticated")
	}
	_, _ = f.pool.Exec(ctx, `UPDATE platform_identity.developer_login_requests SET expires_at=now()-interval '1 second' WHERE id=$1`, request)
	if _, err := repo.PollCLI(ctx, request, verifier, "http://localhost:4317"); err == nil {
		t.Fatal("expired request authenticated")
	}
	browser, proof, err := repo.Start(ctx, "http://localhost:4317")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.PollCLI(ctx, browser, proof, "http://localhost:4317"); err == nil {
		t.Fatal("CLI consumed browser request")
	}
}
