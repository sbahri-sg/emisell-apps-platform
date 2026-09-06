package postgres

import "context"

func (p Repository) KnownMerchant(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := p.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM platform_identity.workspaces WHERE id=$1)`, id).Scan(&ok)
	return ok, err
}
