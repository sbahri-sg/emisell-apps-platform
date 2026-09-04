'use client';

import { useEffect, useRef, useState, type FormEvent } from 'react';
import { CreditCard, LoaderCircle, Plus } from 'lucide-react';
import { apiErrorMessage, appPlatformClient, AppPlatformApiError } from '../lib/app-platform/client';
import { appPrice, type AppPlan, type CreateAppPlan } from '../lib/app-platform/billing';
import { useAppPlatform } from './app-platform-store';
import { InlineNotice, Panel, StatusBadge } from './components/ui';

export function AppPlansView({ appId }: { appId: string }) {
  const { session } = useAppPlatform();
  const canManage = session?.role === 'owner' || session?.role === 'admin';
  const [plans, setPlans] = useState<AppPlan[]>([]);
  const [loading, setLoading] = useState(true);
  const [disabled, setDisabled] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [creating, setCreating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [kind, setKind] = useState<'free' | 'monthly'>('free');
  const [currency, setCurrency] = useState<'IDR' | 'USD'>('IDR');
  const [archive, setArchive] = useState<string | null>(null);
  const request = useRef({ body: '', key: '' });

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient.listAppPlans(appId, controller.signal).then(setPlans).catch((err) => {
      if (controller.signal.aborted) return;
      if (err instanceof AppPlatformApiError && err.code === 'app_billing_disabled') setDisabled(true);
      else setError(apiErrorMessage(err));
    }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [appId]);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const raw = String(form.get('price') ?? '0').trim();
    if (kind === 'monthly' && !(currency === 'USD' ? /^\d+(\.\d{1,2})?$/ : /^\d+$/).test(raw)) { setError('Enter a valid price: whole rupiah for IDR, up to two decimal places for USD.'); return; }
    const [whole, fraction = ''] = raw.split('.');
    const amountMinor = kind === 'free' ? 0 : currency === 'USD' ? Number(whole) * 100 + Number(fraction.padEnd(2, '0')) : Number(raw);
    if (!Number.isSafeInteger(amountMinor) || amountMinor > 1_000_000_000_000 || (kind === 'monthly' && amountMinor <= 0)) { setError('Enter a positive price within the supported amount limit.'); return; }
    const input: CreateAppPlan = { name: String(form.get('name')).trim(), description: String(form.get('description')).trim(), amountMinor, currency, interval: kind, features: String(form.get('features')).split('\n').map((line) => line.trim()).filter(Boolean) };
    const body = JSON.stringify(input);
    if (request.current.body !== body) request.current = { body, key: crypto.randomUUID() };
    setSaving(true); setError(''); setNotice('');
    try {
      const plan = await appPlatformClient.createAppPlan(appId, input, request.current.key);
      setPlans((current) => [...current.filter((item) => item.id !== plan.id), plan]);
      request.current = { body: '', key: '' }; setCreating(false); setNotice('Plan created. No merchant has been subscribed or charged.');
    } catch (err) { setError(apiErrorMessage(err)); } finally { setSaving(false); }
  }

  async function archivePlan(id: string) {
    setSaving(true); setError('');
    try { await appPlatformClient.archiveAppPlan(appId, id); setPlans((current) => current.map((p) => p.id === id ? { ...p, status: 'archived' } : p)); setArchive(null); setNotice('Plan archived. Existing subscriptions keep their agreed price.'); }
    catch (err) { setError(apiErrorMessage(err)); } finally { setSaving(false); }
  }

  return <div className="billing-view">
    <header className="billing-heading"><div><p className="section-kicker">App subscriptions</p><h2>Plans & pricing</h2><p>Offer Free and monthly Paid plans. Merchants review the price before subscribing.</p></div><button className="primary-button" disabled={!canManage || disabled || loading || saving} onClick={() => setCreating(!creating)}><Plus size={15} />Create plan</button></header>
    {disabled && <InlineNotice title="App billing is not enabled">An operator must enable the billing module after its database migration. Creating an app does not automatically create a paid subscription.</InlineNotice>}
    {error && <div role="alert"><InlineNotice tone="warning" title="Could not complete the request">{error}</InlineNotice></div>}
    {notice && <p className="billing-feedback" role="status">{notice}</p>}
    {!canManage && <p className="field-hint">Only organization owners and admins can manage pricing.</p>}
    {creating && <Panel className="billing-form-panel"><form className="billing-form" onSubmit={(event) => void submit(event)}><h3>New plan</h3><div className="billing-fields">
      <label>Plan name<input name="name" required minLength={2} maxLength={80} placeholder="e.g. Reviews Pro" disabled={saving} /></label>
      <label>Billing<select value={kind} onChange={(event) => setKind(event.target.value as typeof kind)} disabled={saving}><option value="free">Free</option><option value="monthly">Paid · monthly</option></select></label>
      <label>Currency<select value={currency} onChange={(event) => setCurrency(event.target.value as typeof currency)} disabled={saving}><option value="IDR">IDR · Indonesian rupiah</option><option value="USD">USD · US dollar</option></select></label>
      {kind === 'monthly' && <label>Monthly price<input name="price" inputMode="decimal" required placeholder={currency === 'IDR' ? '100000' : '15.00'} disabled={saving} /></label>}
      <label className="billing-wide">Description<textarea name="description" maxLength={1000} rows={2} disabled={saving} /></label>
      <label className="billing-wide">Included features · one per line<textarea name="features" rows={3} placeholder={'Unlimited reviews\nCustom branding'} disabled={saving} /></label>
    </div><p className="field-hint">The price cannot be edited after creation. Create a new plan for a new price; current merchants keep their approved terms. Prices exclude invoice taxes.</p><div className="billing-actions"><button type="button" className="secondary-button" disabled={saving} onClick={() => setCreating(false)}>Cancel</button><button className="primary-button" disabled={saving}>{saving && <LoaderCircle size={14} className="spin" />}Save plan</button></div></form></Panel>}
    {loading ? <p role="status"><LoaderCircle className="spin" size={16} /> Loading plans…</p> : !disabled && plans.length === 0 ? <Panel><div className="empty-state"><CreditCard size={24} /><strong>No pricing plans yet</strong><p>Add a Free plan, a Paid plan, or both. Existing installations remain unchanged.</p></div></Panel> : <div className="billing-plan-grid">{plans.map((plan) => <Panel className="billing-plan-card" key={plan.id}><div className="billing-plan-title"><h3>{plan.name}</h3><StatusBadge status={plan.status} /></div><p className="billing-price">{plan.interval === 'free' ? 'Free' : appPrice(plan.amountMinor, plan.currency)}{plan.interval !== 'free' && <small> / month</small>}</p><p>{plan.description || 'No description'}</p>{plan.features.length > 0 && <ul>{plan.features.map((feature, index) => <li key={index}>{feature}</li>)}</ul>}<small className="billing-id">Plan ID: {plan.id}</small>{canManage && plan.status === 'active' && (archive === plan.id ? <div className="billing-confirm"><p>Hide this plan from new subscriptions? Existing subscriptions will continue.</p><div className="billing-actions"><button className="secondary-button" disabled={saving} onClick={() => setArchive(null)}>Keep plan</button><button className="danger-button" disabled={saving} onClick={() => void archivePlan(plan.id)}>Archive plan</button></div></div> : <button className="secondary-button" disabled={saving} onClick={() => setArchive(plan.id)}>Archive plan</button>)}</Panel>)}</div>}
  </div>;
}
