package repository

import (
	"context"
	"encoding/json"
	"errors"

	"emisell.app/platform/internal/app/service"
	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"github.com/jackc/pgx/v5"
)

// JSON projection keeps SQL column ordering independent of the domain struct.
const assignmentJSON = `jsonb_build_object('id',id,'organizationId',organization_id,'releaseId',COALESCE(release_id,managed_release_id,ui_release_id),'releaseKind',CASE WHEN ui_release_id IS NOT NULL THEN 'ui' WHEN managed_release_id IS NOT NULL THEN 'managed_shipping' ELSE '' END,'releaseSha256',release_sha256,'merchantId',merchant_id,'status',status,'revision',revision,'createdAt',created_at,'updatedAt',updated_at)`

func (p Postgres) AssignmentReplay(ctx context.Context, org, actor, key, hash string) (*service.Assignment, error) {
	var storedHash, id string
	err := p.Pool.QueryRow(ctx, `SELECT request_hash,assignment_id FROM platform_app.test_assignment_requests WHERE actor_id=$1 AND request_key=$2`, actor, key).Scan(&storedHash, &id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if storedHash != hash {
		return nil, fault.Conflict
	}
	a, err := p.AssignmentGet(ctx, org, id)
	return &a, err
}

func scanAssignment(row pgx.Row) (service.Assignment, error) {
	var raw []byte
	var a service.Assignment
	err := row.Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return a, fault.NotFound
	}
	if err != nil {
		return a, err
	}
	err = json.Unmarshal(raw, &a)
	return a, err
}
func (p Postgres) AssignmentGet(ctx context.Context, org, id string) (service.Assignment, error) {
	return scanAssignment(p.Pool.QueryRow(ctx, `SELECT `+assignmentJSON+` FROM platform_app.test_assignments WHERE id=$1 AND ($2='' OR organization_id=$2)`, id, org))
}
func (p Postgres) AssignmentList(ctx context.Context, org, merchant, after string, size int) ([]service.Assignment, string, error) {
	return p.assignmentList(ctx, org, merchant, after, size, false)
}
func (p Postgres) UIAssignmentList(ctx context.Context, merchant, after string, size int) ([]service.Assignment, string, error) {
	return p.assignmentList(ctx, "", merchant, after, size, true)
}
func (p Postgres) assignmentList(ctx context.Context, org, merchant, after string, size int, ui bool) ([]service.Assignment, string, error) {
	// Filter before pagination. Portal history is unchanged; merchant distribution
	// only includes currently public capabilities from this module's releases.
	rows, err := p.Pool.Query(ctx, `SELECT `+assignmentJSON+` FROM platform_app.test_assignments WHERE ($1='' OR organization_id=$1) AND ($2='' OR (merchant_id=$2 AND status='approved' AND (($6 AND ui_release_id IS NOT NULL) OR managed_release_id IS NOT NULL OR release_id IN (SELECT id FROM platform_app.integration_releases WHERE manifest->'metadata'->>'capability'=$5)))) AND id>$3 ORDER BY id LIMIT $4`, org, merchant, after, size+1, service.PublicCapability, ui)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	out := []service.Assignment{}
	for rows.Next() {
		a, e := scanAssignment(rows)
		if e != nil {
			return nil, "", e
		}
		out = append(out, a)
	}
	if err = rows.Err(); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > size {
		out = out[:size]
		next = out[len(out)-1].ID
	}
	return out, next, nil
}
func (p Postgres) AssignmentHistory(ctx context.Context, id string) ([]service.CatalogAudit, error) {
	rows, err := p.Pool.Query(ctx, `SELECT id,actor_id,action,reason,occurred_at FROM platform_app.test_assignment_audit WHERE assignment_id=$1 ORDER BY occurred_at,id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []service.CatalogAudit{}
	for rows.Next() {
		var a service.CatalogAudit
		if err = rows.Scan(&a.ID, &a.ActorID, &a.Action, &a.Reason, &a.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (p Postgres) AssignmentMutate(ctx context.Context, org, actor, key, hash, id string, create *service.Assignment, reason string, change func(service.Assignment) (service.Assignment, error)) (service.Assignment, error) {
	var a service.Assignment
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return a, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "testing:"+actor+":"+key); err != nil {
		return a, err
	}
	var oldHash, oldID string
	err = tx.QueryRow(ctx, `SELECT request_hash,assignment_id FROM platform_app.test_assignment_requests WHERE actor_id=$1 AND request_key=$2`, actor, key).Scan(&oldHash, &oldID)
	if err == nil {
		if oldHash != hash {
			return a, fault.Conflict
		}
		return scanAssignment(tx.QueryRow(ctx, `SELECT `+assignmentJSON+` FROM platform_app.test_assignments WHERE id=$1 AND ($2='' OR organization_id=$2)`, oldID, org))
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return a, err
	}
	if create != nil {
		a = *create
		a.ID = ids.New("testasgn")
		var integrationID, managedID, uiID *string
		if a.ReleaseKind == "ui" {
			uiID = &a.ReleaseID
		} else if a.ReleaseKind == "managed_shipping" {
			managedID = &a.ReleaseID
		} else if a.ReleaseKind == "" {
			integrationID = &a.ReleaseID
		} else {
			return a, fault.Invalid
		}
		err = tx.QueryRow(ctx, `INSERT INTO platform_app.test_assignments(id,organization_id,release_id,managed_release_id,release_sha256,merchant_id,status,ui_release_id) VALUES($1,$2,$3,$4,$5,$6,'requested',$7) RETURNING created_at,updated_at`, a.ID, a.OrganizationID, integrationID, managedID, a.ReleaseSHA256, a.MerchantID, uiID).Scan(&a.CreatedAt, &a.UpdatedAt)
	} else {
		a, err = scanAssignment(tx.QueryRow(ctx, `SELECT `+assignmentJSON+` FROM platform_app.test_assignments WHERE id=$1 AND ($2='' OR organization_id=$2) FOR UPDATE`, id, org))
		if err != nil {
			return a, err
		}
		a, err = change(a)
		if err != nil {
			return a, err
		}
		err = tx.QueryRow(ctx, `UPDATE platform_app.test_assignments SET status=$2,revision=$3,updated_at=now() WHERE id=$1 RETURNING updated_at`, a.ID, a.Status, a.Revision).Scan(&a.UpdatedAt)
	}
	if err != nil {
		return a, conflictCatalog(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.test_assignment_requests(actor_id,request_key,request_hash,assignment_id) VALUES($1,$2,$3,$4)`, actor, key, hash, a.ID); err != nil {
		return a, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.test_assignment_audit(id,assignment_id,actor_id,action,reason) VALUES($1,$2,$3,$4,$5)`, ids.New("aud"), a.ID, actor, a.Status, reason); err != nil {
		return a, err
	}
	return a, tx.Commit(ctx)
}
