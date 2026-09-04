'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import { ArrowLeft, ArrowRight, BookOpen, Check, ChevronDown, Copy, Download, FileCode2, LoaderCircle, Search } from 'lucide-react';
import { apiErrorMessage, appPlatformClient } from '../../lib/app-platform/client';
import type { ScopeDefinition, WebhookEventDefinition } from '../../lib/app-platform/domain';
import type { DeveloperChapter, DeveloperContractId, DeveloperGuide } from '../../lib/documentation/developer-types';
import type { DocumentationOperation, DocumentationReference } from '../../lib/documentation/types';
import './developer-documentation.css';
import { ExtensionCatalogPanel } from '../extension-catalog';

const statusLabels: Record<DeveloperChapter['status'], string> = { guide: 'Guide', mixed: 'Periksa status fitur', planned: 'Belum didukung', pilot: 'Pilot · default off' };

export function DeveloperDocumentation() {
  const params = useSearchParams();
  const router = useRouter();
  const chapterId = params.get('chapter') ?? 'getting-started';
  const [guide, setGuide] = useState<DeveloperGuide | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const titleRef = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient.getDeveloperGuide(controller.signal)
      .then((value) => { if (!controller.signal.aborted) { setGuide(value); setError(null); } })
      .catch((reason) => { if (!controller.signal.aborted) setError(apiErrorMessage(reason)); });
    return () => controller.abort();
  }, [retry]);
  useEffect(() => { if (guide) titleRef.current?.focus({ preventScroll: true }); }, [chapterId, guide]);

  const chapter = guide?.chapters.find((entry) => entry.id === chapterId);
  const index = guide?.chapters.findIndex((entry) => entry.id === chapterId) ?? -1;
  const navigate = (id: string) => router.push(`/docs?chapter=${encodeURIComponent(id)}`);
  const reference = (contract: DeveloperContractId, operationId: string) => router.push(`/docs?chapter=api-reference&contract=${contract}&endpoint=${encodeURIComponent(operationId)}#${operationId}`);

  return <main className="dev-docs" lang="id">
    <header className="dev-docs-header"><div><p className="dev-docs-eyebrow">Emisell App Platform / Developers</p><h1>Documentation</h1><p>Panduan membangun app, dari konfigurasi pertama sampai siap ditinjau.</p></div><span className="dev-docs-access"><BookOpen size={16} />Developer guide</span></header>
    {error ? <div className="dev-docs-state" role="alert"><h2>Dokumentasi belum dapat dimuat</h2><p>{error}</p><div className="dev-docs-actions"><button className="secondary-button" onClick={() => { setError(null); setRetry((value) => value + 1); }}>Coba lagi</button><a className="secondary-button" href="/login">Login developer</a></div><p className="dev-docs-note">Gunakan identitas developer dan organisasi aktif. Session Admin atau Merchant tidak menggantikannya.</p></div>
      : !guide ? <div className="dev-docs-state" role="status"><LoaderCircle className="spin" size={20} /><p>Memverifikasi akses dan memuat panduan…</p></div>
      : <div className="dev-docs-workspace">
        <aside className="dev-docs-nav"><p className="dev-docs-eyebrow">Build your app</p><nav aria-label="Developer documentation chapters">{guide.chapters.map((item, position) => <a key={item.id} href={`/docs?chapter=${item.id}`} aria-current={chapterId === item.id ? 'page' : undefined} onClick={(event) => { if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return; event.preventDefault(); navigate(item.id); }}><span>{String(position + 1).padStart(2, '0')}</span><div><strong>{item.title}</strong><small>{item.label}</small></div></a>)}</nav><div className="dev-docs-nav-note"><FileCode2 size={17} /><p>Guide menjelaskan alur.<br />API reference menjelaskan kontrak.</p></div></aside>
        {chapter ? <article className="dev-docs-article" key={chapter.id}>
          <header className="dev-docs-chapter-header"><span className={`dev-docs-badge status-${chapter.status}`}>{statusLabels[chapter.status]}</span><h2 ref={titleRef} tabIndex={-1}>{chapter.title}</h2><p>{chapter.summary}</p></header>
          <section className="dev-docs-goal"><p className="dev-docs-eyebrow">Tujuan</p><p>{chapter.purpose}</p><h3>Sebelum mulai</h3><ul>{chapter.prerequisites.map((item) => <li key={item}>{item}</li>)}</ul></section>
          <nav className="dev-docs-toc" aria-label="On this page"><span>Di halaman ini</span>{chapter.sections.map((section) => <a key={section.id} href={`#${section.id}`}>{section.title}</a>)}<a href="#verify">Pengujian & batasan</a></nav>
          {chapter.sections.map((section) => <section className="dev-docs-section" id={section.id} key={section.id}><h3>{section.title}</h3>{section.paragraphs?.map((paragraph) => <p key={paragraph}>{paragraph}</p>)}{section.steps ? <ol>{section.steps.map((step) => <li key={step}>{step}</li>)}</ol> : null}{section.code ? <CodeSnippet {...section.code} /> : null}<div className="dev-docs-actions">{section.links?.map((link) => <a key={link.href} className="dev-docs-link" href={link.href}>{link.label}<ArrowRight size={14} /></a>)}{section.api?.map((link) => <button className="dev-docs-link" key={link.operationId} onClick={() => reference(link.contract, link.operationId)}>{link.label}<ArrowRight size={14} /></button>)}</div></section>)}
          {chapter.catalog === 'extensions' ? <ExtensionCatalogPanel /> : chapter.catalog ? <LiveCatalog key={chapter.catalog} kind={chapter.catalog} /> : null}
          {chapter.id === 'api-reference' ? <DeveloperApiReference key={params.get('contract') ?? 'provider'} /> : null}
          <section className="dev-docs-section" id="verify"><h3>Periksa hasilnya</h3><ul className="dev-docs-checklist">{chapter.verify.map((item) => <li key={item}><Check size={16} /><span>{item}</span></li>)}</ul><div className="dev-docs-limits"><h4>Batasan yang perlu diingat</h4><ul>{chapter.limits.map((item) => <li key={item}>{item}</li>)}</ul></div></section>
          <footer className="dev-docs-pagination">{index > 0 ? <button onClick={() => navigate(guide.chapters[index - 1].id)}><ArrowLeft size={17} /><span><small>Sebelumnya</small>{guide.chapters[index - 1].title}</span></button> : <span />}{index < guide.chapters.length - 1 ? <button onClick={() => navigate(guide.chapters[index + 1].id)}><span><small>Selanjutnya</small>{guide.chapters[index + 1].title}</span><ArrowRight size={17} /></button> : null}</footer>
        </article> : <div className="dev-docs-state"><h2>Bab tidak ditemukan</h2><p>Pilih salah satu bab di navigasi.</p><button className="secondary-button" onClick={() => navigate('getting-started')}>Getting started</button></div>}
      </div>}
  </main>;
}

