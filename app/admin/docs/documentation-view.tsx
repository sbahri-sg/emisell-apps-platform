'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { ArrowRight, BookOpen, Check, ChevronDown, Code2, Copy, Download, FileJson, KeyRound, Layers3, LoaderCircle, Search, ShieldCheck } from 'lucide-react';
import { apiErrorMessage, appPlatformClient } from '../../../lib/app-platform/client';
import type { ContractId, DocumentationFlow, DocumentationGatewayIntegration, DocumentationOperation, DocumentationReference } from '../../../lib/documentation/types';
import { InlineNotice, Panel } from '../../components/ui';
import './documentation.css';

const views = [
  { id: 'reference', label: 'API contracts' },
  { id: 'gateway', label: 'Gateway integration' },
  { id: 'guide', label: 'Operator guide' },
  { id: 'authentication', label: 'Authentication' },
  { id: 'troubleshooting', label: 'Troubleshooting' },
] as const;
type DocsView = typeof views[number]['id'];
const contractIds: ContractId[] = ['admin', 'developer', 'provider', 'emisell', 'identity', 'resource', 'resource-blueprint', 'managed', 'shipping-provider'];
const primaryContractIds: ContractId[] = ['emisell', 'provider'];

export function DocumentationView() {
  const router = useRouter();
  const params = useSearchParams();
  const requestedContract = contractIds.includes(params.get('contract') as ContractId) ? params.get('contract') as ContractId : 'emisell';
  const advancedMode = params.get('mode') === 'advanced';
  const view = views.some((item) => item.id === params.get('view')) ? params.get('view') as DocsView : 'reference';
  const contract = view === 'gateway' ? 'emisell' : primaryContractIds.includes(requestedContract) || advancedMode ? requestedContract : 'emisell';
  const endpoint = params.get('endpoint');
  const [reference, setReference] = useState<DocumentationReference | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const [search, setSearch] = useState('');
  const [method, setMethod] = useState('all');
  const [download, setDownload] = useState<string | null>(null);
  const [notice, setNotice] = useState('');

  useEffect(() => {
    if (requestedContract === contract) return;
    const query = new URLSearchParams({ contract, view });
    router.replace(`/admin/docs?${query}`, { scroll: false });
  }, [contract, requestedContract, router, view]);

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient.getAdminDocumentation(contract, controller.signal)
      .then((result) => { if (!controller.signal.aborted) { setReference(result); setError(null); } })
      .catch((reason) => { if (!controller.signal.aborted) setError(apiErrorMessage(reason)); });
    return () => controller.abort();
  }, [contract, retry]);

  const loaded = reference?.selected.id === contract;
  const filtered = useMemo(() => (reference?.operations ?? []).filter((operation) => {
    const query = search.trim().toLowerCase();
    return (method === 'all' || operation.method === method) &&
      `${operation.method} ${operation.path} ${operation.summary} ${operation.tag}`.toLowerCase().includes(query);
  }), [reference, search, method]);
  const filteredGroups = useMemo(() => {
    if (!reference) return [];
    const byId = new Map(filtered.map((operation) => [operation.id, operation]));
    return reference.flow.groups.map((group) => ({ ...group,
      operations: group.operationIds.map((operationId) => byId.get(operationId)).filter((operation): operation is DocumentationOperation => Boolean(operation)),
    })).filter((group) => group.operations.length > 0);
  }, [reference, filtered]);
  const authentication = useMemo(() => [...new Set(reference?.operations.flatMap((operation) => operation.authentication) ?? [])], [reference]);

  function navigate(nextContract: ContractId, nextView: DocsView, operationId?: string) {
    setSearch(''); setMethod('all'); setError(null); setNotice('');
    const query = new URLSearchParams({ contract: nextContract, view: nextView });
    if (!primaryContractIds.includes(nextContract)) query.set('mode', 'advanced');
    if (operationId) query.set('endpoint', operationId);
    router.push(`/admin/docs?${query}${operationId ? `#${operationId}` : ''}`, { scroll: false });
  }

  async function downloadContract(format: 'openapi' | 'postman') {
    setDownload(format); setNotice('');
    try {
      const artifact = await appPlatformClient.getAdminDocumentationDownload(contract, format);
      const blob = new Blob([JSON.stringify(artifact, null, 2)], { type: 'application/json' });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `emisell-${contract}.${format === 'postman' ? 'postman_collection' : 'openapi'}.json`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      setNotice(`${format === 'postman' ? 'Postman collection' : 'OpenAPI'} siap diunduh. Secret tidak disertakan.`);
    } catch (reason) { setNotice(apiErrorMessage(reason)); }
    finally { setDownload(null); }
  }

  return <main className="admin-content docs-page">
    <div className="admin-heading docs-heading">
      <div><p>Platform / Documentation</p><h1>Emisell integration contracts</h1><span>Dua kontrak utama: gateway internal Emisell dan API untuk backend partner.</span></div>
      <button className="secondary-button docs-heading-download" disabled={!loaded || Boolean(error) || download !== null} onClick={() => downloadContract('postman')}>{download === 'postman' ? <LoaderCircle className="spin" size={15} /> : <Download size={15} />}Download Postman</button>
    </div>
    <nav className="docs-view-nav" aria-label="Documentation sections">
      {views.map((item) => <button key={item.id} aria-current={view === item.id ? 'page' : undefined} onClick={() => navigate(item.id === 'gateway' ? 'emisell' : contract, item.id)}>{item.label}</button>)}
    </nav>
    {error ? <div className="docs-load-error" role="alert"><InlineNotice tone="warning" title="Dokumentasi tidak dapat dimuat">{error}</InlineNotice><button className="secondary-button" onClick={() => { setError(null); setRetry((value) => value + 1); }}>Coba lagi</button></div> : !loaded ? <div className="docs-loading" role="status"><LoaderCircle className="spin" size={20} /> Memverifikasi akses dan memuat kontrak…</div> : null}
    {!error && loaded && reference ? <>
      {view === 'guide' ? <OperatorGuide onReference={(id, operationId) => navigate(id, 'reference', operationId)} /> : null}
      {view === 'gateway' ? <GatewayIntegrationGuide integrations={reference.gatewayIntegrations} onReference={(operationId) => navigate('emisell', 'reference', operationId)} /> : null}
      {view === 'authentication' ? <AuthenticationGuide /> : null}
      {view === 'troubleshooting' ? <TroubleshootingGuide /> : null}
      {view === 'reference' ? <>
        <section className="docs-primary-boundary" aria-labelledby="primary-contracts-title">
          <div><p className="docs-label">Canonical API surface</p><h2 id="primary-contracts-title">Mulai dari dua kontrak utama</h2></div>
          <p>Route dashboard, login, runtime internal, dan rancangan yang belum aktif tidak dicampur ke jalur integrasi utama. Payment Proxy dan Shipping Gateway tetap menjadi runtime extension terpisah.</p>
        </section>
        <section className="docs-contract-picker" aria-label="Choose API contract">
          {reference.contracts.filter((entry) => entry.visibility === 'primary').map((entry) => <button key={entry.id} aria-current={contract === entry.id ? 'page' : undefined} onClick={() => navigate(entry.id, 'reference')}>
            <span className="docs-picker-meta"><span>{entry.planned ? 'Planned' : entry.status}</span>{contract === entry.id ? <Check size={14} aria-hidden="true" /> : null}</span>
            <strong>{entry.title}</strong><small>{entry.audience}</small>
          </button>)}
        </section>
        <details className="docs-advanced-contracts" open={advancedMode || undefined}>
          <summary><span><strong>Operator & roadmap references</strong><small>Route pendukung dashboard dan desain yang belum menjadi API integrasi</small></span><ChevronDown size={16} /></summary>
          <div>
            <section aria-labelledby="operator-contracts-title"><h3 id="operator-contracts-title">Operator only</h3><p>Dipakai oleh dashboard atau runtime internal. Partner dan Backend Emisell tidak perlu mengintegrasikannya.</p><div className="docs-advanced-grid">{reference.contracts.filter((entry) => entry.visibility === 'operator').map((entry) => <button key={entry.id} aria-current={contract === entry.id ? 'page' : undefined} onClick={() => navigate(entry.id, 'reference')}><span>{entry.status}</span><strong>{entry.title}</strong><small>{entry.count} endpoints · {entry.audience}</small></button>)}</div></section>
            <section aria-labelledby="roadmap-contracts-title"><h3 id="roadmap-contracts-title">Pilot & roadmap</h3><p>Untuk evaluasi dan handoff implementasi. Tidak boleh dianggap tersedia untuk partner.</p><div className="docs-advanced-grid">{reference.contracts.filter((entry) => entry.visibility === 'roadmap').map((entry) => <button key={entry.id} aria-current={contract === entry.id ? 'page' : undefined} onClick={() => navigate(entry.id, 'reference')}><span>{entry.planned ? 'Planned' : entry.status}</span><strong>{entry.title}</strong><small>{entry.count} endpoints · {entry.audience}</small></button>)}</div></section>
          </div>
        </details>
        <dl className="docs-contract-metrics" aria-label="Selected API contract summary">
          <div><dt>Status</dt><dd>{reference.selected.planned ? 'Planned' : reference.flow.status}<small>v{reference.selected.version}</small></dd></div>
          <div><dt>Audience</dt><dd>{reference.selected.audience}</dd></div>
          <div><dt>Route families</dt><dd>{reference.flow.routeFamilies.map((path) => <code key={path}>{path}</code>)}</dd></div>
          <div><dt>Endpoints</dt><dd>{reference.selected.count}<small>documented</small></dd></div>
        </dl>
        <section className="docs-contract-intro" aria-label={reference.selected.title}>
          <div><p className="docs-label">Selected contract</p><h2>{reference.selected.title}</h2><p>{reference.selected.description}</p></div>
          <ol className="docs-flow-line" aria-label={`Alur ${reference.selected.title}`}>{reference.flow.actors.map((actor, index) => <li key={actor}><span>{actor}</span>{index < reference.flow.actors.length - 1 ? <ArrowRight size={15} aria-hidden="true" /> : null}</li>)}</ol>
        </section>
        <div className="docs-workspace">
        <aside className="docs-contracts" aria-label="Contract navigation">
          <nav className="docs-contents" aria-label="Contents for selected contract">
            <p className="docs-label">Contents</p>
            <a href="#integration-flow">Alur integrasi</a>
            <a href="#contract-decisions">Batas kontrak</a>
            {reference.handoff ? <a href="#backend-handoff">Backend blueprint & SDK</a> : null}
            <a href="#contract-authentication">Authentication</a>
            <a href="#contract-downloads">Postman & OpenAPI</a>
            <a href="#endpoint-reference">Endpoint reference</a>
            {filteredGroups.map((group) => <a key={group.id} href={`#${group.id}`}><span>{group.title}</span><small>{group.operations.length}</small></a>)}
          </nav>
          <p className="docs-contract-hint"><FileJson size={16} /> Endpoint dan contoh selalu mengikuti OpenAPI.</p>
        </aside>
        <div className="docs-reference">
          {reference.selected.planned ? <InlineNotice tone="warning" title="Belum tersedia untuk integrasi">Ini target desain, bukan API berjalan. Koleksi Postman diberi pengaman untuk menolak eksekusi. Jangan memasukkan endpoint ini ke integrasi provider.</InlineNotice> : null}
          {reference.selected.pilot ? <InlineNotice tone="warning" title="Kode pilot teruji; deployment belum diverifikasi">Jalur produk telah diuji dengan PostgreSQL terisolasi; ini belum membuktikan koneksi deployment. Kedua service memerlukan key khusus dan daftar merchant uji. Harga dan stok adalah nilai produk dasar, bukan harga variant atau stok siap jual. Scope read_products tetap planned untuk publikasi App Store. Postman melewati request pilot kecuali environment privat menetapkan enable_resource_pilot=true. Panduan: docs/resource-pilot.md.</InlineNotice> : null}
          {['resource', 'resource-blueprint'].includes(contract) ? <section className="docs-resource-links" aria-label="Resource contract stages"><div><strong>Pilot dan kontrak target dipisahkan</strong><p>Produk dasar: {reference.contracts.find((entry) => entry.id === 'resource')?.count} endpoint pilot. Blueprint: {reference.contracts.find((entry) => entry.id === 'resource-blueprint')?.count} endpoint baca untuk dikerjakan backend, belum aktif.</p></div><button className="secondary-button" onClick={() => navigate(contract === 'resource' ? 'resource-blueprint' : 'resource', 'reference')}>{contract === 'resource' ? 'Buka backend blueprint' : 'Lihat product pilot'}<ArrowRight size={14} /></button></section> : null}
          {reference.selected.id === 'managed' ? <InlineNotice tone="warning" title="Credential API nonaktif secara default">Hanya untuk runtime internal Emisell dengan token terikat ke satu instalasi–extension. Ini belum menjalankan Payment atau Shipping. Provision dan rotasi ada di kontrak Admin; app eksternal tetap menggunakan OAuth. Panduan: docs/managed-extensions.md.</InlineNotice> : null}
          {['developer', 'emisell', 'provider'].includes(reference.selected.id) ? <InlineNotice title="App billing: implementasi teruji, belum aktif live">Paket, persetujuan langganan, dan rincian tagihan tersedia di modul default-off. Integrasi cron/callback billing lama belum diaktifkan. Postman melewati request billing kecuali enable_app_billing=true pada environment privat. Panduan: docs/app-billing.md.</InlineNotice> : null}
          <ContractFlow flow={reference.flow} onSelect={(operationId) => navigate(contract, 'reference', operationId)} />
          {reference.handoff ? <BackendHandoff handoff={reference.handoff} /> : null}
          <section className="docs-article" id="contract-authentication"><p className="docs-label">Authentication</p><h2>Credential sesuai pemanggil</h2><p>Sesi Admin, sesi Merchant, token instalasi, dan service assertion tidak dapat dipakai silang. Ikuti pilihan authentication pada endpoint yang akan dipanggil.</p><ul className="docs-contract-auth">{authentication.map((entry) => <li key={entry}><KeyRound size={17} /><span>{entry}</span></li>)}</ul><button className="docs-text-button" onClick={() => navigate(contract, 'authentication')}>Panduan authentication lengkap <ArrowRight size={14} /></button></section>
          <section className="docs-article" id="contract-downloads"><p className="docs-label">Postman & OpenAPI</p><h2>Collection untuk kontrak ini</h2><p>Unduh kontrak yang dipilih. Isi credential melalui environment privat atau vault, lalu tinjau setiap request sebelum menjalankannya.</p><div className="docs-download-card"><FileJson size={25} /><div><strong>{reference.selected.title}</strong><small>v{reference.selected.version} · {reference.selected.count} operations</small></div><button className="secondary-button" disabled={download !== null} onClick={() => downloadContract('postman')}>{download === 'postman' ? <LoaderCircle className="spin" size={14} /> : <Download size={14} />}Postman</button><button className="secondary-button" disabled={download !== null} onClick={() => downloadContract('openapi')}>{download === 'openapi' ? <LoaderCircle className="spin" size={14} /> : <FileJson size={14} />}OpenAPI</button></div><div className="docs-source"><code>{reference.selected.source}</code><span>OpenAPI 3.1 · placeholder credentials only</span></div><p className="docs-safe-note"><ShieldCheck size={15} /> Halaman ini hanya membaca dan menyalin contoh; tidak menjalankan mutasi API.</p></section>
          <section className="docs-endpoint-search" id="endpoint-reference"><p className="docs-label">API reference</p><h2>Endpoint reference</h2>
          <div className="docs-filters"><label className="docs-search"><Search size={16} /><input aria-label="Search API endpoints" placeholder="Search endpoint, action, or resource…" value={search} onChange={(event) => setSearch(event.target.value)} /></label><select aria-label="Filter HTTP method" value={method} onChange={(event) => setMethod(event.target.value)}><option value="all">All methods</option>{[...new Set(reference.operations.map((item) => item.method))].sort().map((value) => <option key={value}>{value}</option>)}</select></div>
          <p className="docs-result-count" aria-live="polite">{filtered.length} of {reference.operations.length} operations</p></section>
          <div className="docs-endpoint-groups">{filteredGroups.map((group) => <section className="docs-endpoint-group" id={group.id} key={group.id}><header><div><p className="docs-label">Endpoint group</p><h2>{group.title}</h2>{group.description ? <p>{group.description}</p> : null}</div><span>{group.operations.length}</span></header><div className="docs-endpoints">{group.operations.map((operation) => <Endpoint key={`${contract}-${operation.id}`} operation={operation} initiallyOpen={endpoint === operation.id} onNotice={setNotice} />)}</div></section>)}</div>
          {!filtered.length ? <Panel className="docs-empty"><Search size={22} /><h3>Tidak ada endpoint yang cocok</h3><p>Coba kata lain atau ubah filter metode.</p><button className="secondary-button" onClick={() => { setSearch(''); setMethod('all'); }}>Reset filters</button></Panel> : null}
        </div>
      </div></> : null}
    </> : null}
    <p className="docs-feedback" role="status" aria-live="polite">{notice}</p>
    <footer className="docs-footer"><BookOpen size={14} /> Internal operator guide · API availability bukan persetujuan production.</footer>
  </main>;
}

