package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) WithExtensionConnection(ctx context.Context, sel ports.ExtensionConnectionSelector, use func(*ports.ExtensionConnectionState) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if sel.TokenHash != "" {
		// The caller cannot supply/override any of these identities.
		err := tx.QueryRow(ctx, `SELECT a.organization_id::text,c.app_id::text,c.installation_id::text,c.extension_id::text
            FROM extension_connections c JOIN apps a ON a.id=c.app_id WHERE c.token_hash=$1`, sel.TokenHash).Scan(&sel.OrganizationID, &sel.AppID, &sel.InstallationID, &sel.ExtensionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrUnauthorized
		}
		if err != nil {
			return err
		}
	}
	var state ports.ExtensionConnectionState
	// Serialize provisioning/resolve against lifecycle changes, and retain the
	// installation lock until the access audit and read transaction commit.
	state.App, err = getApp(ctx, tx, sel.OrganizationID, sel.AppID, true)
	if err != nil {
		return err
	}
	var orgStatus string
	if err = tx.QueryRow(ctx, `SELECT status FROM organizations WHERE id=$1::uuid FOR SHARE`, sel.OrganizationID).Scan(&orgStatus); err != nil {
		return mapError(err)
	}
	state.OrganizationActive = orgStatus == "active"
	state.Extension, err = getExtension(ctx, tx, sel.OrganizationID, sel.AppID, sel.ExtensionID, true)
	if err != nil {
		return err
	}
	state.Installation, err = getInstallation(ctx, tx, sel.OrganizationID, sel.AppID, sel.InstallationID, true)
	if err != nil {
		return err
	}
	state.Version, err = getVersion(ctx, tx, sel.OrganizationID, sel.AppID, state.Installation.InstalledVersionID, false)
	if err != nil {
		return err
	}
	var sandbox, production bool
	err = tx.QueryRow(ctx, `SELECT sandbox_access,production_access FROM organization_entitlements WHERE organization_id=$1::uuid FOR SHARE`, sel.OrganizationID).Scan(&sandbox, &production)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	state.Entitled = (state.Installation.Environment == domain.EnvironmentSandbox && sandbox) || (state.Installation.Environment == domain.EnvironmentProduction && production)
	var value domain.ExtensionConnection
	var scopes []byte
	err = tx.QueryRow(ctx, `SELECT id::text,app_id::text,installation_id::text,extension_id::text,runtime_name,scopes,status,revision,runtime_expires_at,created_at,updated_at,COALESCE(token_hash,''),secret_ciphertext,encryption_key_version
        FROM extension_connections WHERE app_id=$1::uuid AND installation_id=$2::uuid AND extension_id=$3::uuid FOR UPDATE`, sel.AppID, sel.InstallationID, sel.ExtensionID).Scan(
		&value.ID, &value.AppID, &value.InstallationID, &value.ExtensionID, &value.RuntimeName, &scopes, &value.Status, &value.Revision, &value.RuntimeExpiresAt, &value.CreatedAt, &value.UpdatedAt, &value.TokenHash, &value.Ciphertext, &value.KeyVersion)
	if err == nil {
		if err = json.Unmarshal(scopes, &value.Scopes); err != nil {
			return err
		}
		state.Connection = &value
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if sel.TokenHash != "" && (state.Connection == nil || state.Connection.TokenHash != sel.TokenHash) {
		return domain.ErrUnauthorized
	}
	if err = use(&state); err != nil {
		return err
	}
	if state.Write != nil {
		v := state.Write
		encoded, err := json.Marshal(v.Scopes)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO extension_connections(id,app_id,installation_id,extension_id,runtime_name,scopes,status,revision,runtime_expires_at,created_at,updated_at,token_hash,secret_ciphertext,encryption_key_version)
            VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6::jsonb,$7,$8,$9,$10,$11,NULLIF($12,''),$13,$14)
            ON CONFLICT(installation_id,extension_id) DO UPDATE SET runtime_name=EXCLUDED.runtime_name,scopes=EXCLUDED.scopes,status=EXCLUDED.status,revision=EXCLUDED.revision,runtime_expires_at=EXCLUDED.runtime_expires_at,updated_at=EXCLUDED.updated_at,token_hash=EXCLUDED.token_hash,secret_ciphertext=EXCLUDED.secret_ciphertext,encryption_key_version=EXCLUDED.encryption_key_version`,
			v.ID, v.AppID, v.InstallationID, v.ExtensionID, v.RuntimeName, encoded, v.Status, v.Revision, v.RuntimeExpiresAt, v.CreatedAt, v.UpdatedAt, v.TokenHash, v.Ciphertext, v.KeyVersion)
		if err != nil {
			return mapError(err)
		}
		if err = r.appendAudit(ctx, tx, state.App.OrganizationID, state.Audit, "extension_connection", v.ID, map[string]any{"installationId": v.InstallationID, "extensionId": v.ExtensionID, "revision": v.Revision, "status": v.Status}); err != nil {
			return err
		}
	}
	if state.AccessScope != "" {
		id, err := r.id()
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO extension_credential_access_events(id,connection_id,connection_revision,scope,request_id,created_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6)`, id, state.Connection.ID, state.Connection.Revision, state.AccessScope, state.RequestID, r.now().UTC())
		if err != nil {
			return err
		}
	}
	return mapError(tx.Commit(ctx))
}