function CodeSnippet({ label, text }: { label: string; text: string }) {
  const [message, setMessage] = useState('');
  useEffect(() => { if (!message) return; const timer = window.setTimeout(() => setMessage(''), 2200); return () => window.clearTimeout(timer); }, [message]);
  return <div className="dev-docs-code"><header><span>{label}</span><button aria-label={`Salin ${label}`} onClick={async () => { try { await navigator.clipboard.writeText(text); setMessage('Tersalin'); } catch { setMessage('Gagal menyalin; pilih teks secara manual'); } }}><Copy size={14} />Salin</button></header><pre tabIndex={0} aria-label={label}><code>{text}</code></pre><span className="dev-docs-copy-status" role="status">{message}</span></div>;
}

function LiveCatalog({ kind }: { kind: 'scopes' | 'webhooks' }) {
  const [data, setData] = useState<(ScopeDefinition | WebhookEventDefinition)[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    const request = kind === 'scopes' ? appPlatformClient.listScopeCatalog(controller.signal) : appPlatformClient.listWebhookEventCatalog(controller.signal);
    request.then((value) => { if (!controller.signal.aborted) { setData(value.data); setError(null); } }).catch((reason) => { if (!controller.signal.aborted) setError(apiErrorMessage(reason)); });
    return () => controller.abort();
  }, [kind, retry]);
  return <section className="dev-docs-section"><h3>{kind === 'scopes' ? 'Scope catalog' : 'Webhook event catalog'}</h3><p className="dev-docs-note">Dibaca langsung dari <code>{kind === 'scopes' ? '/v1/scope-catalog' : '/v1/webhook-event-catalog'}</code>. Status planned tidak memberi izin memakai fitur.</p>{error ? <div role="alert"><p>{error}</p><button className="secondary-button" onClick={() => { setError(null); setRetry((value) => value + 1); }}>Muat ulang katalog</button></div> : !data ? <p role="status">Memuat katalog dari backend…</p> : !data.length ? <p>Katalog tidak mengembalikan entri. Jangan mengasumsikan nama scope/event sendiri.</p> : <div className="dev-docs-table-wrap"><table><thead><tr><th>{kind === 'scopes' ? 'Scope' : 'Event'}</th><th>Status</th><th>Kontrak</th></tr></thead><tbody>{data.map((entry) => <tr key={'scope' in entry ? entry.scope : entry.event}><td><code>{'scope' in entry ? entry.scope : entry.event}</code><small>{entry.name}</small></td><td><span className={`dev-docs-badge ${entry.availability === 'available' ? 'status-guide' : 'status-planned'}`}>{entry.availability}</span></td><td><p>{entry.description}</p><small>{'scope' in entry ? `Approval: ${entry.approval}` : `Source: ${entry.source} · Scope: ${entry.requiredScope ?? 'Tidak memerlukan resource scope'}`}</small></td></tr>)}</tbody></table></div>}</section>;
}

