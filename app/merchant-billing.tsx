'use client';

import { useEffect, useState } from 'react';
import { LoaderCircle } from 'lucide-react';
import { appPrice, type AppSubscriptionQuote, type InstallationBilling } from '../lib/app-platform/billing';
import { apiErrorMessage, appPlatformClient, AppPlatformApiError } from '../lib/app-platform/client';
import { InlineNotice, StatusBadge } from './components/ui';

const date = (value: string) => new Date(value).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' });

export function MerchantAppBilling({ installationId, appName }: { installationId: string; appName: string }) {
  const [data, setData] = useState<InstallationBilling | null>(null);
  const [error, setError] = useState('');
  const [disabled, setDisabled] = useState(false);
  const [quote, setQuote] = useState<AppSubscriptionQuote | null>(null);
  const [accepted, setAccepted] = useState(false);
  const [busy, setBusy] = useState(false);
  const [cancelling, setCancelling] = useState(false);
  const [notice, setNotice] = useState('');
  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient.getMerchantInstallationBilling(installationId, controller.signal).then(setData).catch((err) => {
      if (controller.signal.aborted) return;
      if (err instanceof AppPlatformApiError && err.code === 'app_billing_disabled') setDisabled(true);
      else setError(apiErrorMessage(err));
    });
    return () => controller.abort();
  }, [installationId]);

  async function review(planId: string) {
    setBusy(true); setError(''); setAccepted(false); setNotice('');
    try { setQuote(await appPlatformClient.quoteAppSubscription(installationId, planId)); }
    catch (err) { setError(apiErrorMessage(err)); } finally { setBusy(false); }
  }

  async function approve() {
    if (!quote || !accepted) return;
    setBusy(true); setError('');
    try {
      const sub = await appPlatformClient.approveAppSubscription(installationId, quote.id);
      setData((current) => current ? { ...current, subscription: sub } : current);
      setQuote(null); setAccepted(false);
      setNotice(sub.plan.interval === 'free' ? 'Free plan activated. No recurring app fee.' : 'Subscription approved. Paid features become available after Emisell confirms payment.');
    } catch (err) { setError(apiErrorMessage(err)); } finally { setBusy(false); }
  }

  async function cancel() {
    if (!data?.subscription) return;
    setBusy(true); setError('');
    try {
      await appPlatformClient.cancelAppSubscription(installationId, data.subscription.id);
      setData(await appPlatformClient.getMerchantInstallationBilling(installationId)); setCancelling(false); setNotice('Renewal cancelled. Already-issued invoices remain payable.');
    } catch (err) { setError(apiErrorMessage(err)); } finally { setBusy(false); }
  }

  const current = data?.subscription;
  const hasCurrent = current && (current.status !== 'cancelled' || (current.paidThrough && new Date(current.paidThrough) > new Date()));
  return <section className="merchant-plan-panel billing-view" aria-label={`${appName} subscription`}>
    <h3>Plan & subscription</h3>
    {disabled && <InlineNotice title="App billing is not enabled">This installation has not been automatically enrolled in a paid plan.</InlineNotice>}
    {error && <div role="alert"><InlineNotice tone="warning" title="Subscription unavailable">{error}</InlineNotice></div>}
    {!data && !error && !disabled && <p role="status"><LoaderCircle className="spin" size={16} /> Loading subscription…</p>}
    {notice && <p className="billing-feedback" role="status">{notice}</p>}
    {data?.test && <InlineNotice title="Test subscription">These records are for development testing. They must never be sent to a live payment provider.</InlineNotice>}
    {data && !data.paidBillingAvailable && <p className="field-hint">Paid subscriptions need an enabled Emisell billing account with a current billing period and matching currency.</p>}
    {current && <div className="billing-current"><div className="billing-plan-title"><h4>{current.plan.name}</h4><StatusBadge status={current.status.replaceAll('_', ' ')} /></div><p>{current.plan.interval === 'free' ? 'Free' : `${appPrice(current.plan.amountMinor, current.plan.currency)} / month`}</p>{current.paidThrough && <p>Paid through {date(current.paidThrough)} · Paid access {data?.paidAccess ? 'available' : 'not available'}</p>}{current.status !== 'cancelled' && (cancelling ? <div className="billing-confirm"><p>Stop future renewal? Unbilled charges will be voided. Issued invoices stay payable; prepaid access lasts until the paid-through date. No automatic refund.</p><div className="billing-actions"><button className="secondary-button" disabled={busy} onClick={() => setCancelling(false)}>Keep subscription</button><button className="danger-button" disabled={busy} onClick={() => void cancel()}>Cancel subscription</button></div></div> : <button className="secondary-button" disabled={busy} onClick={() => setCancelling(true)}>Cancel renewal</button>)}</div>}
    {!quote && data && !hasCurrent && <div className="billing-plan-grid">{data.plans.length === 0 ? <p>No plans are offered for this app yet.</p> : data.plans.map((plan) => <div className="billing-choice" key={plan.id}><h4>{plan.name}</h4><p className="billing-price">{plan.interval === 'free' ? 'Free' : appPrice(plan.amountMinor, plan.currency)}{plan.interval !== 'free' && <small> / month</small>}</p><p>{plan.description}</p>{plan.features.length > 0 && <ul>{plan.features.map((feature, i) => <li key={i}>{feature}</li>)}</ul>}<button className="secondary-button" disabled={busy || (plan.interval !== 'free' && !data.paidBillingAvailable)} onClick={() => void review(plan.id)}>Review plan</button></div>)}</div>}
    {quote && <div className="billing-consent"><h4>Review {quote.plan.name}</h4>{quote.plan.interval === 'free' ? <p>No recurring app fee.</p> : <><dl><div><dt>First app charge · prorated</dt><dd>{appPrice(quote.amountMinor, quote.plan.currency)}</dd></div><div><dt>Initial service period</dt><dd>{date(quote.periodStart)} – {date(quote.periodEnd)}</dd></div><div><dt>Recurring app charge</dt><dd>{appPrice(quote.plan.amountMinor, quote.plan.currency)} / month</dd></div></dl><p>Emisell collects this fee. Renewals are included in your Emisell invoice when due. Taxes, if applicable, appear on the invoice. Paid features start after payment; a late payment does not extend the service period.</p></>}<p className="field-hint">This quote expires {date(quote.expiresAt)}. To change plans, cancel and wait for any prepaid period to end.</p><label className="billing-consent-check"><input type="checkbox" checked={accepted} disabled={busy} onChange={(event) => setAccepted(event.target.checked)} /><span>{quote.plan.interval === 'free' ? 'I agree to activate this Free plan.' : `I approve the first charge and recurring ${appPrice(quote.plan.amountMinor, quote.plan.currency)} monthly app fee until I cancel.`}</span></label><div className="billing-actions"><button className="secondary-button" disabled={busy} onClick={() => { setQuote(null); setAccepted(false); }}>Back</button><button className="primary-button" disabled={busy || !accepted} onClick={() => void approve()}>{busy && <LoaderCircle className="spin" size={14} />}{quote.plan.interval === 'free' ? 'Activate Free plan' : 'Approve subscription'}</button></div></div>}
  </section>;
}