function GatewayIntegrationGuide({ integrations, onReference }: { integrations: DocumentationGatewayIntegration[]; onReference: (operationId: string) => void }) {
  const integration = integrations.find((entry) => entry.id === 'shipping-rate-calculation');
  if (!integration) return <InlineNotice tone="warning" title="Kontrak gateway belum tersedia">Metadata integrasi tidak ditemukan pada OpenAPI yang terverifikasi. Jalur tidak boleh diaktifkan sampai kontrak tersedia.</InlineNotice>;
  return <div className="docs-gateway-guide">
    <section className="docs-gateway-hero">
      <div><p className="docs-label">Internal gateway map</p><h2>{integration.title}</h2><p>{integration.summary}</p></div>
      <span>Pilot · default off</span>
    </section>
    <ol className="docs-gateway-sequence" aria-label="Shipping rate gateway sequence">
      {integration.sequence.map((step, index) => <li key={step.actor}><div><span>{String(index + 1).padStart(2, '0')}</span><strong>{step.actor}</strong><p>{step.action}</p></div>{index < integration.sequence.length - 1 ? <ArrowRight size={18} aria-hidden="true" /> : null}</li>)}
    </ol>
    <InlineNotice title="App Platform hanya authorization relay">App Platform memverifikasi merchant, instalasi, immutable version, dan capability. Engine kurir, credential provider, cache, rate card, quota, dan fallback tetap dimiliki API Kurir.</InlineNotice>
    <section className="docs-gateway-section" aria-labelledby="gateway-hops-title">
      <div className="docs-gateway-section-heading"><div><p className="docs-label">Transport contracts</p><h2 id="gateway-hops-title">Dua hop, dua batas kepercayaan</h2></div><button className="secondary-button" onClick={() => onReference(integration.operationId)}>Buka API contract <ArrowRight size={14} /></button></div>
      <div className="docs-gateway-hops">{integration.hops.map((hop) => <article key={hop.id}>
        <header><div><p>{hop.label}</p><h3>{hop.from} <ArrowRight size={14} /> {hop.to}</h3></div><code className={`docs-method method-${hop.method.toLowerCase()}`}>{hop.method}</code></header>
        <code className="docs-gateway-path">{hop.path}</code>
        <dl>
          <div><dt>Authentication</dt><dd>{hop.authentication}</dd></div>
          <div><dt>Identity</dt><dd>{hop.identity}</dd></div>
          <div><dt>Content type</dt><dd><code>{hop.contentType}</code></dd></div>
          <div><dt>Timeout</dt><dd>{hop.timeoutMs / 1000} seconds · no automatic retry</dd></div>
          <div><dt>Payload</dt><dd>{hop.fields.map((field) => <code key={field}>{field}</code>)}</dd></div>
          <div><dt>Headers</dt><dd>{hop.headers.map((header) => <code key={header}>{header}</code>)}</dd></div>
        </dl>
        <div className="docs-gateway-forbidden"><strong>Caller tidak boleh memilih</strong><p>{hop.forbiddenSelectors.map((selector) => <code key={selector}>{selector}</code>)}</p></div>
      </article>)}</div>
    </section>
    <section className="docs-gateway-section" aria-labelledby="gateway-ownership-title"><p className="docs-label">Ownership matrix</p><h2 id="gateway-ownership-title">Siapa menguasai keputusan</h2><div className="docs-table-wrap"><table className="docs-fields docs-gateway-ownership"><thead><tr><th>Concern</th><th>Owner</th><th>Invariant</th></tr></thead><tbody>{integration.ownership.map((item) => <tr key={item.concern}><td>{item.concern}</td><td><strong>{item.owner}</strong></td><td>{item.rule}</td></tr>)}</tbody></table></div></section>
    <div className="docs-gateway-policy-grid">
      <Panel><p className="docs-label">Failure & retry</p><h2>Fail closed</h2><ul>{integration.failurePolicy.map((item) => <li key={item}><ShieldCheck size={16} /><span>{item}</span></li>)}</ul></Panel>
      <Panel><p className="docs-label">Sandbox checklist</p><h2>Sebelum menyalakan bridge</h2><ol>{integration.sandboxChecklist.map((item, index) => <li key={item}><span>{index + 1}</span><p>{item}</p></li>)}</ol></Panel>
    </div>
    <p className="docs-gateway-source"><FileJson size={15} /> Diagram dan aturan ini dihasilkan dari metadata <code>x-emisell-gateway-integrations</code> pada OpenAPI; bukan daftar endpoint UI yang dipelihara terpisah.</p>
  </div>;
}

