'use client';

import { useState } from 'react';
import { ArrowRight, CheckCircle2, KeyRound, LoaderCircle, ShieldCheck } from 'lucide-react';
import type { InvitationAcceptanceResult } from '../../lib/app-platform/contracts';
import { apiErrorMessage, appPlatformClient } from '../../lib/app-platform/client';
import { InlineNotice } from '../components/ui';

export default function AcceptInvitationPage() {
  const [token, setToken] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState<InvitationAcceptanceResult | null>(null);
  const accept = async () => {
    setSaving(true); setError('');
    try {
      setResult(await appPlatformClient.acceptDeveloperInvitation(token));
    } catch (acceptError) {
      setError(apiErrorMessage(acceptError));
    } finally {
      setSaving(false);
    }
  };
  return <main className="invitation-page"><section className="invitation-shell">
    <div className="invitation-brand"><span>E</span><div><strong>Emisell</strong><small>App Platform</small></div></div>
    {result ? <div className="invitation-success"><span><CheckCircle2 size={24} /></span><p className="eyebrow">Invitation accepted</p><h1>Your developer workspace is ready</h1><p>{result.application.companyName} now has sandbox access for <strong>{result.application.requestedAppName}</strong>. Sign in again or switch organization to load the new membership.</p><div className="entitlement-card"><div><small>Environment</small><strong>Sandbox</strong></div><div><small>App limit</small><strong>{result.entitlement.maxApps}</strong></div><div><small>Production</small><strong>Separate review</strong></div></div><a className="primary-button invitation-link" href="/overview">Return to Emisell <ArrowRight size={15} /></a></div> : <>
      <div className="invitation-icon"><KeyRound size={21} /></div><p className="eyebrow">Private developer program</p><h1>Accept your Emisell invitation</h1><p className="invitation-copy">Sign in with the same verified business email Emisell invited, then enter the one-time code you received.</p>
      <label className="invitation-token-field">Invitation code<input autoFocus value={token} onChange={(event) => { setToken(event.target.value); setError(''); }} placeholder="emi_inv_…" disabled={saving} /></label>
      {error ? <InlineNotice tone="warning" title="Invitation could not be accepted">{error}</InlineNotice> : <InlineNotice title="Identity-bound acceptance">The code only works for the invited email and expires automatically. Emisell never enables production access through this step.</InlineNotice>}
      <button className="primary-button invitation-submit" onClick={() => void accept()} disabled={saving || token.trim().length < 32}>{saving ? <LoaderCircle className="spin" size={16} /> : <ShieldCheck size={16} />}{saving ? 'Verifying…' : 'Accept invitation'}</button>
    </>}
    <footer>Invite-only · Identity verified · Sandbox first</footer>
  </section></main>;
}
