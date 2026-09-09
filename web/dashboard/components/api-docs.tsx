'use client';

import { useState } from 'react';
import { BookOpen, Copy, Download, LockKeyhole, Search } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import GatewayHandoff from '@/components/gateway-handoff';
import GatewayOperationStatus from '@/components/gateway-operation-status';
import { useScopeReadiness } from '@/components/use-scope-readiness';
import type { PortalAPI } from '@/lib/portal';
import type { ScopeCatalog, ScopeVerification } from '@/lib/access-scopes';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import {
  Empty,
  EmptyHeader,
  EmptyTitle,
  EmptyDescription,
} from '@/components/ui/empty';
import {
  documents,
  example,
  filterOperations,
  groups,
  operations,
  schemaType,
  endpointAddress,
  type Operation,
  type Schema,
} from '@/lib/api-docs';

function Code({ title, value }: { title: string; value: string }) {
  const [notice, setNotice] = useState('');
  return (
    <section className="api-code">
      <div className="api-code-heading">
        <h3>{title}</h3>
        <Button
          variant="ghost"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(value);
              setNotice('Tersalin.');
            } catch {
              setNotice('Tidak bisa menyalin otomatis. Pilih teks di bawah.');
            }
          }}
        >
          <Copy />
          Salin
        </Button>
      </div>
      {notice && <output>{notice}</output>}
      <Textarea
        readOnly
        wrap="off"
        rows={Math.min(18, Math.max(2, value.split('\n').length))}
        aria-label={title}
        value={value}
        spellCheck={false}
      />
    </section>
  );
}
function Fields({ schema, rpc }: { schema: Schema; rpc: boolean }) {
  return (
    <dl className="api-fields">
      {Object.entries(schema.properties ?? {}).map(([name, field]) => (
        <div key={name}>
          <dt>
            <code>{name}</code>
            <span>
              {schemaType(field)}
              {schema.required?.includes(name)
                ? ' · wajib'
                : rpc
                  ? ''
                  : ' · opsional'}
            </span>
          </dt>
          <dd>
            {field.description ||
              (field.enum
                ? field.enum.join(' · ')
                : field.pattern
                  ? `Pola: ${field.pattern}`
                  : field.format
                    ? `Format: ${field.format}`
                    : 'Lihat skema untuk detail struktur.')}
            {field.minLength !== undefined && ` minLength: ${field.minLength}.`}
            {field.maxLength !== undefined && ` maxLength: ${field.maxLength}.`}
            {field.minimum !== undefined && ` Minimum ${field.minimum}.`}
            {field.maximum !== undefined && ` Maksimum ${field.maximum}.`}
          </dd>
        </div>
      ))}
    </dl>
  );
}
function Endpoint({
  operation: o,
  catalog,
  verification,
}: {
  operation: Operation;
  catalog: ScopeCatalog | null;
  verification: ScopeVerification | null;
}) {
  const [status, setStatus] = useState(o.responses[0]?.status ?? '200');
  const response =
    o.responses.find((r) => r.status === status) ?? o.responses[0];
  const rpc = o.method === 'RPC';
  return (
    <article className="api-detail" aria-label="Detail endpoint">
      <div className="api-endpoint-heading">
        <span className={`api-method api-method-${o.method.toLowerCase()}`}>
          {o.method}
        </span>
        <span className="api-source">{o.source}</span>
      </div>
      <h2>{o.title}</h2>
      <code className="api-path">{o.path}</code>
      {o.group === 'gateway' && (
        <GatewayOperationStatus
          procedure={o.path}
          catalog={catalog}
          verification={verification}
        />
      )}
      {o.description && <p>{o.description}</p>}
      <div className="api-auth">
        <LockKeyhole aria-hidden="true" />
        <div>
          <strong>Authentication & authorization</strong>
          <p>{o.auth}</p>
        </div>
      </div>
      {rpc && (
        <p>
          Gunakan SDK Go atau Connect client. Nama field di bawah mengikuti
          ProtoJSON (camelCase); int64 berupa string. Validasi domain tetap
          berlaku meskipun Protobuf tidak memberi label required.
        </p>
      )}
      <Code
        title={
          o.target === 'emisell-core'
            ? 'Template URL · host disediakan tim Core'
            : 'URL lokal'
        }
        value={endpointAddress(o)}
      />
      <Tabs defaultValue="request" className="api-detail-tabs">
        <TabsList variant="line">
          <TabsTrigger value="request">Request</TabsTrigger>
          <TabsTrigger value="response">Response & error</TabsTrigger>
        </TabsList>
        <TabsContent value="request">
          {o.parameters.length > 0 && (
            <section>
              <h3>Parameter & header</h3>
              <dl className="api-fields">
                {o.parameters.map((p) => (
                  <div key={`${p.in}-${p.name}`}>
                    <dt>
                      <code>{p.name}</code>
                      <span>
                        {p.in} · {p.required ? 'wajib' : 'opsional'}
                      </span>
                    </dt>
                    <dd>
                      {p.description || schemaType(p.schema ?? {})}
                      {p.schema?.pattern && ` · ${p.schema.pattern}`}
                    </dd>
                  </div>
                ))}
              </dl>
            </section>
          )}
          {o.request ? (
            <section>
              <h3>Request body · application/json</h3>
              <Fields schema={o.request} rpc={rpc} />
              <p className="api-example-note">
                Contoh struktur dengan placeholder, bukan payload siap pakai.
                Ganti ID, nilai, dan izin sesuai validasi backend. Tidak ada
                request yang dijalankan dari halaman ini.
              </p>
              <Code
                title="Contoh struktur request"
                value={JSON.stringify(example(o.request), null, 2)}
              />
              <Code
                title="Skema request"
                value={JSON.stringify(o.request, null, 2)}
              />
            </section>
          ) : (
            <p>Operasi ini tidak mendefinisikan JSON request body.</p>
          )}
        </TabsContent>
        <TabsContent value="response">
          <section>
            <div className="api-response-heading">
              <h3>Response & error</h3>
              <NativeSelect
                aria-label="Status response"
                value={status}
                onChange={(e) => setStatus(e.target.value)}
              >
                {o.responses.map((r) => (
                  <NativeSelectOption key={r.status} value={r.status}>
                    {r.status}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </div>
            <p>{response?.description}</p>
            {response?.schema && (
              <Code
                key={status}
                title={`Skema response ${status}`}
                value={JSON.stringify(response.schema, null, 2)}
              />
            )}
          </section>
        </TabsContent>
      </Tabs>
    </article>
  );
}
export default function APIDocs({
  api,
  initialGroup = 'admin',
  initialOperation = '',
}: {
  api: PortalAPI;
  initialGroup?: string;
  initialOperation?: string;
}) {
  const [group, setGroup] = useState(
    groups.some((g) => g.id === initialGroup) ? initialGroup : 'admin',
  );
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState(initialOperation);
  const readiness = useScopeReadiness(api);
  const [downloadNotice, setDownloadNotice] = useState('');
  const matches = filterOperations(group, query);
  const current = selected
    ? matches.find((o) => o.id === selected)
    : matches[0];
  const category = groups.find((g) => g.id === group)!;
  return (
    <div className="api-docs">
      <div className="overview-heading">
        <div>
          <h1>Dokumentasi API</h1>
          <p>Kontrak, autentikasi, dan struktur data Emisell App Platform.</p>
        </div>
        <Button
          variant="outline"
          onClick={() =>
            document
              .getElementById('api-contract-downloads')
              ?.scrollIntoView({ behavior: 'smooth' })
          }
        >
          <Download /> Unduh kontrak
        </Button>
      </div>
      <details className="api-reference-guides">
        <summary>
          <BookOpen size={16} /> Panduan integrasi · {operations.length} operasi
        </summary>
        <div className="api-contract-note">
          <strong>Referensi read-only</strong>
          <p>
            API Platform lokal: REST/JSON di :8087 · ConnectRPC di :8088.
            Kelompok Gateway Emisell berada di backend Core, bukan pada port
            tersebut. Status dibaca dari sumber verifikasi Katalog Scope.
            Dokumentasi tidak memuat token atau data merchant.
          </p>
        </div>
        <details className="api-contract-note">
          <summary>
            Embedded Apps · panduan identitas v1 (fondasi, belum endpoint live)
          </summary>
          <p>
            Target alur: login Core → Open app → iframe terverifikasi → token
            identitas → validasi backend aplikasi. Akses resource memakai token
            terpisah sesuai scope dan grant, bukan token identitas.
          </p>
          <p>
            API Go tersedia: <code>pkg/embedded.Issue</code> dan{' '}
            <code>Verify</code>, serta{' '}
            <code>internal/oauth/embedded.Service.Issue</code> /{' '}
            <code>Authenticate</code>
            dengan pemeriksaan akses terkini yang wajib. Token Ed25519 berlaku
            60 detik, terikat merchant, staf, app-client dan installation.
          </p>
          <p>
            Signature valid bukan bukti grant aktif. Jangan simpan token di URL
            atau penyimpanan persisten browser, jangan kirim key Core/credential
            provider ke iframe. Signed launch binding, adapter HTTP penerbitan
            sesi dan SDK iframe/bridge sudah tersedia sebagai reference yang
            diuji, tetapi belum terhubung ke sesi Core dan Open app seller.
            Token exchange belum tersedia; scope Plan tidak berubah. Panduan
            lengkap repository:
            <code> docs/embedded-apps.md</code>.
          </p>
        </details>
        <section className="api-guidance" aria-label="Aturan integrasi">
          <div>
            <h2>Auth sesuai surface</h2>
            <p>
              /admin dan /development memakai login terpadu dengan peran yang
              diverifikasi backend serta Origin/Referer yang sesuai. Core
              memakai bearer server-to-server, bukan sesi browser.
            </p>
          </div>
          <div>
            <h2>Retry & idempotency</h2>
            <p>
              Ikuti field/header pada tiap operasi. Retry hasil ambigu dengan
              key dan body sama. Core: key 16–128 karakter; metadata
              portal/katalog: 8–128 karakter.
            </p>
          </div>
          <div>
            <h2>Consent bukan instalasi</h2>
            <p>
              Grant intent berumur 10 menit; executionAllowed tetap false
              setelah consent. UI grant milik Core. Katalog metadata belum
              installable.
            </p>
          </div>
        </section>
      </details>
      <Tabs
        className="api-group-tabs"
        value={group}
        onValueChange={(next) => {
          setGroup(next);
          setQuery('');
          setSelected('');
        }}
      >
        <TabsList variant="line" aria-label="Kelompok API">
          {groups.map((g) => (
            <TabsTrigger key={g.id} value={g.id}>
              {g.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>
      <div className="api-toolbar">
        <div className="api-search">
          <Search aria-hidden="true" />
          <Input
            aria-label="Cari endpoint API"
            placeholder="Cari endpoint, operasi, atau scope…"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setSelected('');
            }}
          />
        </div>
      </div>
      <p className="api-group-note">{category.note}</p>
      {group === 'gateway' && (
        <GatewayHandoff
          verification={readiness.verification}
          checking={readiness.checking}
          error={readiness.error || readiness.verificationError}
          refreshStatus={readiness.refreshStatus}
        />
      )}
      {matches.length ? (
        <div className="api-browser">
          <nav className="api-endpoints" aria-label="Daftar endpoint">
            <p>
              {matches.length} operasi · {category.label}
            </p>
            {matches.map((o) => (
              <button
                type="button"
                key={o.id}
                aria-current={o.id === current?.id ? 'true' : undefined}
                onClick={() => setSelected(o.id)}
              >
                <span
                  className={`api-method api-method-${o.method.toLowerCase()}`}
                >
                  {o.method}
                </span>
                <span>
                  <strong>{o.title}</strong>
                  <code>{o.path}</code>
                </span>
              </button>
            ))}
          </nav>
          {current ? (
            <Endpoint
              key={current.id}
              operation={current}
              catalog={readiness.catalog}
              verification={readiness.verification}
            />
          ) : (
            <section
              className="api-detail"
              aria-label="Endpoint tidak ditemukan"
            >
              <h2>Endpoint tidak ditemukan</h2>
              <p>
                Kontrak pada tautan ini belum tersedia di versi dokumentasi ini.
                Pilih endpoint dari daftar atau kembali ke Katalog Scope.
              </p>
              <Button
                variant="outline"
                onClick={() => window.location.assign('/admin?view=scopes')}
              >
                Katalog Scope →
              </Button>
            </section>
          )}
        </div>
      ) : (
        <Empty>
          <EmptyHeader>
            <EmptyTitle>Endpoint tidak ditemukan</EmptyTitle>
            <EmptyDescription>
              Ubah kata pencarian atau pilih kelompok API lain.
            </EmptyDescription>
          </EmptyHeader>
          <Button variant="outline" onClick={() => setQuery('')}>
            Hapus pencarian
          </Button>
        </Empty>
      )}
      <section className="api-guidance api-webhooks">
        <div>
          <h2>Webhook & events</h2>
          <p>
            Webhook reference ditandatangani HMAC-SHA256. Verifikasi raw body
            dengan pkg/appapi.VerifyWebhook, timestamp, tenant, installation,
            dan delivery ID sebelum memproses.
          </p>
        </div>
        <div>
          <h2>Replay protection</h2>
          <p>
            Timestamp maksimal 5 menit lalu / 30 detik ke depan. Deduplicate
            delivery ID secara durable; signature valid saja belum mencegah
            pemrosesan ulang. Delivery delivered bukan bukti pembayaran sukses.
          </p>
        </div>
      </section>
      <section
        className="api-downloads"
        id="api-contract-downloads"
        tabIndex={-1}
      >
        <h2>Sumber kontrak</h2>
        <p>
          Unduh definisi yang dipakai halaman ini. Simpan file OpenAPI bersama
          agar referensi schema katalog dan access-scopes dapat di-resolve.
          Endpoint workspace merchant tidak termasuk dokumentasi portal ini.
        </p>
        <div>
          {documents.map((d) => (
            <Button
              key={d.name}
              variant="outline"
              onClick={() => {
                try {
                  const url = URL.createObjectURL(
                    new Blob([d.content], {
                      type:
                        d.format === 'OpenAPI'
                          ? 'application/json'
                          : 'text/plain',
                    }),
                  );
                  const link = document.createElement('a');
                  link.href = url;
                  link.download = d.name.split('/').pop()!;
                  document.body.appendChild(link);
                  link.click();
                  link.remove();
                  setTimeout(() => URL.revokeObjectURL(url), 1000);
                  setDownloadNotice(`Unduhan ${link.download} dimulai.`);
                } catch {
                  setDownloadNotice('Unduhan belum berhasil. Coba kembali.');
                }
              }}
            >
              <Download />
              {d.name.split('/').pop()}
            </Button>
          ))}
        </div>
        <output>{downloadNotice}</output>
      </section>
    </div>
  );
}
