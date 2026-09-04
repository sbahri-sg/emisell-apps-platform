package memory

import (
	"context"
	"encoding/json"
	"sort"

	"emisell-app-platform/services/app-gateway/internal/domain"
	"emisell-app-platform/services/app-gateway/internal/ports"
)

func copyBilling[T any](value T) T {
	data, _ := json.Marshal(value)
	var result T
	_ = json.Unmarshal(data, &result)
	return result
}

func (r *Repository) WithAppPlans(_ context.Context, org, appID string, use func(domain.App, *[]domain.AppPlan) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	app, ok := r.apps[appID]
	if !ok || app.OrganizationID != org {
		return domain.ErrNotFound
	}
	plans := copyBilling(r.appPlans[appID])
	if plans == nil {
		plans = []domain.AppPlan{}
	}
	for i := range plans {
		plans[i].RequestKey = r.appPlans[appID][i].RequestKey
		plans[i].RequestHash = r.appPlans[appID][i].RequestHash
	}
	if err := use(app, &plans); err != nil {
		return err
	}
	r.appPlans[appID] = plans
	return nil
}

func (r *Repository) WithMerchantBilling(_ context.Context, merchant string, env domain.Environment, use func(*ports.AppBillingState) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := merchant + ":" + string(env)
	state := copyBilling(r.appBilling[key])
	state.AvailableApps, state.Apps, state.Installations, state.Plans = map[string]bool{}, []domain.App{}, []domain.AppInstallation{}, []domain.AppPlan{}
	for _, app := range r.apps {
		found := false
		for _, inst := range r.installations[app.ID] {
			if inst.MerchantID == merchant && inst.Environment == env {
				state.Installations = append(state.Installations, inst)
				found = true
			}
		}
		if !found {
			continue
		}
		state.Apps = append(state.Apps, app)
		state.Plans = append(state.Plans, copyBilling(r.appPlans[app.ID])...)
		entitlement, entitled := r.organizationEntitlements[app.OrganizationID]
		if !entitled {
			entitlement.SandboxAccess = true
		} // Same built-in development rule as requireEnvironmentAccess.
		orgActive := true
		if org, ok := r.developerOrganizations[app.OrganizationID]; ok {
			orgActive = org.Status == "active"
		}
		state.AvailableApps[app.ID] = orgActive && app.Status != domain.AppStatusArchived && ((env == domain.EnvironmentSandbox && entitlement.SandboxAccess) || (env == domain.EnvironmentProduction && entitlement.ProductionAccess && app.Status == domain.AppStatusActive))
	}
	sort.Slice(state.Plans, func(i, j int) bool { return state.Plans[i].ID < state.Plans[j].ID })
	if err := use(&state); err != nil {
		return err
	}
	r.appBilling[key] = copyBilling(state)
	return nil
}

func (r *Repository) cancelAppBillingLocked(inst domain.AppInstallation) {
	key := inst.MerchantID + ":" + string(inst.Environment)
	state, ok := r.appBilling[key]
	if !ok {
		return
	}
	now := r.now().UTC()
	for i := range state.Subscriptions {
		sub := &state.Subscriptions[i]
		if sub.InstallationID == inst.ID && sub.Status != "cancelled" {
			sub.Status, sub.CancelledAt, sub.CancellationReason = "cancelled", &now, "uninstalled"
		}
	}
	for i := range state.Charges {
		charge := &state.Charges[i]
		if charge.InstallationID == inst.ID && charge.Status == "unbilled" {
			charge.Status = "void"
		}
	}
	r.appBilling[key] = state
}