function ContractFlow({ flow, onSelect }: { flow: DocumentationFlow; onSelect: (operationId: string) => void }) {
  return <><section className="docs-article docs-contract-flow" id="integration-flow" aria-labelledby="integration-flow-title">
    <div className="docs-flow-heading"><div><p className="docs-label">Integration quick-start</p><h2 id="integration-flow-title">Alur integrasi</h2></div><span><Layers3 size={15} /> {flow.quickStart.length} langkah utama</span></div>
    <p className="docs-flow-outcome">{flow.outcome}</p>
    <ol className="docs-quickstart">{flow.quickStart.map((step, index) => <li key={step.id}><button onClick={() => onSelect(step.id)}><span className="docs-step-number">{String(index + 1).padStart(2, '0')}</span><span className="docs-step-copy"><strong>{step.summary}</strong><span><code className={`docs-method method-${step.method.toLowerCase()}`}>{step.method}</code><code>{step.path}</code></span></span><ArrowRight size={14} /></button></li>)}</ol>
    <p className="docs-generated-note"><FileJson size={14} /> Buka langkah untuk melihat parameter dan contoh request sesuai OpenAPI.</p>
  </section><section className="docs-article docs-guardrails" id="contract-decisions"><p className="docs-label">Contract decisions</p><h2>Batas kontrak</h2><ul>{flow.guardrails.map((guardrail) => <li key={guardrail}><ShieldCheck size={17} /><span>{guardrail}</span></li>)}</ul></section></>;
}

