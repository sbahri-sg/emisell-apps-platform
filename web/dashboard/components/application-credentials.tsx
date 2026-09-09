'use client';

import Link from 'next/link';
import { useEffect, useRef, useState } from 'react';
import { ArrowUpRight, Copy, Eye, EyeOff, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel,
  AlertDialogAction,
} from '@/components/ui/alert-dialog';
import { PortalError, type PortalAPI } from '@/lib/portal';

type Credential = {
  clientId: string;
  appId: string;
  version: number;
  createdAt: string;
  secretCreatedAt: string;
};
export default function ApplicationCredentials({
  appId,
  api,
  busy,
}: {
  appId: string;
  api: PortalAPI;
  busy: boolean;
}) {
  const [credential, setCredential] = useState<Credential | null>(null);
  const [secret, setSecret] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [working, setWorking] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const generation = useRef(0);
  const mounted = useRef(false);
  const path = `/apps/${appId}/credentials`;
  useEffect(() => {
    mounted.current = true;
    const invalidate = () => {
      generation.current++;
    };
    const hide = () => {
      if (document.hidden) {
        generation.current++;
        setSecret('');
        setWorking(false);
        setConfirm(false);
      }
    };
    document.addEventListener('visibilitychange', hide);
    return () => {
      mounted.current = false;
      invalidate();
      document.removeEventListener('visibilitychange', hide);
    };
  }, []);
  useEffect(() => {
    let active = true;
    api
      .request<{ credential: Credential }>(path)
      .then(({ credential: c }) => {
        if (active) {
          if (c.appId !== appId)
            throw new Error('Identitas aplikasi tidak cocok.');
          setCredential(c);
          setError('');
        }
      })
      .catch((e: unknown) => {
        if (active)
          setError(
            e instanceof PortalError && e.status === 404
              ? 'Credential aplikasi belum dimigrasikan. Hubungi administrator platform.'
              : 'Credential belum dapat dimuat. Coba muat ulang.',
          );
      });
    return () => {
      active = false;
    };
  }, [api, appId, path, attempt]);
  useEffect(() => {
    if (!secret) return;
    const timer = setTimeout(() => setSecret(''), 60000);
    return () => clearTimeout(timer);
  }, [secret]);
  useEffect(() => {
    if (!notice) return;
    const timer = setTimeout(() => setNotice(''), 4000);
    return () => clearTimeout(timer);
  }, [notice]);
  const run = async (action: 'reveal' | 'copy-secret' | 'rotate') => {
    if (!credential || working || busy) return;
    const token = ++generation.current;
    const current = () => mounted.current && token === generation.current;
    setWorking(true);
    setError('');
    setNotice('');
    if (action === 'rotate') setSecret('');
    try {
      if (action === 'rotate') {
        const r = await api.request<{ credential: Credential }>(
          `${path}/rotate`,
          'POST',
          { version: credential.version },
          true,
        );
        if (current()) {
          setCredential(r.credential);
          setNotice('Secret dirotasi. Perbarui konfigurasi aplikasi Anda.');
          setConfirm(false);
        }
      } else {
        const r = await api.request<{ credential: Credential; secret: string }>(
          `${path}/reveal`,
          'POST',
          { version: credential.version },
        );
        if (!current()) return;
        if (action === 'reveal') setSecret(r.secret);
        else {
          await navigator.clipboard.writeText(r.secret);
          if (current()) setNotice('Secret disalin.');
        }
      }
    } catch (e: unknown) {
      if (current()) {
        setSecret('');
        if (e instanceof PortalError && e.status === 409) {
          setCredential(null);
          setAttempt((v) => v + 1);
          setError('Credential berubah. Data dimuat ulang; coba lagi.');
        } else
          setError(
            'Tindakan belum berhasil. Coba lagi; untuk menyalin, izinkan akses clipboard.',
          );
      }
    } finally {
      if (current()) setWorking(false);
    }
  };
  const copyID = async () => {
    if (!credential) return;
    const token = generation.current;
    try {
      await navigator.clipboard.writeText(credential.clientId);
      if (mounted.current && token === generation.current)
        setNotice('Client ID disalin.');
    } catch {
      if (mounted.current && token === generation.current)
        setError('Pilih teks Client ID lalu salin secara manual.');
    }
  };
  return (
    <section
      className="dev-settings-card dev-credentials-card"
      aria-labelledby="settings-credentials"
    >
      <div className="dev-settings-card-heading">
        <h2 id="settings-credentials">Credentials</h2>
        <Link href="/docs/credentials">
          Docs <ArrowUpRight />
        </Link>
      </div>
      {error && (
        <p role="alert" className="dev-settings-help">
          {error}
        </p>
      )}
      {!credential ? (
        <>
          {!error && <Skeleton className="h-32 w-full" />}
          {error && (
            <Button
              variant="outline"
              onClick={() => {
                setError('');
                setAttempt((v) => v + 1);
              }}
            >
              <RefreshCw />
              Muat ulang
            </Button>
          )}
        </>
      ) : (
        <dl className="dev-credential-rows">
          <div className="dev-credential-row">
            <dt>Client ID</dt>
            <dd id="settings-client-id" className="dev-credential-value">
              <code>{credential.clientId}</code>
            </dd>
            <dd className="dev-credential-actions">
              <Button
                size="icon"
                variant="ghost"
                disabled={busy || working}
                aria-label="Salin Client ID"
                onClick={() => void copyID()}
              >
                <Copy />
              </Button>
            </dd>
          </div>
          <div className="dev-credential-row">
            <dt>Secret</dt>
            <dd id="settings-client-secret" className="dev-credential-value">
              <code aria-label={secret ? undefined : 'Secret disembunyikan'}>
                {secret || '••••••••••••••••••••••••'}
              </code>
              <p className="dev-credential-created">
                Dibuat{' '}
                {new Date(credential.secretCreatedAt).toLocaleString('id-ID')}
              </p>
            </dd>
            <dd className="dev-credential-actions">
              <Button
                size="icon"
                variant="ghost"
                disabled={busy || working}
                aria-label={secret ? 'Sembunyikan Secret' : 'Tampilkan Secret'}
                onClick={() => (secret ? setSecret('') : void run('reveal'))}
              >
                {secret ? <EyeOff /> : <Eye />}
              </Button>
              <Button
                size="icon"
                variant="ghost"
                disabled={busy || working}
                aria-label="Salin Secret"
                onClick={() => void run('copy-secret')}
              >
                <Copy />
              </Button>
              <Button
                className="dev-credential-rotate"
                variant="destructive"
                disabled={busy || working}
                onClick={() => setConfirm(true)}
              >
                Rotate
              </Button>
            </dd>
          </div>
        </dl>
      )}
      <output className="dev-settings-copy-message">{notice}</output>
      <AlertDialog
        open={confirm}
        onOpenChange={(open) => {
          if (!working) setConfirm(open);
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Rotasi Secret aplikasi?</AlertDialogTitle>
            <AlertDialogDescription>
              Secret lama langsung tidak berlaku. Client ID tetap sama. Perbarui
              Secret pada backend aplikasi setelah rotasi; izin toko tidak
              berubah.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={working}>Batal</AlertDialogCancel>
            <AlertDialogAction
              disabled={working}
              onClick={() => void run('rotate')}
            >
              Rotasi Secret
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  );
}
