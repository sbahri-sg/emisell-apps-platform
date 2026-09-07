package bootstrap_test

import (
	"emisell.app/platform/internal/bootstrap"
	"encoding/json"
	"testing"
)

func TestUIResourceAssignmentIsolation(t *testing.T) {
	f := setup(t)
	ctx := t.Context()
	release, client, app, org, assignment := "release_"+key(), "client_"+key(), "app_"+key(), "org_"+key(), "assignment_"+key()
	// Synthetic fixtures only; production enrollment must use reviewed authoring.
	document, _ := json.Marshal(map[string]any{"id": release})
	if _, err := f.pool.Exec(ctx, `INSERT INTO platform_app.ui_resource_releases(id,app_id,organization_id,version,document) VALUES($1,$2,$3,'0.1.0',$4)`, release, app, org, document); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO platform_oauth.app_clients(id,organization_id,release_id,binding,status,revision,challenge_id,challenge,challenge_expires_at,last_result) VALUES($1,$2,$3,'{}','pending',1,'challenge','proof',now()+interval '1 hour','')`, client, org, release); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO platform_app.ui_resource_assignments(id,merchant_id,organization_id,app_id,version,release_id,client_id,status,actor_id,reason) VALUES($1,$2,$3,$4,'0.1.0',$5,$6,'approved','test-admin','Isolated test')`, assignment, f.tenant, org, app, release, client); err != nil {
		t.Fatal(err)
	}
	source := bootstrap.PostgresResourceAssignments{Pool: f.pool}
	called := 0
	callback := func(o, r, c string) error {
		called++
		if o != org || r != release || c != client {
			t.Fatal("binding mismatch")
		}
		return nil
	}
	if err := source.WithApproved(ctx, f.tenant, app, "0.1.0", callback); err != nil || called != 1 {
		t.Fatal(err)
	}
	if source.WithApproved(ctx, f.other, app, "0.1.0", callback) == nil || called != 1 {
		t.Fatal("cross tenant")
	}
	if source.WithApproved(ctx, f.tenant, app, "0.2.0", callback) == nil {
		t.Fatal("wrong version")
	}
	if _, err := f.pool.Exec(ctx, `UPDATE platform_app.ui_resource_assignments SET merchant_id=$2 WHERE id=$1`, assignment, f.other); err == nil {
		t.Fatal("mutable merchant")
	}
	if _, err := f.pool.Exec(ctx, `UPDATE platform_app.ui_resource_assignments SET status='revoked' WHERE id=$1`, assignment); err != nil {
		t.Fatal(err)
	}
	if source.WithApproved(ctx, f.tenant, app, "0.1.0", callback) == nil || called != 1 {
		t.Fatal("revoked accepted")
	}
	if _, err := f.pool.Exec(ctx, `UPDATE platform_app.ui_resource_assignments SET status='approved' WHERE id=$1`, assignment); err == nil {
		t.Fatal("revocation undone")
	}
}
