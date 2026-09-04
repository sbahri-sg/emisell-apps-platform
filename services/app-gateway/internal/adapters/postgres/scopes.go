package postgres

import (
	"context"
	"fmt"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

func listScopes(ctx context.Context, source interface {
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
}, organizationID, appID string) ([]domain.AppScope, error) {
	rows, err := source.Query(ctx, `
		SELECT s.app_id::text, s.scope, s.access
		FROM app_scopes s
		JOIN apps a ON a.id = s.app_id
		WHERE a.organization_id = $1::uuid AND a.id = $2::uuid
		ORDER BY s.access, s.scope`, organizationID, appID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()
	items := make([]domain.AppScope, 0)
	for rows.Next() {
		var item domain.AppScope
		var access string
		if err := rows.Scan(&item.AppID, &item.Scope, &access); err != nil {
			return nil, mapError(err)
		}
		item.Access = domain.ScopeAccess(access)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}

func (r *Repository) ListScopes(ctx context.Context, organizationID, appID string) ([]domain.AppScope, error) {
	if _, err := getApp(ctx, r.pool, organizationID, appID, false); err != nil {
		return nil, err
	}
	return listScopes(ctx, r.pool, organizationID, appID)
}

func (r *Repository) ReplaceScopes(ctx context.Context, organizationID, appID string, scopes []domain.AppScope, meta ports.MutationMeta) ([]domain.AppScope, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin replace scopes: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := getApp(ctx, tx, organizationID, appID, true); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM app_scopes WHERE app_id = $1::uuid`, appID); err != nil {
		return nil, mapError(err)
	}
	for _, item := range scopes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO app_scopes (app_id, scope, access, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, $4, $4)`, appID, item.Scope, item.Access, r.now().UTC()); err != nil {
			return nil, mapError(err)
		}
	}
	if err := r.appendAudit(ctx, tx, organizationID, meta, "app", appID, map[string]interface{}{"scopeCount": len(scopes)}); err != nil {
		return nil, err
	}
	items, err := listScopes(ctx, tx, organizationID, appID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, mapError(err)
	}
	return items, nil
}
