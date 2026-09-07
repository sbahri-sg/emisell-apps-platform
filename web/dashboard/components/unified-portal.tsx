'use client';
import { useEffect, useState } from 'react';
import Portal from '@/components/portal';
import { PortalError, type Session } from '@/lib/portal';
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
export default function UnifiedPortal() {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
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
  return (
    <Portal
      key={session?.user.id ?? 'login'}
      surface={session?.user.surface ?? 'admin'}
      unifiedLogin={async (email, password) => {
        setSession(await request('login', { email, password }));
      }}
      onSignedOut={() => setSession(null)}
    />
  );
}