function CodeBlock({ text, label, onNotice }: { text: string; label: string; onNotice: (message: string) => void }) {
  const [copiedText, setCopiedText] = useState<string | null>(null);
  const timer = useRef<number | null>(null);
  const copied = copiedText === text;
  useEffect(() => () => { if (timer.current !== null) window.clearTimeout(timer.current); }, []);
  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedText(text);
      if (timer.current !== null) window.clearTimeout(timer.current);
      timer.current = window.setTimeout(() => setCopiedText(null), 2000);
      onNotice('Contoh disalin. Ganti placeholder sebelum digunakan.');
    } catch { onNotice('Clipboard tidak tersedia. Pilih dan salin contoh secara manual.'); }
  }
  return <div className="docs-code"><div><span>{label}</span><button aria-label={`Copy ${label}`} onClick={copy}>{copied ? <Check size={13} /> : <Copy size={13} />}{copied ? 'Copied' : 'Copy'}</button></div><pre tabIndex={0} aria-label={label}><code>{text}</code></pre></div>;
}

function BackendHandoff({ handoff }: { handoff: NonNullable<DocumentationReference['handoff']> }) {
  return <section className="docs-article docs-backend-handoff" id="backend-handoff">
    <p className="docs-label">Backend implementation guide</p><h2>Pola SDK Shopify, kontrak milik Emisell</h2>
    <p>{handoff.protocol}</p><p className="docs-example-note">Diperiksa {handoff.reviewedOn} · {handoff.source}</p>
    <div className="docs-table-wrap"><table className="docs-fields docs-comparison"><thead><tr><th>Pola</th><th>Referensi Shopify</th><th>Keputusan Emisell</th></tr></thead><tbody>{handoff.decisions.map((decision) => <tr key={decision.topic}><td>{decision.topic}</td><td>{decision.shopify}<small><a href={decision.source} target="_blank" rel="noopener noreferrer">Sumber resmi ↗</a></small></td><td>{decision.emisell}</td></tr>)}</tbody></table></div>
    <h3>Syarat sebelum endpoint dinyatakan siap</h3><ol className="docs-backend-checklist">{handoff.acceptance.map((item) => <li key={item}>{item}</li>)}</ol>
    <details className="docs-schema"><summary>Di luar slice baca ini</summary><ul>{handoff.deferred.map((item) => <li key={item}>{item}</li>)}</ul></details>
  </section>;
}

