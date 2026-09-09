package merchantlogin

import (
	"context"
	"emisell.app/platform/internal/platform/fault"
)

func (p Repository) IsCLI(ctx context.Context, request string) (bool, error) {
	var cli bool
	err := p.Pool.QueryRow(ctx, `SELECT login_kind='cli' FROM platform_identity.developer_login_requests WHERE id=$1 AND expires_at>now()`, request).Scan(&cli)
	return cli, err
}

// The browser holds the Core-issued single-use code, never the CLI verifier.
func (p Repository) CLIConfirmation(ctx context.Context, request, code, origin string, confirm bool) (string, error) {
	if !requestID.MatchString(request) || !requestID.MatchString(code) {
		return "", fault.Unauthenticated
	}
	var email string
	query := `SELECT core_email FROM platform_identity.developer_login_requests WHERE id=$1 AND code_hash=$2 AND portal_origin=$3 AND login_kind='cli' AND expires_at>now()`
	if confirm {
		query = `UPDATE platform_identity.developer_login_requests SET cli_confirmed=true WHERE id=$1 AND code_hash=$2 AND portal_origin=$3 AND login_kind='cli' AND expires_at>now() RETURNING core_email`
	}
	if err := p.Pool.QueryRow(ctx, query, request, Hash(code), origin).Scan(&email); err != nil {
		return "", fault.Unauthenticated
	}
	return email, nil
}

func (p Repository) PollCLI(ctx context.Context, request, verifier, origin string) (string, error) {
	if !requestID.MatchString(request) || !requestID.MatchString(verifier) {
		return "", fault.Unauthenticated
	}
	var ready bool
	err := p.Pool.QueryRow(ctx, `SELECT cli_confirmed FROM platform_identity.developer_login_requests WHERE id=$1 AND verifier_hash=$2 AND portal_origin=$3 AND login_kind='cli' AND expires_at>now()`, request, Hash(verifier), origin).Scan(&ready)
	if err != nil {
		return "", fault.Unauthenticated
	}
	if !ready {
		return "", nil
	}
	return p.finish(ctx, request, verifier, "", origin, true)
}
