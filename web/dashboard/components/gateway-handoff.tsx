import { Button } from '@/components/ui/button';
import { gateway } from '@/lib/api-docs';
import { scopeCatalogURL } from '@/lib/scope-readiness';
import type { ScopeVerification } from '@/lib/access-scopes';

export default function GatewayHandoff({
  verification,
  checking,
  error,
  refreshStatus,
}: {
  verification: ScopeVerification | null;
  checking: boolean;
  error: string;
  refreshStatus: () => void;
}) {
  const compatible =
    verification?.contractRevision === gateway.contractRevision;
  return (
    <section className="gateway-handoff" aria-label="Handoff gateway Emisell">
      <div className="gateway-heading">
        <div>
          <h2>Kontrak endpoint Gateway Emisell</h2>
          <p>
            Platform → backend Core · status berasal dari verifikasi yang sama
            dengan Katalog Scope.
          </p>
        </div>
        <Button variant="outline" disabled={checking} onClick={refreshStatus}>
          {checking ? 'Memeriksa…' : 'Periksa ulang status'}
        </Button>
      </div>
      <p>
        <a href={scopeCatalogURL()}>Buka Katalog Scope →</a> untuk seluruh
        daftar izin, cakupan implementasi dan kendala. Halaman ini menjelaskan
        request/response dan izin per endpoint; tidak menjalankan request
        resource.
      </p>
      <p className="access-meta">
        Lingkungan: lokal ·{' '}
        {verification
          ? 'Diperiksa ' +
            new Date(verification.checkedAt).toLocaleString('id-ID')
          : 'Belum terverifikasi'}
        . Inventaris build Platform, bukan health check Core.{' '}
        {verification?.coreChecked
          ? 'Bukti Core tersedia.'
          : 'Backend Core belum diverifikasi.'}
      </p>
      {error && (
        <p role="alert" className="access-error">
          {error}
        </p>
      )}
      {verification && !compatible && (
        <p role="alert" className="access-error">
          Versi kontrak dokumentasi berbeda dari backend. Status endpoint belum
          terverifikasi; perbarui dokumentasi, jangan memakai status snapshot.
        </p>
      )}
      <details className="gateway-checklist">
        <summary>Checklist handoff untuk tim backend</summary>
        <ol>
          {gateway.checks.map((c) => (
            <li key={c}>{c}</li>
          ))}
        </ol>
        <p>
          Unduh panduan handoff, Protobuf dan snapshot kontrak melalui Sumber
          kontrak. Unduhan bukan laporan status deployment.
        </p>
      </details>
    </section>
  );
}