function Endpoint({ operation, initiallyOpen, onNotice }: { operation: DocumentationOperation; initiallyOpen: boolean; onNotice: (message: string) => void }) {
  const element = useRef<HTMLDetailsElement>(null);
  const [status, setStatus] = useState(operation.responses[0]?.status ?? '');
  useEffect(() => { if (initiallyOpen) element.current?.scrollIntoView({ block: 'start' }); }, [initiallyOpen]);
  const response = operation.responses.find((item) => item.status === status);
  return <details ref={element} className="docs-endpoint" id={operation.id} open={initiallyOpen || undefined}>
    <summary><span className={`docs-method method-${operation.method.toLowerCase()}`}>{operation.method}</span><span className="docs-endpoint-name"><code>{operation.path}</code><small>{operation.summary}</small></span><ChevronDown className="docs-chevron" size={16} /></summary>
    <div className="docs-endpoint-body"><div className="docs-operation-meta"><span>{operation.tag}</span><code>{operation.id}</code></div>
      {operation.description ? <p className="docs-description">{operation.description}</p> : null}
      {operation.backend ? <section className="docs-backend-contract" aria-label={`Backend requirements for ${operation.id}`}>
        <h3>Backend contract · planned</h3>
        <dl><div><dt>Exact scope</dt><dd>{operation.backend.scopes.map((scope) => <code key={scope}>{scope}</code>)}</dd></div><div><dt>Source models</dt><dd>{operation.backend.models.join(', ')}</dd></div><div><dt>Pagination</dt><dd>{operation.backend.pagination}</dd></div><div><dt>Provider path · belum tersedia</dt><dd><code>{operation.backend.providerTargetPath}</code></dd></div></dl>
        <p className="docs-inline-requirement"><strong>Tenant predicate:</strong> {operation.backend.tenantRule}</p>
        <details className="docs-schema"><summary>Field mapping · sumber data & maknanya</summary><div className="docs-table-wrap"><table className="docs-fields"><thead><tr><th>Response field</th><th>Sumber api-service</th><th>Aturan proyeksi</th></tr></thead><tbody>{operation.backend.fields.map((field) => <tr key={field.name}><td><code>{field.name}</code></td><td><code>{field.source}</code></td><td>{field.description}</td></tr>)}</tbody></table></div></details>
      </section> : null}
      <h3>Authentication</h3><ul className="docs-auth-options">{operation.authentication.map((entry, index) => <li key={entry}>{index > 0 ? <strong>OR · </strong> : null}{entry}</li>)}</ul>
      {operation.path.startsWith('/v1/internal/') ? <p className="docs-inline-requirement">Wajib session Admin khusus. Token Developer, session Merchant, dan role owner/admin organisasi tidak diterima.</p> : null}
      {operation.fields.length ? <><h3>Parameters & body fields</h3><div className="docs-table-wrap"><table className="docs-fields"><thead><tr><th>Field</th><th>Type</th><th>Requirements</th></tr></thead><tbody>{operation.fields.map((field) => <tr key={`${field.location}-${field.name}`}><td><code>{field.name}</code><small>{field.location} · {field.required ? 'required' : 'optional'}</small></td><td>{field.type}</td><td>{field.description || '—'}{field.constraints ? <small>{field.constraints}</small> : null}</td></tr>)}</tbody></table></div></> : null}
      <div className="docs-example-grid"><section><h3>Request example</h3><p className="docs-example-note">Ilustrasi dari schema, bukan request yang sudah dieksekusi. Gunakan ID, revision, scope yang tersedia, dan nilai terkini. Jangan menyalin placeholder ke production.</p>
      <CodeBlock text={operation.curl} label="cURL · placeholders" onNotice={onNotice} />
      {operation.request ? <details className="docs-schema"><summary>Request schema · {operation.request.contentType}</summary><pre tabIndex={0} aria-label="Request schema">{JSON.stringify(operation.request.schema, null, 2)}</pre></details> : null}</section>
      <section><div className="docs-response-heading"><h3>Responses</h3><label>Status <select value={status} onChange={(event) => setStatus(event.target.value)}>{operation.responses.map((item) => <option key={item.status} value={item.status}>{item.status}</option>)}</select></label></div>
      {response ? <><p className="docs-description">{response.description}</p>{response.example !== null ? <CodeBlock text={JSON.stringify(response.example, null, 2)} label={`${response.status} · schema illustration, not live data`} onNotice={onNotice} /> : <p className="docs-example-note">Tidak ada JSON body yang didefinisikan untuk status ini; periksa deskripsi dan header pada OpenAPI.</p>}<details className="docs-schema"><summary>Response schema</summary><pre tabIndex={0} aria-label="Response schema">{JSON.stringify(response.schema, null, 2)}</pre></details></> : null}</section></div>
    </div>
  </details>;
}

