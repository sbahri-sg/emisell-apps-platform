'use client';
import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import type { PortalAPI, Session } from '@/lib/portal';
import { resourceReviewActions } from '@/lib/resource-review';

type Release = {
  id: string;
  revision: number;
  status: string;
  manifest: { requiredScopes: string[]; ui: { name: string; version: string } };
};
const labels: Record<string, string> = {
  submitted: 'Menunggu review',
  approved: 'Review disetujui',
  signed: 'Ditandatangani',
  rejected: 'Ditolak',
  suspended: 'Ditangguhkan',
};
export default function ResourceReleases({
  api,
  session,
}: {
  api: PortalAPI;
  session: Session;
}) {
  const [review, setReview] = useState<{
      release: Release;
      status: string;
    } | null>(null),
    [reason, setReason] = useState(''),
    [saving, setSaving] = useState(false);
  const [rows, setRows] = useState<Release[]>([]),
    [error, setError] = useState('');
  const [loaded, setLoaded] = useState<{
    api: PortalAPI;
    generation: number;
  } | null>(null);
  const [generation, setGeneration] = useState(0);
  const busy = loaded?.api !== api || loaded?.generation !== generation;
  async function decide() {
    if (!review || !reason.trim() || saving) return;
    setSaving(true);
    setError('');
    try {
      await api.request(
        `/ui-resource-releases/${review.release.id}/status`,
        'POST',
        {
          revision: review.release.revision,
          status: review.status,
          reason: reason.trim(),
        },
        true,
      );
      setReview(null);
      setReason('');
      setGeneration((v) => v + 1);
    } catch (e) {
      setError(
        e instanceof Error
          ? e.message
          : 'Review gagal. Perbarui data sebelum mencoba lagi.',
      );
    } finally {
      setSaving(false);
    }
  }
  useEffect(() => {
    let active = true;
    api
      .request<{ releases: Release[] }>('/ui-resource-releases')
      .then((data) => {
        if (active) {
          setRows(data.releases);
          setError('');
        }
      })
      .catch((e) => {
        if (active)
          setError(
            e instanceof Error ? e.message : 'Pengajuan tidak dapat dimuat',
          );
      })
      .finally(() => {
        if (active) setLoaded({ api, generation });
      });
    return () => {
      active = false;
    };
  }, [api, generation]);
  return (
    <section
      className="rounded-xl border bg-white p-5 space-y-4"
      aria-label="Pengajuan izin produk"
    >
      <div className="flex items-center justify-between gap-4">
        <div>
          <h2 className="font-semibold">Aplikasi dengan izin produk</h2>
          <p className="text-sm text-muted-foreground">
            Pengajuan asli dari API. Persetujuan rilis bukan izin akses toko.
          </p>
        </div>
        <Button
          variant="outline"
          disabled={busy}
          onClick={() => setGeneration((v) => v + 1)}
        >
          Perbarui
        </Button>
      </div>
      {busy ? (
        <output>Memuat pengajuan…</output>
      ) : error ? (
        <p role="alert">{error}</p>
      ) : rows.length === 0 ? (
        <p>Belum ada pengajuan izin produk.</p>
      ) : (
        <ul className="space-y-3">
          {rows.map((r) => (
            <li key={r.id} className="rounded-lg border p-4 space-y-2">
              <div className="flex justify-between gap-4">
                <strong>{r.manifest.ui.name}</strong>
                <span>{labels[r.status] || r.status}</span>
              </div>
              <p className="text-sm">
                Versi {r.manifest.ui.version} ·{' '}
                {r.manifest.requiredScopes
                  .map((s) => (s === 'read_products' ? 'Membaca produk' : s))
                  .join(', ')}
              </p>
              <p className="text-sm text-muted-foreground">
                Assignment Testing, persetujuan seller, dan instalasi aktif
                tetap diperlukan untuk akses produk.
              </p>
              <div className="flex gap-2">
                {resourceReviewActions(
                  session.user.surface,
                  session.user.role,
                  r.status,
                ).map((action) => (
                  <Button
                    key={action}
                    variant="outline"
                    disabled={saving}
                    onClick={() => {
                      setReview({ release: r, status: action });
                      setReason('');
                      setError('');
                    }}
                  >
                    {
                      (
                        {
                          approved: 'Setujui review',
                          rejected: 'Tolak',
                          signed: 'Tandatangani',
                          suspended: 'Tangguhkan',
                        } as Record<string, string>
                      )[action]
                    }
                  </Button>
                ))}
              </div>
              <code className="text-xs break-all">{r.id}</code>
            </li>
          ))}
        </ul>
      )}
      {review && (
        <fieldset className="rounded-lg border p-4 space-y-3">
          <legend>
            Konfirmasi: {labels[review.status]} —{' '}
            {review.release.manifest.ui.name}
          </legend>
          <p className="text-sm">
            Tindakan ini tidak memberi persetujuan seller atau akses produk.
          </p>
          <label className="block">
            Alasan review
            <textarea
              className="block w-full border rounded p-2"
              value={reason}
              maxLength={2000}
              disabled={saving}
              onChange={(e) => setReason(e.target.value)}
            />
          </label>
          <div className="flex gap-2">
            <Button
              disabled={saving || !reason.trim()}
              onClick={() => void decide()}
            >
              Konfirmasi
            </Button>
            <Button
              variant="outline"
              disabled={saving}
              onClick={() => setReview(null)}
            >
              Batal
            </Button>
          </div>
        </fieldset>
      )}
    </section>
  );
}
