'use client';

import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Textarea } from '@/components/ui/textarea';
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select';
import type { PortalAPI, Session } from '@/lib/portal';

type Launch = { id: string; revision: number; status: string; binding: { url: string; mode?: 'embedded' | 'external' } };
export default function AppLaunch({ api, session, clientId, ready }: { api: PortalAPI; session: Session; clientId: string; ready: boolean }) {
  const [launch, setLaunch] = useState<Launch | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [mode, setMode] = useState('embedded');
  const [url, setURL] = useState('');
  const [reason, setReason] = useState('');
  const developer = session.user.surface === 'developer';
  const administrator = session.user.surface === 'admin' && session.user.role === 'administrator';
  useEffect(() => {
    let active = true;
    api.request<{ launch: Launch | null }>(`/app-clients/${clientId}/launch`).then(result => {
      if (active) { setLaunch(result.launch); setLoaded(true); }
    }).catch(() => { if (active) setError('Konfigurasi pembukaan aplikasi belum tersedia pada server ini, atau akses ditolak.'); });
    return () => { active = false; };
  }, [api, clientId]);
  async function submit(status?: string) {
    setBusy(true); setError('');
    try {
      const result = status && launch
        ? await api.request<{ launch: Launch }>(`/embedded-launches/${launch.id}/status`, 'POST', { status, revision: launch.revision, reason }, true)
        : await api.request<{ launch: Launch }>('/embedded-launches', 'POST', { clientId, mode, url, reason }, true);
      setLaunch(result.launch); setReason('');
    } catch (e) { setError(e instanceof Error ? e.message : 'Perubahan belum terkonfirmasi.'); }
    finally { setBusy(false); }
  }
  return <section className="portal-panel" aria-label="Pembukaan aplikasi">
    <h2>Pembukaan aplikasi</h2>
    <p>Atur bagaimana seller membuka aplikasi. Persetujuan URL tidak otomatis mengaktifkan akses instalasi.</p>
    {error && <p role="alert">{error}</p>}
    {!loaded && !error && <p>Memuat konfigurasi…</p>}
    {loaded && launch && <>
      <p><strong>{launch.binding.mode === 'external' ? 'Dashboard eksternal' : 'Embedded'}</strong> · {({ submitted: 'Menunggu review', approved: 'Disetujui', rejected: 'Ditolak', revoked: 'Dicabut' } as Record<string, string>)[launch.status] ?? launch.status}</p>
      <p className="break-all">{launch.binding.url}</p>
      <p>Konfigurasi rilis immutable. Perubahan mode atau URL membutuhkan rilis dan review baru. Open app seller belum diaktifkan oleh konfigurasi ini.</p>
    </>}
    {loaded && !launch && developer && <>
      <label htmlFor="launch-mode">Cara membuka aplikasi</label>
      <NativeSelect id="launch-mode" value={mode} onChange={e => setMode(e.target.value)} disabled={busy}>
        <NativeSelectOption value="embedded">Embedded — di dalam Dashboard Emisell</NativeSelectOption>
        <NativeSelectOption value="external">Dashboard eksternal — website aplikasi</NativeSelectOption>
      </NativeSelect>
      <label htmlFor="launch-url">URL aplikasi</label>
      <Input id="launch-url" type="url" value={url} onChange={e => setURL(e.target.value)} placeholder="https://app.example.com/dashboard" disabled={busy} />
      <p>Gunakan HTTPS pada origin endpoint yang sudah diverifikasi, tanpa query atau fragment. Dashboard eksternal tidak otomatis memberikan SSO.</p>
    </>}
    {loaded && !launch && !developer && <p>Developer belum mengajukan konfigurasi pembukaan aplikasi ini.</p>}
    {loaded && ((!launch && developer) || (launch && administrator && ['submitted', 'approved'].includes(launch.status))) && <>
      <label htmlFor="launch-reason">Alasan pengajuan atau keputusan</label>
      <Textarea id="launch-reason" value={reason} maxLength={2000} onChange={e => setReason(e.target.value)} disabled={busy} />
      {!launch && <Button disabled={busy || !ready || !url || !reason.trim()} onClick={() => void submit()}>Ajukan review URL</Button>}
      {launch?.status === 'submitted' && <><Button disabled={busy || !ready || !reason.trim()} onClick={() => void submit('approved')}>Setujui URL</Button><Button variant="outline" disabled={busy || !reason.trim()} onClick={() => void submit('rejected')}>Tolak</Button></>}
      {launch?.status === 'approved' && <Button variant="outline" disabled={busy || !reason.trim()} onClick={() => void submit('revoked')}>Cabut persetujuan URL</Button>}
      {!ready && <p>Pengajuan dan approval memerlukan release signed, bukti endpoint, dan client secret yang masih aktif.</p>}
    </>}
  </section>;
}
