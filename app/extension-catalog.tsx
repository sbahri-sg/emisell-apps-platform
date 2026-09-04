'use client';

import { useEffect, useState } from 'react';
import { ArrowRight, LoaderCircle, RefreshCw } from 'lucide-react';
import { apiErrorMessage, appPlatformClient } from '../lib/app-platform/client';
import type { ExtensionCatalog } from '../lib/app-platform/extension-catalog';
import './extension-catalog.css';

export function useExtensionCatalog() {
  const [catalog, setCatalog] = useState<ExtensionCatalog | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    appPlatformClient.getExtensionCatalog(controller.signal)
      .then(value => { if (!controller.signal.aborted) { setCatalog(value); setError(null); } })
      .catch(reason => { if (!controller.signal.aborted) setError(apiErrorMessage(reason)); });
    return () => controller.abort();
  }, [retry]);
  return { catalog, error, reload: () => { setCatalog(null); setError(null); setRetry(value => value + 1); } };
}

const labels = { available: 'Available · tetap perlu izin', pilot: 'Pilot · default off', planned: 'Planned · belum aktif' };

export function ExtensionCatalogPanel() {
  const { catalog, error, reload } = useExtensionCatalog();
  const [view, setView] = useState<'capabilities' | 'runtime' | 'categories' | 'surfaces'>('capabilities');
  const [family, setFamily] = useState('all');
  const visible = catalog?.capabilities.filter(item => family === 'all' || item.type === family) ?? [];
  return <section className="extension-catalog" lang="id" aria-label="App and extension catalog">
    <header><div><h2>App & extension catalog</h2><p>Kategori menjelaskan kegunaan. Kemampuan menentukan kontrak. Surface menentukan tempat UI.</p></div><button className="secondary-button" onClick={reload} aria-label="Muat ulang katalog"><RefreshCw size={15} />Refresh</button></header>
    {error ? <div role="alert"><p>{error}</p><p>Katalog belum tersedia. Tidak ada daftar pengganti atau fitur yang dianggap aktif.</p></div> : !catalog ? <p role="status"><LoaderCircle size={16} className="spin" /> Memuat katalog backend…</p> : <>
      <div className="extension-catalog-controls"><div role="group" aria-label="Bagian katalog">{(['capabilities', 'runtime', 'categories', 'surfaces'] as const).map(id => <button key={id} aria-pressed={view === id} onClick={() => setView(id)}>{id === 'capabilities' ? 'Capabilities' : id === 'runtime' ? 'Runtime contract' : id === 'categories' ? 'App categories' : 'UI surfaces'}</button>)}</div>{view === 'capabilities' ? <label>Family <select value={family} onChange={event => setFamily(event.target.value)}><option value="all">Semua</option>{catalog.families.map(item => <option key={item.type} value={item.type}>{item.name}</option>)}</select></label> : null}</div>
      <p className="extension-catalog-note">Satu app dapat memiliki beberapa extension. Memilih kategori atau menyimpan konfigurasi tidak memberikan scope dan tidak mengaktifkan operasi.</p>
      <div className="extension-catalog-items" aria-live="polite">
        {view === 'capabilities' ? visible.length ? visible.map(item => <article key={item.id}><div className="extension-catalog-item-head"><h3>{item.name}</h3><span className={`extension-catalog-status ${item.availability}`}>{labels[item.availability]}</span></div><p>{item.description}</p><dl><div><dt>Capability</dt><dd><code>{item.id}</code> · bukan scope</dd></div><div><dt>Arah panggilan</dt><dd>{item.invocation === 'provider_to_platform' ? 'Backend app → App Platform' : 'App Platform → Provider (belum tersambung)'}</dd></div><div><dt>Scope</dt><dd>{item.requiredScopes.join(', ') || 'Belum ditetapkan; tidak ada izin execution'}</dd></div></dl>{item.endpoints.map(endpoint => <code className="extension-catalog-route" key={endpoint.path}>{endpoint.method} {endpoint.path}</code>)}<ul>{item.limitations.map(text => <li key={text}>{text}</li>)}</ul><a href={`/docs?chapter=${encodeURIComponent(item.guideChapter)}`}>Pelajari kontrak<ArrowRight size={14} /></a></article>) : <p>Tidak ada kemampuan untuk family ini.</p> : null}
        {view === 'runtime' ? <RuntimeContract contract={catalog.runtimeContract} /> : null}
        {view === 'categories' ? catalog.categories.length ? catalog.categories.map(item => <article key={item.id}><h3>{item.name}</h3><p>{item.description}</p><p className="extension-catalog-note">Contoh use case: {item.examples.join(', ')}. Bukan daftar fitur yang sudah aktif.</p><code>listing.category = {item.id}</code></article>) : <p>Belum ada kategori.</p> : null}
        {view === 'surfaces' ? catalog.surfaces.length ? catalog.surfaces.map(item => <article key={item.id}><div className="extension-catalog-item-head"><h3>{item.name}</h3><span className={`extension-catalog-status ${item.availability}`}>{labels[item.availability]}</span></div><p>{item.description}</p></article>) : <p>Belum ada surface.</p> : null}
      </div><footer>Registry {catalog.version} · Dibaca dari backend · Bukan pemeriksaan kesiapan instalasi</footer>
    </>}
  </section>;
}

function RuntimeContract({ contract }: { contract: ExtensionCatalog['runtimeContract'] }) {
  return <div className="extension-runtime-contract">
    <article className="extension-runtime-summary">
      <div className="extension-catalog-item-head"><h3>Extension Runtime Contract {contract.version}</h3><span className="extension-catalog-status planned">Draft · execution off</span></div>
      <p>Kontrak panggilan App Platform → Payment/Shipping provider. Ini belum endpoint aktif dan tidak memberi izin baru.</p>
      <dl>
        <div><dt>Transport</dt><dd>HTTPS + JSON</dd></div>
        <div><dt>Authentication</dt><dd>JWT per invocation · berlaku maksimal {contract.tokenTtlSeconds} detik</dd></div>
        <div><dt>Tenant binding</dt><dd>{contract.identityClaims.join(', ')}</dd></div>
        <div><dt>Retry platform</dt><dd>Tidak otomatis pada v1; mutasi memakai idempotency key yang sama</dd></div>
      </dl>
      <ul>{contract.invariants.map(rule => <li key={rule}>{rule}</li>)}</ul>
    </article>
    <div className="extension-runtime-operations">
      {contract.operations.map(operation => <article key={operation.id}>
        <div className="extension-catalog-item-head"><h3><code>{operation.id}</code></h3><span className="extension-catalog-status planned">Belum aktif</span></div>
        <p>{operation.description}</p>
        <dl>
          <div><dt>Capability</dt><dd><code>{operation.capabilityId}</code></dd></div>
          <div><dt>Mode</dt><dd>Synchronous · timeout {operation.timeoutMs / 1000}s</dd></div>
          <div><dt>Idempotency</dt><dd>{operation.idempotency === 'required' ? 'Wajib' : 'Tidak wajib'} · max {operation.maximumAttempts} attempt</dd></div>
        </dl>
      </article>)}
    </div>
  </div>;
}