function OperatorGuide({ onReference }: { onReference: (contract: ContractId, operationId?: string) => void }) {
  const steps: { title: string; text: string; contract: ContractId; operationId: string; action: string }[] = [
    { title: 'Catat developer yang dipilih Emisell', text: 'Di Developer requests, verifikasi perusahaan, domain, email bisnis, use case, jenis app, dan kebutuhan scope. Tidak ada pendaftaran developer publik.', contract: 'admin', operationId: 'createDeveloperApplication', action: 'Create candidate' },
    { title: 'Review dengan revision terbaru', text: 'Buka detail kandidat, mulai review, lalu simpan catatan tanpa secret atau dokumen identitas sensitif. Jika ditolak, tulis alasan. Konflik 409 perlu reload, bukan retry membabi buta.', contract: 'admin', operationId: 'reviewDeveloperApplication', action: 'Review API' },
    { title: 'Approve dan kirim undangan secara aman', text: 'Approval sekaligus membuat organisasi, entitlement development, dan owner invitation. Kode hanya ditampilkan sekali. Kirim lewat kanal terverifikasi; jangan taruh di URL, log, atau tiket. Approval tidak memberi production access.', contract: 'admin', operationId: 'approveDeveloperApplication', action: 'Approval API' },
    { title: 'Developer menerima undangan', text: 'Developer login memakai identitas dengan verified email yang cocok, lalu memasukkan kode di Accept invitation. Kode satu kali pakai; default lokal 48 jam. Operator dapat revoke atau menerbitkan ulang jika diperlukan.', contract: 'identity', operationId: 'acceptDeveloperInvitation', action: 'Acceptance API' },
    { title: 'Bangun app dan uji lewat Merchant ID', text: 'Developer mengatur URL/callback, extension, scopes, credential, dan webhook; membuat version snapshot lalu undangan test install ke Merchant ID. Merchant tetap login dan menyetujui consent. Memasukkan ID bukan otorisasi akses toko.', contract: 'developer', operationId: 'createDevelopmentInstallRequest', action: 'Development install API' },
    { title: 'Periksa konfigurasi dan bukti instalasi', text: 'Developer membuka tab Integration; Admin membuka App catalog → Review app. Hasil berasal dari backend, bukan tes koneksi. Instalasi aktif dan credential tersimpan tidak membuktikan resource calls atau penanganan uninstall berhasil. Periksa bukti uji secara terpisah.', contract: 'admin', operationId: 'getInternalIntegrationReadiness', action: 'Inspection API' },
    { title: 'Putuskan publikasi setelah review', text: 'Publish reviewed version mengirim revisi app dan versi yang diperiksa. Perubahan sebelum publikasi menghasilkan 409; refresh dan review ulang. Publikasi tetap keputusan operator dan tidak memberi production entitlement. Katalog published masih mengikuti versi aktif, bukan version pinning.', contract: 'admin', operationId: 'updateCatalogListing', action: 'Publication API' },
  ];
  return <div className="docs-guide">
    <Panel className="docs-guide-intro"><div className="docs-intro-icon"><BookOpen size={22} /></div><div><p className="docs-label">Operator playbook</p><h2>Dari developer invitation sampai app listing</h2><p>Gunakan panduan ini untuk pekerjaan admin sehari-hari. Buka API reference untuk parameter dan contoh sesuai kontrak aktual.</p></div><button className="secondary-button" onClick={() => onReference('admin')}>Explore Admin API <ArrowRight size={15} /></button></Panel>
    <div className="docs-guide-columns"><section><h2 className="docs-section-title">Alur yang tersedia sekarang</h2><ol className="docs-workflow">{steps.map((step, index) => <li key={step.title}><span>{String(index + 1).padStart(2, '0')}</span><div><h3>{step.title}</h3><p>{step.text}</p><button className="docs-text-button" onClick={() => onReference(step.contract, step.operationId)}>{step.action}<ArrowRight size={13} /></button></div></li>)}</ol></section>
      <aside className="docs-guide-aside"><Panel><p className="docs-label">Console boundaries</p><h3>Admin ≠ Developer ≠ Merchant</h3><p><strong>Admin:</strong> admission developer, organisasi read-only, kurasi katalog.</p><p><strong>Developer:</strong> app dan konfigurasi milik organisasi.</p><p><strong>Merchant:</strong> app store, consent, connected apps.</p><button className="docs-text-button" onClick={() => onReference('emisell')}>Backend integration <ArrowRight size={13} /></button></Panel>
        <Panel><p className="docs-label">Lifecycle</p><h3>Tiga status, tiga keputusan</h3><p><strong>Admission:</strong> submitted → under_review → invited → active.</p><p><strong>Version:</strong> snapshot konfigurasi yang direlease tidak otomatis menjadi listing publik.</p><p><strong>App:</strong> Development / Released mengikuti publikasi katalog. Environment internal tetap batas keamanan, bukan pengganti status app.</p></Panel>
        <Panel className="docs-boundary-panel"><p className="docs-label">Rollout boundaries</p><h3>Jangan dianggap sudah tersedia umum</h3><ul><li>Dashboard audit dan pencarian audit events.</li><li>Manajemen akun platform operator melalui UI/API.</li><li>Suspend/reactivate organisasi melalui Admin UI/API.</li><li>Workflow approval production.</li><li>Products baru pilot nonaktif secara default; orders/inventory dan webhook resource masih planned.</li></ul><p>Akun Admin sudah dikelola manual melalui CLI server; belum ada halaman manajemen akun. Audit records internal sudah ada; UI/API audit khusus belum ada. Payment dan Shipping tetap extension runtime terpisah.</p><button className="docs-text-button" onClick={() => onReference('resource')}>View resource pilot <ArrowRight size={13} /></button></Panel>
      </aside>
    </div>
  </div>;
}

