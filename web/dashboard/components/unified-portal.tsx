'use client';
import Link from 'next/link';
import { useEffect, useState } from 'react';
import Portal from '@/components/portal';
import { Button } from '@/components/ui/button';
import { ArrowUpRight, LogOut, ShieldCheck } from 'lucide-react';
import { endPortalSession } from '@/lib/portal-account';
import { PortalError, type Session, type Surface } from '@/lib/portal';
async function request(
  path: 'login' | 'session',
  body?: { email: string; password: string },
): Promise<Session> {
  const r = await fetch(`/api/v1/portal/${path}`, {
    method: body ? 'POST' : 'GET',
    credentials: 'same-origin',
    cache: 'no-store',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
    signal: AbortSignal.timeout(12000),
  });
  if (!r.ok)
    throw new PortalError(
      r.status,
      r.status === 401 ? 'unauthenticated' : 'unknown',
    );
  const data = (await r.json()) as Session;
  if (!data.user || !['admin', 'developer'].includes(data.user.surface))
    throw new Error('Identitas akun tidak valid.');
  return data;
}
export default function UnifiedPortal({
  surface = 'admin',
}: {
  surface?: Surface;
}) {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [switching, setSwitching] = useState(false);
  const [switchError, setSwitchError] = useState('');
  useEffect(() => {
    let alive = true;
    request('session')
      .then((s) => {
        if (alive) setSession(s);
      })
      .catch((e) => {
        if (alive && !(e instanceof PortalError && e.status === 401))
          setError('Tidak dapat memeriksa sesi. Muat ulang halaman.');
      })
      .finally(() => {
        if (alive) setLoading(false);
      });
    return () => {
      alive = false;
    };
  }, []);
  if (loading) return <main className="portal-loading">Memuat dashboard…</main>;
  if (error)
    return (
      <main className="portal-loading" role="alert">
        {error}
      </main>
    );
  // A URL selects a workspace, never the authenticated account's permissions.
  if (session && session.user.surface !== surface)
    return (
      <main className="portal-account-screen">
        <section
          className="portal-account-card"
          aria-labelledby="account-heading"
        >
          <Link href="/" className="portal-account-brand">
            emisell <span>developer platform</span>
          </Link>
          <ShieldCheck className="portal-account-icon" aria-hidden="true" />
          <h1 id="account-heading">
            Masuk dengan akun {surface === 'developer' ? 'developer' : 'admin'}
          </h1>
          <p>
            Sesi yang aktif saat ini menggunakan akun{' '}
            {session.user.surface === 'developer' ? 'developer' : 'admin'}.
            Untuk membuka dashboard ini, gunakan akun dengan peran yang sesuai.
          </p>
          <div className="portal-account-identity">
            <span>Akun aktif</span>
            <strong>{session.user.email}</strong>
          </div>
          {switchError && (
            <p className="portal-account-error" role="alert">
              {switchError}
            </p>
          )}
          <Button
            className="portal-account-switch"
            disabled={switching}
            onClick={async () => {
              if (switching) return;
              setSwitching(true);
              setSwitchError('');
              try {
                await endPortalSession(session.user.surface);
                setSession(null);
              } catch {
                setSwitchError(
                  'Belum dapat mengakhiri sesi. Periksa koneksi, lalu coba lagi.',
                );
              } finally {
                setSwitching(false);
              }
            }}
          >
            <LogOut />
            {switching ? 'Mengakhiri sesi…' : 'Ganti akun'}
          </Button>
          <p className="portal-account-note">
            Sesi aktif akan diakhiri, lalu formulir login ditampilkan.
          </p>
          <Link
            className="portal-account-link"
            href={
              session.user.surface === 'developer' ? '/development' : '/admin'
            }
          >
            Tetap gunakan dashboard{' '}
            {session.user.surface === 'developer' ? 'developer' : 'admin'}{' '}
            <ArrowUpRight aria-hidden="true" />
          </Link>
          <Link className="portal-account-docs" href="/">
            Kembali ke dokumentasi
          </Link>
        </section>
      </main>
    );
  return (
    <Portal
      key={session?.user.id ?? 'login'}
      surface={surface}
      knownSignedOut={!session}
      unifiedLogin={async (email, password) => {
        setSession(await request('login', { email, password }));
      }}
      onSignedOut={() => setSession(null)}
    />
  );
}
