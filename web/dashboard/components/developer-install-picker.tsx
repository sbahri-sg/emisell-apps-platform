'use client';

import { useEffect, useState } from 'react';
import { Button } from '@/components/ui/button';
import { installSelectionURL } from '@/lib/developer-install-navigation';
import type { PortalAPI } from '@/lib/portal';

// Navigation only: merchant selection and consent belong to Dashboard Emisell.
export default function DeveloperInstallPicker({
  api,
  appId,
  back,
}: {
  api: PortalAPI;
  appId: string;
  back: () => void;
}) {
  const [error, setError] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let current = true;
    api
      .request<{ url: string }>(
        `/apps/${encodeURIComponent(appId)}/install-url`,
      )
      .then((result) => {
        const target = installSelectionURL(result.url, appId);
        if (!target) throw new Error('Invalid install destination');
        if (current) window.location.assign(target);
      })
      .catch(() => {
        if (current) setError(true);
      });
    return () => {
      current = false;
    };
  }, [api, appId, attempt]);
  return (
    <section className="dev-card" aria-busy={!error}>
      <p role={error ? 'alert' : 'status'}>
        {error
          ? 'Dashboard Emisell belum dapat dibuka. Periksa koneksi Apps Platform lalu coba lagi.'
          : 'Membuka Dashboard Emisell untuk memilih toko…'}
      </p>
      {error && (
        <Button
          onClick={() => {
            setError(false);
            setAttempt((value) => value + 1);
          }}
        >
          Coba lagi
        </Button>
      )}
      <Button variant="ghost" onClick={back}>
        Kembali ke aplikasi
      </Button>
    </section>
  );
}