function AuthenticationGuide() {
  const rows = [
    ['Admin operator', 'emisell_admin_session dari email + password akun manual.', 'Cookie dan CSRF khusus Admin. Token Developer dan role owner/admin organisasi tidak diterima.'],
    ['Developer management', 'Control-plane RS256 JWT atau emisell_session.', 'Role dan organisasi aktif menentukan hak. X-Organization-Id tidak boleh mengganti signed organization.'],
    ['Provider OAuth exchange', 'HTTP Basic: client ID + secret; authorization code + PKCE verifier.', 'Dilakukan dari backend app melalui HTTPS. Callback dan state harus diverifikasi; kode single-use.'],
    ['Provider resource', 'Opaque installation access token.', 'Merchant, installation, environment, dan effective scopes berasal dari token. Merchant ID saja tidak memberi akses.'],
    ['Managed extension runtime', 'Opaque er_ token dari operator Emisell; default-off.', 'Terikat ke satu instalasi–extension. Bukan token app eksternal. Secret tetap di App Platform dan hanya diambil runtime terikat; bukan oleh browser atau Emisell Backend.'],
    ['Emisell Backend → Platform', 'Dedicated short-lived RS256 service JWT.', 'Hanya server-to-server merchant-session grant; bukan pengganti merchant cookie atau token provider.'],
    ['Merchant browser', 'emisell_merchant_session; CSRF untuk mutasi.', 'Session terpisah dari admin. Store identity dari bridge Emisell yang terverifikasi.'],
    ['Platform → Emisell Backend', 'RS256 App Gateway assertion, pilot nonaktif secara default.', 'Baca produk memerlukan konfigurasi key dan daftar merchant uji. Jangan mengirim token instalasi provider langsung ke backend Emisell.'],
  ];
  return <section className="docs-guide"><h2 className="docs-section-title">Pilih credential sesuai pemanggil</h2><InlineNotice title="Akun Admin dibuat manual">Tidak ada public sign-up. Create, reset password, dan disable dilakukan lewat CLI server. Password asli tidak disimpan; reset dan disable mencabut seluruh sesi akun.</InlineNotice><Panel><div className="docs-table-wrap"><table className="docs-fields docs-auth-table"><thead><tr><th>Caller</th><th>Authentication</th><th>Boundary</th></tr></thead><tbody>{rows.map(([caller, authentication, boundary]) => <tr key={caller}><td><strong>{caller}</strong></td><td>{authentication}</td><td>{boundary}</td></tr>)}</tbody></table></div></Panel>
    <div className="docs-security-grid"><Panel><ShieldCheck size={19} /><h3>Session, cookie & CSRF</h3><p>Cookie session HttpOnly disimpan browser. Mutasi wajib memakai CSRF yang cocok. Session Admin, Developer, dan Merchant terpisah dan tidak dapat dipakai silang.</p></Panel><Panel><Code2 size={19} /><h3>Postman tanpa secret bawaan</h3><p>Isi variabel auth melalui environment lokal/private atau vault. baseUrl default menunjuk localhost. ID, revision, token, dan idempotency key sengaja belum diisi. Tinjau setiap request mutasi; jangan jalankan seluruh koleksi ke production.</p></Panel><Panel><FileJson size={19} /><h3>Akses dokumentasi internal</h3><p>Konten API dan semua unduhan diverifikasi melalui session Admin pada setiap request. Tidak ada fallback token Developer, cache respons, atau secret yang disisipkan ke contoh.</p></Panel></div>
    <InlineNotice tone="warning" title="Development convenience bukan keamanan production">Token bootstrap lokal memang public fixture. Jangan memakai nilai lokal atau NEXT_PUBLIC_* untuk production secret. Swagger Docker di port 8082 adalah viewer development terpisah dan tidak dilindungi login Admin; batasi ke localhost.</InlineNotice>
  </section>;
}