function DeveloperApiReference() {
  const params = useSearchParams();
  const router = useRouter();
  const contract: DeveloperContractId = 'provider';
  const endpoint = params.get('endpoint');
  const [reference, setReference] = useState<DocumentationReference | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const [query, setQuery] = useState('');
  const [downloading, setDownloading] = useState(false);
  const [notice, setNotice] = useState('');
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    const requested = params.get('contract');
    if (requested && requested !== contract) router.replace('/docs?chapter=api-reference&contract=provider#reference', { scroll: false });
  }, [contract, params, router]);
  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient.getDeveloperReference(contract, controller.signal).then((value) => { if (!controller.signal.aborted) { setReference(value); setError(null); } }).catch((reason) => { if (!controller.signal.aborted) setError(apiErrorMessage(reason)); });
    return () => controller.abort();
  }, [contract, retry]);
  const loaded = reference?.selected.id === contract;
  useEffect(() => {
    if (loaded && endpoint) document.getElementById(endpoint)?.scrollIntoView({ block: 'start' });
  }, [endpoint, loaded]);
  const filtered = useMemo(() => reference?.operations.filter((operation) => `${operation.method} ${operation.path} ${operation.summary} ${operation.tag}`.toLowerCase().includes(query.trim().toLowerCase())) ?? [], [reference, query]);
  async function download(format: 'openapi' | 'postman') {
    setDownloading(true); setNotice('');
    try {
      const result = await appPlatformClient.getDeveloperDocumentationDownload(contract, format);
      const url = URL.createObjectURL(new Blob([JSON.stringify(result, null, 2)], { type: 'application/json' }));
      const link = document.createElement('a'); link.href = url; link.download = `emisell-${contract}.${format === 'openapi' ? 'openapi' : 'postman_collection'}.json`; link.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000); setNotice('Unduhan siap. Isi credential hanya di environment privat.');
    } catch (reason) { setNotice(apiErrorMessage(reason)); }
    finally { setDownloading(false); }
  }
  return <section className="dev-docs-section dev-docs-reference" id="reference"><h3 ref={heading}>Partner API reference</h3><div className="dev-docs-partner-boundary" role="note"><strong>Satu API untuk backend app</strong><p>Gunakan endpoint di bawah setelah OAuth. Pengelolaan app dilakukan melalui Developer Console; Payment dan Shipping memakai kontrak runtime terpisah setelah capability disetujui Emisell.</p></div>
    {error ? <div role="alert"><p>{error}</p><button className="secondary-button" onClick={() => setRetry((value) => value + 1)}>Coba lagi</button></div> : !loaded || !reference ? <p role="status">Memuat kontrak…</p> : <><p>{reference.selected.description}</p><p className="dev-docs-note">v{reference.selected.version} · {reference.selected.count} endpoint · Hanya referensi, tidak mengeksekusi request.</p><div className="dev-docs-reference-toolbar"><label><Search size={16} /><input aria-label="Cari endpoint API" placeholder="Cari endpoint atau fungsi…" value={query} onChange={(event) => setQuery(event.target.value)} /></label><button className="secondary-button" disabled={downloading} onClick={() => void download('openapi')}><Download size={14} />OpenAPI</button><button className="secondary-button" disabled={downloading} onClick={() => void download('postman')}><Download size={14} />Postman</button></div><p role="status" className="dev-docs-note">{notice || `${filtered.length} endpoint ditampilkan`}</p>{endpoint && !reference.operations.some((operation) => operation.id === endpoint) ? <p role="status">Endpoint yang ditautkan tidak ditemukan di kontrak ini.</p> : null}{filtered.length ? filtered.map((operation) => <ApiOperation key={`${contract}-${operation.id}`} operation={operation} open={endpoint === operation.id} />) : <p>Tidak ada endpoint yang cocok. Coba kata pencarian lain.</p>}</>}
  </section>;
}

