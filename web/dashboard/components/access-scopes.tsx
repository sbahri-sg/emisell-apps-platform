'use client';

import { useEffect, useId, useState } from 'react';
import {
  ShieldCheck,
  Search,
  BookOpen,
  Files,
  CalendarClock,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
  TableCaption,
} from '@/components/ui/table';
import { ScopeSummary } from '@/components/scope-summary';
import { useScopeReadiness } from '@/components/use-scope-readiness';
import { gatewayOperationURL } from '@/lib/scope-readiness';
import { PortalAPI } from '@/lib/portal';
import {
  declarationError,
  filterScopes,
  scopeStatus,
  selectScope,
  verifiedScopeState,
  verifiedOperationState,
  readinessLabel,
  type ScopeDeclaration,
} from '@/lib/access-scopes';

export default function AccessScopes({
  api,
  value,
  onChange,
  onValidity,
  disabled = false,
  initialScope = '',
}: {
  api: PortalAPI;
  value?: ScopeDeclaration;
  onChange?: (value: ScopeDeclaration | undefined) => void;
  onValidity?: (valid: boolean) => void;
  disabled?: boolean;
  initialScope?: string;
}) {
  const {
    catalog,
    verification,
    verificationError,
    checking,
    error,
    refreshStatus,
  } = useScopeReadiness(api);
  const controlId = useId();
  const [query, setQuery] = useState(initialScope);
  const [filter, setFilter] = useState('all');
  const [resource, setResource] = useState('all');
  const [action, setAction] = useState('all');
  const [page, setPage] = useState(0);
  const adminCatalog = api.surface === 'admin' && !onChange;
  const validation = catalog ? declarationError(catalog, value) : '';
  useEffect(() => {
    onValidity?.(!value || Boolean(catalog && !validation && !error));
  }, [catalog, validation, error, value, onValidity]);
  const selected =
    (value?.required.length ?? 0) + (value?.optional.length ?? 0);
  const results = catalog
    ? filterScopes(catalog, query, filter, value, verification).filter(
        (s) =>
          !adminCatalog ||
          ((resource === 'all' || s.resource === resource) &&
            (action === 'all' || s.action === action)),
      )
    : [];
  const activeCount =
    catalog?.scopes.filter(
      (s) => verifiedScopeState(catalog, verification, s.handle) === 'active',
    ).length ?? 0;
  const plannedCount =
    catalog?.scopes.filter(
      (s) => verifiedScopeState(catalog, verification, s.handle) === 'planned',
    ).length ?? 0;
  const unknownCount =
    (catalog?.scopes.length ?? 0) - activeCount - plannedCount;
  const totalPages = Math.max(1, Math.ceil(results.length / 6));
  const currentPage = Math.min(page, totalPages - 1);
  const rows = adminCatalog
    ? results.slice(currentPage * 6, currentPage * 6 + 6)
    : results;
  return (
    <section
      className={`access-scopes ${onChange ? 'access-editor' : ''} ${adminCatalog ? 'admin-scope-page' : ''}`}
    >
      {adminCatalog ? (
        <>
          <div className="overview-heading">
            <div>
              <h1>Katalog scope</h1>
              <p>Satu referensi izin aplikasi dan kesiapan endpoint gateway.</p>
            </div>
            <Button
              variant="outline"
              onClick={() =>
                window.location.assign('/?view=api-docs&api_group=gateway')
              }
            >
              <BookOpen /> Dokumentasi gateway
            </Button>
          </div>
          <div className="scope-metrics">
            <article>
              <span>
                <Files />
              </span>
              <div>
                <p>Total scope</p>
                <strong>{catalog ? catalog.scopes.length : '—'}</strong>
              </div>
            </article>
            <article>
              <span className="scope-metric-active">
                <ShieldCheck />
              </span>
              <div>
                <p>Aktif</p>
                <strong>{catalog && verification ? activeCount : '—'}</strong>
              </div>
            </article>
            <article>
              <span className="scope-metric-plan">
                <CalendarClock />
              </span>
              <div>
                <p>Direncanakan</p>
                <strong>{catalog && verification ? plannedCount : '—'}</strong>
              </div>
            </article>
          </div>
        </>
      ) : (
        <>
          <div className="panel-title">
            <ShieldCheck />
            <h2>{onChange ? 'Akses data aplikasi' : 'Katalog scope'}</h2>
          </div>
          <p>
            Referensi nama izin Shopify authenticated. Pilih kebutuhan minimum;
            ini tidak menjadikan aplikasi kompatibel dengan API Shopify.
          </p>
          <p className="field-help">
            Scope data terkait pembayaran bukan izin menjadi payment gateway.
            Payment gateway dikelola internal Emisell; katalog referensi dan
            status Plan tetap dipertahankan.
          </p>
          <div className="access-notice">
            <strong>Status implementasi terpisah dari persetujuan akses</strong>
            <p>
              Required/optional dapat disimpan untuk review dan listing. Akses
              baru dapat diberikan setelah gateway mendukung operasi tersebut
              dan merchant menyetujuinya di Emisell Core. API key tidak
              menggantikan grant merchant.
            </p>
          </div>
          {api.surface === 'admin' && (
            <p>
              <Button
                type="button"
                variant="outline"
                onClick={() =>
                  window.location.assign('/?view=api-docs&api_group=gateway')
                }
              >
                Dokumentasi endpoint Gateway Emisell →
              </Button>
            </p>
          )}
        </>
      )}
      {error ? (
        <div role="alert">
          <p>{error}</p>
          <Button
            type="button"
            variant="outline"
            disabled={disabled}
            onClick={refreshStatus}
          >
            Muat ulang katalog scope
          </Button>
        </div>
      ) : !catalog ? (
        <Skeleton className="h-32 w-full" />
      ) : (
        <>
          <div className={adminCatalog ? 'scope-table-card' : undefined}>
            {adminCatalog && (
              <Tabs
                value={filter}
                onValueChange={(next) => {
                  setFilter(next);
                  setPage(0);
                }}
              >
                <TabsList variant="line">
                  <TabsTrigger value="all">Semua scope</TabsTrigger>
                  <TabsTrigger value="active">Aktif</TabsTrigger>
                  <TabsTrigger value="planned">Direncanakan</TabsTrigger>
                  <TabsTrigger value="unknown">
                    Belum terverifikasi ({unknownCount})
                  </TabsTrigger>
                </TabsList>
              </Tabs>
            )}
            <div className="access-toolbar">
              <label htmlFor={`${controlId}-search`}>
                <span>
                  <Search size={14} /> Cari scope
                </span>
                <Input
                  id={`${controlId}-search`}
                  placeholder="Produk, read_orders, pelanggan…"
                  value={query}
                  onChange={(e) => {
                    setQuery(e.target.value);
                    setPage(0);
                  }}
                />
              </label>
              <label htmlFor={`${controlId}-filter`}>
                <span>Tampilkan</span>
                <NativeSelect
                  id={`${controlId}-filter`}
                  value={filter}
                  onChange={(e) => {
                    setFilter(e.target.value);
                    setPage(0);
                  }}
                >
                  <NativeSelectOption value="all">
                    Semua scope
                  </NativeSelectOption>
                  {onChange && (
                    <NativeSelectOption value="selected">
                      Dipilih ({selected})
                    </NativeSelectOption>
                  )}
                  <NativeSelectOption value="restricted">
                    Perlu review khusus
                  </NativeSelectOption>
                  <NativeSelectOption value="planned">
                    Plan — belum aktif
                  </NativeSelectOption>
                  <NativeSelectOption value="active">
                    Active — terverifikasi
                  </NativeSelectOption>
                  <NativeSelectOption value="unknown">
                    Belum terverifikasi
                  </NativeSelectOption>
                  <NativeSelectOption value="reference_only">
                    Khusus Shopify
                  </NativeSelectOption>
                  <NativeSelectOption value="future_reference">
                    Versi mendatang
                  </NativeSelectOption>
                </NativeSelect>
              </label>
              {adminCatalog && (
                <>
                  <label htmlFor={`${controlId}-resource`}>
                    <span>Resource</span>
                    <NativeSelect
                      id={`${controlId}-resource`}
                      aria-label="Resource"
                      value={resource}
                      onChange={(e) => {
                        setResource(e.target.value);
                        setPage(0);
                      }}
                    >
                      <NativeSelectOption value="all">
                        Semua resource
                      </NativeSelectOption>
                      {[...new Set(catalog.scopes.map((s) => s.resource))]
                        .sort()
                        .map((name) => (
                          <NativeSelectOption key={name} value={name}>
                            {name}
                          </NativeSelectOption>
                        ))}
                    </NativeSelect>
                  </label>
                  <label htmlFor={`${controlId}-action`}>
                    <span>Akses</span>
                    <NativeSelect
                      id={`${controlId}-action`}
                      aria-label="Akses"
                      value={action}
                      onChange={(e) => {
                        setAction(e.target.value);
                        setPage(0);
                      }}
                    >
                      <NativeSelectOption value="all">
                        Semua akses
                      </NativeSelectOption>
                      <NativeSelectOption value="read">Baca</NativeSelectOption>
                      <NativeSelectOption value="write">
                        Tulis
                      </NativeSelectOption>
                    </NativeSelect>
                  </label>
                </>
              )}
            </div>
            <div className="access-verification">
              <div>
                <p>
                  {catalog.scopes.length} scope · {results.length} ditampilkan ·{' '}
                  {verification
                    ? `${activeCount} aktif terverifikasi`
                    : 'status belum terverifikasi'}
                </p>
                <p className="access-meta">
                  Snapshot referensi: {catalog.checkedAt}. Pemeriksaan:{' '}
                  {verification
                    ? new Date(verification.checkedAt).toLocaleString('id-ID')
                    : 'belum tersedia'}
                  .
                </p>
              </div>
              <Button
                type="button"
                variant="outline"
                disabled={checking}
                onClick={refreshStatus}
              >
                {checking ? 'Memeriksa…' : 'Periksa ulang status'}
              </Button>
            </div>
            <p className="access-meta">
              Pusat status scope · lingkungan lokal. Dokumentasi endpoint
              memakai sumber verifikasi yang sama. Pemeriksaan inventaris
              implementasi App Platform, bukan uji koneksi atau health check
              gateway Core.{' '}
              {verification?.coreChecked
                ? 'Bukti Core tersedia.'
                : 'Backend Core belum diverifikasi.'}
            </p>
            {verificationError && (
              <p className="access-error" role="alert">
                {verificationError}
              </p>
            )}
            {validation && (
              <p className="access-error" role="alert">
                {validation}
              </p>
            )}
            <div className="access-table">
              <Table aria-label="Katalog scope dan status implementasi">
                <TableCaption>
                  Active memerlukan implementasi dan bukti verifikasi lengkap;
                  Plan belum memberikan akses.
                </TableCaption>
                <TableHeader>
                  <TableRow>
                    <TableHead>Scope</TableHead>
                    <TableHead>Resource</TableHead>
                    <TableHead>Akses</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Endpoint terkait</TableHead>
                    <TableHead>Detail & verifikasi</TableHead>
                    {onChange && <TableHead>Kebutuhan app</TableHead>}
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map((s) => {
                    const state = verifiedScopeState(
                      catalog,
                      verification,
                      s.handle,
                    );
                    const report =
                      verification?.profile === catalog.profile
                        ? verification.scopes.find((v) => v.handle === s.handle)
                        : undefined;
                    return (
                      <TableRow key={s.handle}>
                        <TableCell>
                          <code>{s.handle}</code>
                        </TableCell>
                        <TableCell>{s.resource}</TableCell>
                        <TableCell>
                          {s.action === 'read' ? 'Baca' : 'Tulis'}
                        </TableCell>
                        <TableCell>
                          <span
                            className={`access-state access-state-${state}`}
                          >
                            {state === 'active'
                              ? 'Active'
                              : state === 'planned'
                                ? 'Plan'
                                : 'Belum terverifikasi'}
                          </span>
                        </TableCell>
                        <TableCell>
                          {report?.operations.length ? (
                            report.operations.map((operation) => (
                              <div
                                className="access-operation-link"
                                key={operation}
                              >
                                {api.surface === 'admin' ? (
                                  <a href={gatewayOperationURL(operation)}>
                                    <code>{operation.split('.').at(-1)}</code>
                                  </a>
                                ) : (
                                  <code>{operation.split('.').at(-1)}</code>
                                )}
                                <small>
                                  Endpoint:{' '}
                                  {
                                    readinessLabel[
                                      verifiedOperationState(
                                        verification,
                                        operation,
                                      )
                                    ]
                                  }
                                </small>
                              </div>
                            ))
                          ) : (
                            <small>
                              {verification
                                ? 'Belum ada kontrak endpoint'
                                : 'Mapping belum terverifikasi'}
                            </small>
                          )}
                        </TableCell>
                        <TableCell>
                          <details>
                            <summary>Detail {s.handle}</summary>
                            <div className="access-tags">
                              {s.status !== 'planned' && (
                                <span>
                                  Kategori referensi: {scopeStatus[s.status]}
                                </span>
                              )}
                              {s.review === 'restricted' && (
                                <span>Review khusus Emisell</span>
                              )}
                              {s.availableFrom && (
                                <span>Shopify {s.availableFrom}</span>
                              )}
                            </div>
                            <small>{s.notes}</small>
                            {s.implies.length > 0 && (
                              <small>Mencakup: {s.implies.join(', ')}</small>
                            )}
                            {s.requiresAny.length > 0 && (
                              <small>
                                Memerlukan salah satu:{' '}
                                {s.requiresAny.join(', ')}
                              </small>
                            )}
                            {report && (
                              <>
                                <p>
                                  Kontrak:{' '}
                                  {report.contractStatus === 'partial'
                                    ? 'Parsial'
                                    : report.contractStatus === 'missing'
                                      ? 'Belum tersedia'
                                      : report.contractStatus ===
                                          'reference_only'
                                        ? 'Referensi saja'
                                        : report.contractStatus}
                                </p>
                                <ul>
                                  {report.blockers.map((reason) => (
                                    <li key={reason}>{reason}</li>
                                  ))}
                                </ul>
                              </>
                            )}
                          </details>
                        </TableCell>
                        {onChange && (
                          <TableCell>
                            <label className="access-choice">
                              <span className="sr-only">
                                Kebutuhan {s.handle}
                              </span>
                              <NativeSelect
                                aria-label={`Kebutuhan ${s.handle}`}
                                disabled={
                                  disabled ||
                                  (value !== undefined &&
                                    value.profile !== catalog.profile)
                                }
                                value={
                                  value?.required.includes(s.handle)
                                    ? 'required'
                                    : value?.optional.includes(s.handle)
                                      ? 'optional'
                                      : 'none'
                                }
                                onChange={(e) =>
                                  onChange(
                                    selectScope(
                                      catalog,
                                      value,
                                      s.handle,
                                      e.target.value,
                                    ),
                                  )
                                }
                              >
                                <NativeSelectOption value="none">
                                  Tidak diminta
                                </NativeSelectOption>
                                <NativeSelectOption value="required">
                                  Wajib
                                </NativeSelectOption>
                                <NativeSelectOption value="optional">
                                  Opsional
                                </NativeSelectOption>
                              </NativeSelect>
                            </label>
                          </TableCell>
                        )}
                      </TableRow>
                    );
                  })}
                  {results.length === 0 && (
                    <TableRow>
                      <TableCell
                        colSpan={onChange ? 7 : 6}
                        className="access-empty"
                      >
                        Tidak ada scope yang cocok. Ubah pencarian atau filter.
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>
            {adminCatalog && (
              <div className="scope-pagination">
                <span>
                  Menampilkan {results.length ? currentPage * 6 + 1 : 0}–
                  {Math.min((currentPage + 1) * 6, results.length)} dari{' '}
                  {results.length} scope
                </span>
                <div>
                  <Button
                    variant="outline"
                    aria-label="Halaman scope sebelumnya"
                    disabled={currentPage === 0}
                    onClick={() => setPage(currentPage - 1)}
                  >
                    <ChevronLeft />
                  </Button>
                  <span>
                    {currentPage + 1} / {totalPages}
                  </span>
                  <Button
                    variant="outline"
                    aria-label="Halaman scope berikutnya"
                    disabled={currentPage + 1 >= totalPages}
                    onClick={() => setPage(currentPage + 1)}
                  >
                    <ChevronRight />
                  </Button>
                </div>
              </div>
            )}
          </div>
          <p className="access-meta">
            Profil <code>{catalog.profile}</code>. Storefront dan Customer
            Account memakai profil berbeda, tidak termasuk daftar ini.{' '}
            <a
              href="https://shopify.dev/docs/api/usage/access-scopes"
              target="_blank"
              rel="noreferrer"
            >
              Sumber Shopify ↗
            </a>
          </p>
        </>
      )}
      {adminCatalog && (
        <div className="scope-explainer">
          <ShieldCheck />
          <div>
            <h2>Kapan scope menjadi aktif?</h2>
            <p>
              Active memerlukan implementasi dan bukti verifikasi lengkap.
              Instalasi aplikasi tetap memerlukan grant dari toko; API key tidak
              menggantikannya.
            </p>
            <p>
              Katalog dan dokumentasi memakai sumber verifikasi yang sama.
              Referensi nama izin Shopify bukan kompatibilitas API Shopify.
              Scope pembayaran bukan izin menjadi payment gateway.
            </p>
          </div>
        </div>
      )}
      {onChange && <ScopeSummary value={value} />}
    </section>
  );
}