function TroubleshootingGuide() {
  const items = [
    ['401 · Session atau token ditolak', 'Login kembali atau cek issuer, audience, expiry, dan jenis token. Installation token tidak diterima di control plane. Cookie session yang kedaluwarsa tidak boleh dianggap valid karena ada token fallback.'],
    ['403 · Hak akses tidak sesuai', 'Untuk Admin, periksa akun manual masih aktif dan session Admin valid; role organisasi dan entitlement berlaku untuk Developer. Untuk mutasi berbasis cookie, cek pasangan CSRF. Jangan mencoba mengatasi dengan menambah header role/merchant ID.'],
    ['404 · Resource tidak ditemukan', 'Pastikan ID berasal dari respons API dan organisasi sesuai. Isolasi tenant dapat sengaja mengembalikan 404; jangan menyimpulkan ID global tidak ada.'],
    ['409 · Revision / state conflict', 'Ambil data terbaru, periksa status, lalu ulang keputusan secara sadar memakai revision terbaru. Untuk endpoint yang mensyaratkan Idempotency-Key, retry request identik memakai key yang sama; tindakan baru memakai key baru.'],
    ['422 · Input tidak sesuai', 'Baca schema: required fields, enum, format UUID, limit, revision, dan unknown fields. Bedakan Merchant ID opaque dengan app/organization UUID. Contoh schema tetap perlu diganti dengan data valid.'],
    ['Undangan tidak bisa diterima', 'Verifikasi email login cocok dengan alamat undangan, kode belum dipakai/expired/revoked, dan application state. Jangan meminta developer mengirim kode lewat log; operator dapat rotate undangan dari queue.'],
    ['App tidak muncul di App Store', 'Periksa status listing published dan hasil eligibility/blockingReason. App/version aktif belum cukup. Scope planned memblokir publikasi; production access juga tidak otomatis diberikan oleh publish.'],
    ['Webhook belum diterima', 'Baca event catalog: hanya event available yang boleh dipakai. Periksa subscription aktif, scope, URL HTTPS, status delivery dan retry. Test event memakai source=test; bukan bukti event resource backend sudah terhubung. Jangan log signing secret atau payload sensitif.'],
    ['502 · Dokumentasi gagal verifikasi identitas', 'Pastikan gateway hidup dan APP_GATEWAY_INTERNAL_URL server frontend benar. Di Docker gunakan http://app-gateway:8080, bukan localhost container. Jika nama cookie diubah, samakan AUTH_SESSION_COOKIE_NAME pada frontend server dan gateway.'],
  ];
  return <section className="docs-guide"><h2 className="docs-section-title">Periksa penyebab tanpa melemahkan akses</h2><p className="docs-description">Status berikut adalah panduan lintas API. Daftar response tiap endpoint tetap mengikuti OpenAPI.</p><div className="docs-troubleshooting">{items.map(([title, text]) => <details key={title}><summary>{title}<ChevronDown size={15} /></summary><p>{text}</p></details>)}</div><InlineNotice title="Saat eskalasi masalah">Sertakan metode/path tanpa kode rahasia, waktu kejadian, HTTP status, dan requestId dari error gateway. Jangan sertakan Authorization, cookie, CSRF token, invitation code, client secret, atau webhook secret.</InlineNotice></section>;
}