function ApiOperation({ operation, open }: { operation: DocumentationOperation; open: boolean }) {
  const [responseStatus, setResponseStatus] = useState(operation.responses[0]?.status ?? '200');
  const response = operation.responses.find((entry) => entry.status === responseStatus);
  return <details className="dev-docs-endpoint" id={operation.id} open={open || undefined}><summary><span className={`dev-docs-method method-${operation.method.toLowerCase()}`}>{operation.method}</span><span><code>{operation.path}</code><small>{operation.summary}</small></span><ChevronDown size={16} /></summary><div className="dev-docs-endpoint-body"><p>{operation.description}</p><h4>Authentication</h4><ul>{operation.authentication.map((entry) => <li key={entry}>{entry}</li>)}</ul>{operation.fields.length ? <><h4>Parameters & body fields</h4><div className="dev-docs-table-wrap"><table><thead><tr><th>Field</th><th>Type</th><th>Ketentuan</th></tr></thead><tbody>{operation.fields.map((field) => <tr key={`${field.location}-${field.name}`}><td><code>{field.name}</code><small>{field.location} · {field.required ? 'required' : 'optional'}</small></td><td>{field.type}</td><td>{field.description}<small>{field.constraints}</small></td></tr>)}</tbody></table></div></> : null}<h4>Contoh request · placeholder</h4><CodeSnippet label="cURL" text={operation.curl} />{operation.request ? <details className="dev-docs-schema"><summary>Request schema</summary><CodeSnippet label={operation.request.contentType} text={JSON.stringify(operation.request.schema, null, 2)} /></details> : null}<label className="dev-docs-response-status">Response <select value={responseStatus} onChange={(event) => setResponseStatus(event.target.value)}>{operation.responses.map((item) => <option key={item.status} value={item.status}>{item.status}</option>)}</select></label>{response ? <><p>{response.description}</p>{response.example !== null ? <CodeSnippet label={`${response.status} · ilustrasi, bukan data live`} text={JSON.stringify(response.example, null, 2)} /> : <p>Tidak ada JSON body untuk respons ini.</p>}<details className="dev-docs-schema"><summary>Response schema</summary><CodeSnippet label="JSON Schema" text={JSON.stringify(response.schema, null, 2)} /></details></> : null}</div></details>;
}
