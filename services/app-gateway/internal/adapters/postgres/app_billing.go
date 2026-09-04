package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
	"github.com/jackc/pgx/v5"
)

func billingPlans(ctx context.Context, tx pgx.Tx, appID string) ([]domain.AppPlan, error) {
	rows, err := tx.Query(ctx, `SELECT document,request_key,request_hash FROM app_plans WHERE app_id=$1 ORDER BY id`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := []domain.AppPlan{}
	for rows.Next() {
		var encoded []byte
		var p domain.AppPlan
		if err := rows.Scan(&encoded, &p.RequestKey, &p.RequestHash); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(encoded, &p); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (r *Repository) WithAppPlans(ctx context.Context, org, appID string, use func(domain.App, *[]domain.AppPlan) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	app, err := getApp(ctx, tx, org, appID, true)
	if err != nil {
		return err
	}
	plans, err := billingPlans(ctx, tx, appID)
	if err != nil {
		return err
	}
	before, err := json.Marshal(plans)
	if err != nil {
		return err
	}
	if err := use(app, &plans); err != nil {
		return err
	}
	after, err := json.Marshal(plans)
	if err != nil {
		return err
	}
	if bytes.Equal(before, after) {
		return mapError(tx.Commit(ctx))
	}
	for _, p := range plans {
		encoded, err := json.Marshal(p)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO app_plans(id,app_id,request_key,request_hash,document) VALUES($1,$2,$3,$4,$5)
            ON CONFLICT(id) DO UPDATE SET document=EXCLUDED.document WHERE app_plans.document IS DISTINCT FROM EXCLUDED.document`, p.ID, p.AppID, p.RequestKey, p.RequestHash, encoded)
		if err != nil {
			return mapError(err)
		}
	}
	return mapError(tx.Commit(ctx))
}

// The table argument is selected exclusively from the static calls below.
func loadBillingDocuments[T any](ctx context.Context, tx pgx.Tx, table, merchant string, env domain.Environment) ([]T, error) {
	rows, err := tx.Query(ctx, `SELECT document FROM `+table+` WHERE merchant_id=$1 AND environment=$2 ORDER BY document->>'id'`, merchant, string(env))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []T{}
	for rows.Next() {
		var data []byte
		var item T
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) WithMerchantBilling(ctx context.Context, merchant string, env domain.Environment, use func(*ports.AppBillingState) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "app-billing:"+merchant+":"+string(env)); err != nil {
		return err
	}
	state := ports.AppBillingState{AvailableApps: map[string]bool{}, Plans: []domain.AppPlan{}, Apps: []domain.App{}, Installations: []domain.AppInstallation{}}
	var account []byte
	err = tx.QueryRow(ctx, `SELECT account FROM merchant_app_billing WHERE merchant_id=$1 AND environment=$2`, merchant, string(env)).Scan(&account)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if len(account) > 0 {
		if err := json.Unmarshal(account, &state.Account); err != nil {
			return err
		}
	}
	// Lock apps first, then organizations and installations, matching lifecycle
	// writers. Uninstall's trigger and invoice/consent therefore commit atomically.
	rows, err := tx.Query(ctx, `SELECT DISTINCT a.id::text,a.organization_id::text FROM apps a JOIN app_installations i ON i.app_id=a.id WHERE i.merchant_id=$1 AND i.environment=$2 ORDER BY a.id::text`, merchant, string(env))
	if err != nil {
		return err
	}
	type appKey struct{ id, org string }
	keys := []appKey{}
	for rows.Next() {
		var key appKey
		if err := rows.Scan(&key.id, &key.org); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	for _, key := range keys {
		app, err := getApp(ctx, tx, key.org, key.id, true)
		if err != nil {
			return err
		}
		state.Apps = append(state.Apps, app)
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM organizations WHERE id=$1 FOR SHARE`, key.org).Scan(&status); err != nil {
			return err
		}
		var sandbox, production bool
		err = tx.QueryRow(ctx, `SELECT sandbox_access,production_access FROM organization_entitlements WHERE organization_id=$1 FOR SHARE`, key.org).Scan(&sandbox, &production)
		if errors.Is(err, pgx.ErrNoRows) {
			sandbox = true
		} // Built-in development organization; production stays denied.
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		state.AvailableApps[app.ID] = status == "active" && app.Status != domain.AppStatusArchived && ((env == domain.EnvironmentSandbox && sandbox) || (env == domain.EnvironmentProduction && production && app.Status == domain.AppStatusActive))
		plans, err := billingPlans(ctx, tx, key.id)
		if err != nil {
			return err
		}
		state.Plans = append(state.Plans, plans...)
	}
	rows, err = tx.Query(ctx, `SELECT `+installationColumns+` FROM app_installations WHERE merchant_id=$1 AND environment=$2 ORDER BY app_id,id FOR UPDATE`, merchant, string(env))
	if err != nil {
		return err
	}
	for rows.Next() {
		item, err := scanInstallation(rows)
		if err != nil {
			rows.Close()
			return err
		}
		state.Installations = append(state.Installations, item)
	}
	rows.Close()
	if rows.Err() != nil {
		return rows.Err()
	}
	state.Quotes, err = loadBillingDocuments[domain.AppSubscriptionQuote](ctx, tx, "app_billing_quotes", merchant, env)
	if err != nil {
		return err
	}
	state.Subscriptions, err = loadBillingDocuments[domain.AppSubscription](ctx, tx, "app_subscriptions", merchant, env)
	if err != nil {
		return err
	}
	state.Charges, err = loadBillingDocuments[domain.AppSubscriptionCharge](ctx, tx, "app_subscription_charges", merchant, env)
	if err != nil {
		return err
	}
	state.Invoices, err = loadBillingDocuments[domain.AppBillingInvoice](ctx, tx, "app_billing_invoices", merchant, env)
	if err != nil {
		return err
	}
	state.Events, err = loadBillingDocuments[domain.AppBillingEvent](ctx, tx, "app_billing_events", merchant, env)
	if err != nil {
		return err
	}
	before, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := use(&state); err != nil {
		return err
	}
	after, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if bytes.Equal(before, after) {
		return mapError(tx.Commit(ctx))
	}
	if err := saveMerchantBilling(ctx, tx, merchant, env, &state); err != nil {
		return mapError(err)
	}
	return mapError(tx.Commit(ctx))
}

func saveMerchantBilling(ctx context.Context, tx pgx.Tx, merchant string, env domain.Environment, state *ports.AppBillingState) error {
	var account []byte
	var err error
	if state.Account != nil {
		account, err = json.Marshal(state.Account)
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO merchant_app_billing(merchant_id,environment,account) VALUES($1,$2,$3) ON CONFLICT(merchant_id,environment) DO UPDATE SET account=EXCLUDED.account`, merchant, string(env), account); err != nil {
		return err
	}
	for _, q := range state.Quotes {
		data, err := json.Marshal(q)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app_billing_quotes(id,merchant_id,environment,installation_id,document) VALUES($1,$2,$3,$4,$5) ON CONFLICT(id) DO NOTHING`, q.ID, merchant, string(env), q.InstallationID, data); err != nil {
			return err
		}
	}
	// Save cancellations before new subscriptions to respect the live-plan index.
	for _, cancelled := range []bool{true, false} {
		for _, sub := range state.Subscriptions {
			if (sub.Status == "cancelled") != cancelled {
				continue
			}
			data, err := json.Marshal(sub)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO app_subscriptions(id,merchant_id,environment,installation_id,quote_id,document) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(id) DO UPDATE SET document=EXCLUDED.document WHERE app_subscriptions.document IS DISTINCT FROM EXCLUDED.document`, sub.ID, merchant, string(env), sub.InstallationID, sub.QuoteID, data); err != nil {
				return err
			}
		}
	}
	for _, invoice := range state.Invoices {
		data, err := json.Marshal(invoice)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app_billing_invoices(merchant_id,environment,invoice_id,document) VALUES($1,$2,$3,$4) ON CONFLICT(merchant_id,environment,invoice_id) DO UPDATE SET document=EXCLUDED.document WHERE app_billing_invoices.document IS DISTINCT FROM EXCLUDED.document`, merchant, string(env), invoice.InvoiceID, data); err != nil {
			return err
		}
	}
	for _, charge := range state.Charges {
		data, err := json.Marshal(charge)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app_subscription_charges(id,merchant_id,environment,subscription_id,period_start,invoice_id,document) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7) ON CONFLICT(id) DO UPDATE SET invoice_id=EXCLUDED.invoice_id,document=EXCLUDED.document WHERE app_subscription_charges.document IS DISTINCT FROM EXCLUDED.document`, charge.ID, merchant, string(env), charge.SubscriptionID, charge.PeriodStart, charge.InvoiceID, data); err != nil {
			return err
		}
	}
	for _, event := range state.Events {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO app_billing_events(merchant_id,environment,event_id,document) VALUES($1,$2,$3,$4) ON CONFLICT(merchant_id,environment,event_id) DO NOTHING`, merchant, string(env), event.ID, data); err != nil {
			return fmt.Errorf("persist billing audit: %w", err)
		}
	}
	return nil
}
