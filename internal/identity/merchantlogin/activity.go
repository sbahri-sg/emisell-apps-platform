package merchantlogin

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

// Only explicit foreground user activity renews a valid session. Polling cannot revive it.
func (p Repository) Activity(ctx context.Context, token string) (time.Time, error) {
	var expires time.Time
	err := p.Pool.QueryRow(ctx, `UPDATE platform_identity.portal_sessions s SET last_active_at=now(),expires_at=now()+interval '1 hour'
 FROM platform_identity.portal_accounts a,platform_identity.developer_core_links l
 WHERE s.token_hash=$1 AND s.surface='developer' AND a.id=s.account_id AND a.enabled
 AND l.account_id=a.id AND s.expires_at>now() AND s.last_active_at>now()-interval '1 hour'
 RETURNING s.expires_at`, Hash(token)).Scan(&expires)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.Unauthenticated
	}
	return expires, err
}

// ActivityExpiry is read-only: other tabs and background checks must not renew a session.
func (p Repository) ActivityExpiry(ctx context.Context, token string) (time.Time, error) {
	var expires time.Time
	err := p.Pool.QueryRow(ctx, `SELECT least(s.expires_at,s.last_active_at+interval '1 hour')
 FROM platform_identity.portal_sessions s JOIN platform_identity.portal_accounts a ON a.id=s.account_id
 JOIN platform_identity.developer_core_links l ON l.account_id=a.id
 WHERE s.token_hash=$1 AND s.surface='developer' AND a.enabled
 AND s.expires_at>now() AND s.last_active_at>now()-interval '1 hour'`, Hash(token)).Scan(&expires)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.Unauthenticated
	}
	return expires, err
}
