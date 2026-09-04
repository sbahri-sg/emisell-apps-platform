'use client';

import { useEffect, useMemo, useState } from 'react';
import {
  AlertCircle,
  Building2,
  Check,
  ChevronDown,
  ChevronRight,
  ClipboardCheck,
  Copy,
  KeyRound,
  LoaderCircle,
  MailCheck,
  Plus,
  RefreshCw,
  Search,
  ShieldCheck,
  UserRoundCheck,
  X,
} from 'lucide-react';
import type { DeveloperApplication, DeveloperAppType } from '../../lib/app-platform/domain';
import { apiErrorMessage, appPlatformClient } from '../../lib/app-platform/client';
import { ConfirmDialog, InlineNotice, Panel, StatusBadge } from '../components/ui';

type Notify = (message: string, tone?: 'success' | 'info' | 'warning') => void;

function statusLabel(status: string) {
  return status.split('_').map((part) => part.charAt(0).toUpperCase() + part.slice(1)).join(' ');
}

function titleCase(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat('en', { day: 'numeric', month: 'short', year: 'numeric' }).format(new Date(value));
}

export function DeveloperRequestsView({ notify }: { notify: Notify }) {
  const [applications, setApplications] = useState<DeveloperApplication[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState('');
  const [status, setStatus] = useState('All');
  const [createOpen, setCreateOpen] = useState(false);
  const [selected, setSelected] = useState<DeveloperApplication | null>(null);
  const [pendingApproval, setPendingApproval] = useState<DeveloperApplication | null>(null);
  const [saving, setSaving] = useState<string | null>(null);
  const [oneTimeInvite, setOneTimeInvite] = useState<{ application: DeveloperApplication; token: string } | null>(null);

  const load = async (signal?: AbortSignal) => {
    setLoading(true);
    setError(null);
    try {
      const response = await appPlatformClient.listDeveloperApplications({}, signal);
      setApplications(response.data);
      setSelected((current) => current ? response.data.find((item) => item.id === current.id) ?? null : null);
    } catch (loadError) {
      if (loadError instanceof DOMException && loadError.name === 'AbortError') return;
      setError(apiErrorMessage(loadError));
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  };

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient.listDeveloperApplications({}, controller.signal)
      .then((response) => setApplications(response.data))
      .catch((loadError) => {
        if (!(loadError instanceof DOMException && loadError.name === 'AbortError')) setError(apiErrorMessage(loadError));
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, []);

  const replace = (application: DeveloperApplication) => {
    setApplications((current) => current.map((item) => item.id === application.id ? application : item));
    setSelected((current) => current?.id === application.id ? application : current);
  };

  const startReview = async (application: DeveloperApplication) => {
    setSaving(application.id);
    try {
      const updated = await appPlatformClient.reviewDeveloperApplication(application.id, { revision: application.revision });
      replace(updated);
      notify(`${application.companyName} moved to review.`);
    } catch (actionError) {
      notify(apiErrorMessage(actionError), 'warning');
    } finally {
      setSaving(null);
    }
  };

  const approve = async () => {
    const application = pendingApproval;
    if (!application) return;
    setSaving(application.id);
    try {
      const result = await appPlatformClient.approveDeveloperApplication(application.id, { revision: application.revision, maxApps: 3, maxWebhooks: 20 });
      replace(result.application);
      setPendingApproval(null);
      setSelected(null);
      setOneTimeInvite({ application: result.application, token: result.invitationToken });
      notify(`${application.companyName} approved with sandbox access.`);
    } catch (actionError) {
      notify(apiErrorMessage(actionError), 'warning');
    } finally {
      setSaving(null);
    }
  };

  const reject = async (application: DeveloperApplication, notes: string) => {
    setSaving(application.id);
    try {
      const updated = await appPlatformClient.rejectDeveloperApplication(application.id, { revision: application.revision, notes });
      replace(updated);
      setSelected(null);
      notify(`${application.companyName} request rejected.`, 'info');
    } catch (actionError) {
      notify(apiErrorMessage(actionError), 'warning');
    } finally {
      setSaving(null);
    }
  };

  const rotate = async (application: DeveloperApplication) => {
    setSaving(application.id);
    try {
      const result = await appPlatformClient.rotateDeveloperInvitation(application.id, application.revision);
      replace(result.application);
      setSelected(null);
      setOneTimeInvite({ application: result.application, token: result.invitationToken });
      notify('Previous invite revoked and a new one-time invite created.');
    } catch (actionError) {
      notify(apiErrorMessage(actionError), 'warning');
    } finally {
      setSaving(null);
    }
  };

  const revoke = async (application: DeveloperApplication) => {
    if (!application.currentInvitation) return;
    setSaving(application.id);
    try {
      await appPlatformClient.revokeDeveloperInvitation(application.currentInvitation.id);
      await load();
      setSelected(null);
      notify('Pending invitation revoked.', 'info');
    } catch (actionError) {
      notify(apiErrorMessage(actionError), 'warning');
    } finally {
      setSaving(null);
    }
  };

  const filtered = useMemo(() => applications.filter((application) => {
    const search = `${application.companyName} ${application.companyDomain} ${application.contactName} ${application.contactEmail} ${application.requestedAppName}`.toLowerCase();
    return search.includes(query.toLowerCase()) && (status === 'All' || statusLabel(application.status) === status);
  }), [applications, query, status]);

  const counts = useMemo(() => ({
    submitted: applications.filter((item) => item.status === 'submitted').length,
    review: applications.filter((item) => item.status === 'under_review').length,
    invited: applications.filter((item) => item.status === 'invited' || item.status === 'approved').length,
    active: applications.filter((item) => item.status === 'active').length,
  }), [applications]);

  return <main className="page-content developer-requests-page">
    <section className="page-heading">
      <div><p className="eyebrow">Emisell internal</p><h1>Developer requests</h1><p>Review selected partners, issue one-time invitations, and grant sandbox access.</p></div>
      <button className="primary-button" onClick={() => setCreateOpen(true)}><Plus size={16} />New request</button>
    </section>

    <InlineNotice title="Invite-only program">There is no public signup. Emisell records each candidate, reviews the use case and requested scopes, then issues a time-limited owner invitation.</InlineNotice>

    <div className="summary-grid developer-summary">
      <DeveloperMetric icon={ClipboardCheck} label="Submitted" value={counts.submitted} note="Awaiting triage" />
      <DeveloperMetric icon={ShieldCheck} label="In review" value={counts.review} note="Security and scope review" tone="blue" />
      <DeveloperMetric icon={MailCheck} label="Invited" value={counts.invited} note="Waiting for acceptance" tone="amber" />
      <DeveloperMetric icon={UserRoundCheck} label="Active" value={counts.active} note="Sandbox enabled" tone="green" />
    </div>

    <section className="apps-card developer-applications-card">
      <div className="table-toolbar">
        <label className="search-field"><Search size={17} /><input aria-label="Search developer requests" placeholder="Search company, contact, or app" value={query} onChange={(event) => setQuery(event.target.value)} /></label>
        <div className="filters"><label className="filter-select"><span>{status}</span><select aria-label="Filter developer requests" value={status} onChange={(event) => setStatus(event.target.value)}><option>All</option><option>Submitted</option><option>Under Review</option><option>Approved</option><option>Invited</option><option>Active</option><option>Rejected</option></select><ChevronDown size={13} /></label></div>
      </div>
      <div className="table-wrap"><table><thead><tr><th>Company</th><th>Requested app</th><th>Contact</th><th>Status</th><th>Updated</th><th aria-label="Open" /></tr></thead><tbody>
        {!loading && !error ? filtered.map((application) => <tr key={application.id} onClick={() => setSelected(application)} tabIndex={0} onKeyDown={(event) => event.key === 'Enter' && setSelected(application)}>
          <td><div className="developer-company"><span>{application.companyName.slice(0, 2).toUpperCase()}</span><div><strong>{application.companyName}</strong><small>{application.companyDomain}</small></div></div></td>
          <td><div className="developer-app-request"><strong>{application.requestedAppName}</strong><small>{titleCase(application.appType)} extension</small></div></td>
          <td><div className="developer-contact"><strong>{application.contactName}</strong><small>{application.contactEmail}</small></div></td>
          <td><StatusBadge status={statusLabel(application.status)} /></td><td className="updated">{formatDate(application.updatedAt)}</td><td><ChevronRight className="row-arrow" size={16} /></td>
        </tr>) : null}
      </tbody></table>
      {loading ? <div className="empty-state"><LoaderCircle className="spin" size={22} /><strong>Loading developer requests</strong><p>Reading the internal review queue.</p></div> : error ? <div className="empty-state"><AlertCircle size={22} /><strong>Could not load requests</strong><p>{error}</p><button className="secondary-button empty-action" onClick={() => void load()}><RefreshCw size={14} />Retry</button></div> : filtered.length === 0 ? <div className="empty-state"><ClipboardCheck size={22} /><strong>{applications.length ? 'No matching requests' : 'No developer requests yet'}</strong><p>Use New request when Emisell selects a developer candidate.</p></div> : null}
      </div><footer className="table-footer"><span>{filtered.length} requests</span><span>Platform operator access only</span></footer>
    </section>

    {createOpen ? <CreateDeveloperRequestModal onClose={() => setCreateOpen(false)} onCreated={(application) => { setApplications((current) => [application, ...current]); setCreateOpen(false); notify(`${application.companyName} added to the review queue.`); }} /> : null}
    {selected ? <DeveloperRequestModal application={selected} saving={saving === selected.id} onClose={() => setSelected(null)} onReview={startReview} onApprove={(application) => setPendingApproval(application)} onReject={reject} onRotate={rotate} onRevoke={revoke} /> : null}
    {oneTimeInvite ? <OneTimeInvitationModal application={oneTimeInvite.application} token={oneTimeInvite.token} onClose={() => setOneTimeInvite(null)} /> : null}
    <ConfirmDialog open={Boolean(pendingApproval)} eyebrow="Create developer workspace" title={`Approve ${pendingApproval?.companyName ?? 'developer'}?`} description="This creates a new organization, enables sandbox access, and generates a one-time owner invitation. Production access remains disabled." confirmLabel="Approve & create invite" loading={Boolean(pendingApproval && saving === pendingApproval.id)} onClose={() => setPendingApproval(null)} onConfirm={() => void approve()} />
  </main>;
}

function DeveloperMetric({ icon: Icon, label, value, note, tone = 'violet' }: { icon: typeof ClipboardCheck; label: string; value: number; note: string; tone?: string }) {
  return <Panel className="summary-card"><span className={`metric-icon ${tone}`}><Icon size={17} /></span><p>{label}</p><strong>{value}</strong><small>{note}</small></Panel>;
}

function CreateDeveloperRequestModal({ onClose, onCreated }: { onClose: () => void; onCreated: (application: DeveloperApplication) => void }) {
  const [companyName, setCompanyName] = useState('');
  const [companyDomain, setCompanyDomain] = useState('');
  const [contactName, setContactName] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [requestedAppName, setRequestedAppName] = useState('');
  const [appType, setAppType] = useState<DeveloperAppType>('custom');
  const [useCase, setUseCase] = useState('');
  const [requestedScopes, setRequestedScopes] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const submit = async () => {
    setSaving(true); setError('');
    try {
      const application = await appPlatformClient.createDeveloperApplication({
        companyName, companyDomain, contactName, contactEmail, requestedAppName, appType, useCase,
        requestedScopes: requestedScopes.split(',').map((scope) => scope.trim()).filter(Boolean),
      });
      onCreated(application);
    } catch (submitError) {
      setError(apiErrorMessage(submitError));
    } finally {
      setSaving(false);
    }
  };
  return <div className="modal-backdrop" role="presentation" onMouseDown={() => { if (!saving) onClose(); }}><section className="modal developer-request-modal" role="dialog" aria-modal="true" aria-labelledby="developer-request-title" onMouseDown={(event) => event.stopPropagation()}>
    <div className="modal-head"><div><p className="section-kicker">Internal intake</p><h2 id="developer-request-title">Add developer candidate</h2><p>Record a partner Emisell has already selected or contacted.</p></div><button className="icon-button" aria-label="Close" onClick={onClose} disabled={saving}><X size={17} /></button></div>
    <div className="modal-body developer-form">
      <div className="modal-grid"><label>Company name<input autoFocus value={companyName} onChange={(event) => setCompanyName(event.target.value)} placeholder="PT Example Teknologi" /></label><label>Company domain<input value={companyDomain} onChange={(event) => setCompanyDomain(event.target.value)} placeholder="example.co.id" /></label></div>
      <div className="modal-grid"><label>Contact name<input value={contactName} onChange={(event) => setContactName(event.target.value)} placeholder="Full name" /></label><label>Business email<input type="email" value={contactEmail} onChange={(event) => setContactEmail(event.target.value)} placeholder="name@example.co.id" /></label></div>
      <div className="modal-grid"><label>Requested app<input value={requestedAppName} onChange={(event) => setRequestedAppName(event.target.value)} placeholder="Example Connect" /></label><label>App type<select value={appType} onChange={(event) => setAppType(event.target.value as DeveloperAppType)}><option value="custom">Custom</option><option value="payment">Payment</option><option value="shipping">Shipping</option><option value="erp">ERP</option><option value="marketing">Marketing</option></select></label></div>
      <label>Use case<textarea rows={4} value={useCase} onChange={(event) => setUseCase(event.target.value)} placeholder="What will the integration do, and which merchants will use it?" /></label>
      <label>Requested scopes <small className="optional-label">Comma separated</small><input value={requestedScopes} onChange={(event) => setRequestedScopes(event.target.value)} placeholder="read_orders, write_fulfillments" /></label>
      {error ? <InlineNotice tone="warning" title="Could not add request">{error}</InlineNotice> : <InlineNotice title="Manual verification">Confirm company identity and contact ownership outside the platform before approval.</InlineNotice>}
    </div>
    <div className="modal-foot"><button className="secondary-button" onClick={onClose} disabled={saving}>Cancel</button><button className="primary-button" onClick={() => void submit()} disabled={saving}>{saving ? <LoaderCircle className="spin" size={15} /> : <Plus size={15} />}{saving ? 'Adding…' : 'Add to review queue'}</button></div>
  </section></div>;
}

function DeveloperRequestModal({ application, saving, onClose, onReview, onApprove, onReject, onRotate, onRevoke }: {
  application: DeveloperApplication; saving: boolean; onClose: () => void;
  onReview: (application: DeveloperApplication) => Promise<void>; onApprove: (application: DeveloperApplication) => void;
  onReject: (application: DeveloperApplication, notes: string) => Promise<void>; onRotate: (application: DeveloperApplication) => Promise<void>;
  onRevoke: (application: DeveloperApplication) => Promise<void>;
}) {
  const [notes, setNotes] = useState(application.reviewNotes ?? '');
  const [showReject, setShowReject] = useState(false);
  const canInvite = application.status === 'approved' || application.status === 'invited';
  return <div className="modal-backdrop" role="presentation" onMouseDown={() => { if (!saving) onClose(); }}><section className="modal developer-detail-modal" role="dialog" aria-modal="true" aria-labelledby="developer-detail-title" onMouseDown={(event) => event.stopPropagation()}>
    <div className="modal-head"><div><p className="section-kicker">{application.companyDomain}</p><h2 id="developer-detail-title">{application.companyName}</h2><p>{application.requestedAppName} · {titleCase(application.appType)} extension</p></div><button className="icon-button" aria-label="Close" onClick={onClose} disabled={saving}><X size={17} /></button></div>
    <div className="modal-body developer-detail-body">
      <div className="developer-detail-status"><StatusBadge status={statusLabel(application.status)} /><span>Revision {application.revision}</span><span>Submitted {formatDate(application.createdAt)}</span></div>
      <div className="developer-detail-grid"><div><small>Contact</small><strong>{application.contactName}</strong><span>{application.contactEmail}</span></div><div><small>Organization</small><strong>{application.organizationId ? 'Workspace created' : 'Not created'}</strong><span>{application.organizationId ?? 'Created after approval'}</span></div></div>
      <div className="developer-use-case"><small>Use case</small><p>{application.useCase}</p></div>
      <div className="developer-scope-list"><small>Requested scopes</small><div>{application.requestedScopes.length ? application.requestedScopes.map((scope) => <code key={scope}>{scope}</code>) : <span>No scopes requested</span>}</div></div>
      {application.currentInvitation ? <div className="invitation-state"><MailCheck size={16} /><div><strong>Owner invitation · {statusLabel(application.currentInvitation.status)}</strong><span>{application.currentInvitation.email} · expires {formatDate(application.currentInvitation.expiresAt)}</span></div></div> : null}
      {showReject ? <label>Rejection notes<textarea autoFocus rows={3} value={notes} onChange={(event) => setNotes(event.target.value)} placeholder="Explain the decision for the audit trail." /></label> : null}
      {application.status === 'active' ? <InlineNotice title="Sandbox access active">The developer accepted the invitation. Production access remains disabled until a separate launch review.</InlineNotice> : null}
    </div>
    <div className="modal-foot developer-actions">
      {application.status === 'submitted' ? <button className="secondary-button" onClick={() => void onReview(application)} disabled={saving}>{saving ? <LoaderCircle className="spin" size={14} /> : <ClipboardCheck size={14} />}Start review</button> : null}
      {application.status === 'under_review' && !showReject ? <button className="danger-text-button" onClick={() => setShowReject(true)} disabled={saving}>Reject</button> : null}
      {application.status === 'under_review' && showReject ? <button className="danger-button" onClick={() => void onReject(application, notes)} disabled={saving || notes.trim().length < 5}>{saving ? <LoaderCircle className="spin" size={14} /> : null}Confirm rejection</button> : null}
      {canInvite && application.currentInvitation?.status === 'pending' ? <button className="danger-text-button" onClick={() => void onRevoke(application)} disabled={saving}>Revoke invite</button> : null}
      <span className="action-spacer" />
      <button className="secondary-button" onClick={onClose} disabled={saving}>Close</button>
      {application.status === 'under_review' ? <button className="primary-button" onClick={() => onApprove(application)} disabled={saving}><ShieldCheck size={15} />Approve</button> : null}
      {canInvite ? <button className="primary-button" onClick={() => void onRotate(application)} disabled={saving}>{saving ? <LoaderCircle className="spin" size={14} /> : <RefreshCw size={14} />}{application.currentInvitation ? 'Create new invite' : 'Create invite'}</button> : null}
    </div>
  </section></div>;
}

function OneTimeInvitationModal({ application, token, onClose }: { application: DeveloperApplication; token: string; onClose: () => void }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    await navigator.clipboard.writeText(token);
    setCopied(true);
  };
  return <div className="modal-backdrop" role="presentation"><section className="modal compact-modal secret-modal" role="dialog" aria-modal="true" aria-labelledby="invitation-token-title">
    <div className="modal-head"><div><p className="section-kicker">Shown once</p><h2 id="invitation-token-title">Developer invitation ready</h2><p>Send this code to {application.contactName} through a verified secure channel.</p></div></div>
    <div className="modal-body"><div className="invite-recipient"><Building2 size={16} /><span><strong>{application.companyName}</strong><small>{application.contactEmail}</small></span></div><div className="secret-field"><small>One-time invitation code</small><code>{token}</code><button className="icon-button" onClick={() => void copy()} aria-label="Copy invitation code">{copied ? <Check size={15} /> : <Copy size={15} />}</button></div><InlineNotice tone="warning" title="Raw code is not stored">After this dialog closes, Emisell can only revoke the invitation and generate a new code. Production access is still disabled.</InlineNotice></div>
    <div className="modal-foot"><button className="primary-button" onClick={onClose}><KeyRound size={15} />I saved the code</button></div>
  </section></div>;
}
