package contact

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"emisell.app/platform/internal/platform/fault"
	"emisell.app/platform/internal/platform/ids"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Contact struct {
	AppID     string     `json:"appId"`
	Email     string     `json:"email"`
	Revision  int        `json:"revision"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

type Repository struct{ Pool *pgxpool.Pool }

func ValidEmail(email string) bool {
	if len(email) > 254 || strings.ContainsAny(email, "\r\n\t ") {
		return false
	}
	address, err := mail.ParseAddress(email)
	return err == nil && address.Name == "" && address.Address == email && strings.Contains(email, "@")
}

func (p Repository) Get(ctx context.Context, org, app string) (Contact, error) {
	var c Contact
	err := p.Pool.QueryRow(ctx, `SELECT d.id,coalesce(c.email,''),coalesce(c.revision,0),c.updated_at
 FROM platform_app.drafts d LEFT JOIN platform_app.contacts c ON c.app_id=d.id
 WHERE d.id=$1 AND d.organization_id=$2`, app, org).Scan(&c.AppID, &c.Email, &c.Revision, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = fault.NotFound
	}
	return c, err
}

func (p Repository) Save(ctx context.Context, org, app, actor, email string, revision int) (Contact, error) {
	var c Contact
	if !ValidEmail(email) || revision < 0 || actor == "" {
		return c, fault.Invalid
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return c, err
	}
	defer tx.Rollback(ctx)
	var id string
	// Lock the owning draft also when no contact exists yet.
	err = tx.QueryRow(ctx, `SELECT id FROM platform_app.drafts WHERE id=$1 AND organization_id=$2 FOR UPDATE`, app, org).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, fault.NotFound
	}
	if err != nil {
		return c, err
	}
	var current int
	if err = tx.QueryRow(ctx, `SELECT coalesce((SELECT revision FROM platform_app.contacts WHERE app_id=$1),0)`, app).Scan(&current); err != nil {
		return c, err
	}
	if current != revision {
		return c, fault.Conflict
	}
	err = tx.QueryRow(ctx, `INSERT INTO platform_app.contacts(app_id,email,revision) VALUES($1,$2,1)
 ON CONFLICT(app_id) DO UPDATE SET email=excluded.email,revision=platform_app.contacts.revision+1,updated_at=now()
 RETURNING app_id,email,revision,updated_at`, app, email).Scan(&c.AppID, &c.Email, &c.Revision, &c.UpdatedAt)
	if err != nil {
		return c, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO platform_app.contact_audit(id,app_id,actor_id,revision) VALUES($1,$2,$3,$4)`, ids.New("aud"), app, actor, c.Revision); err != nil {
		return c, err
	}
	return c, tx.Commit(ctx)
}
