'use client';

import { useEffect, useRef, useState, useSyncExternalStore } from 'react';
import { Check, Code2, LogOut, RefreshCw, UserRound } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import type { PortalAPI, Session } from '@/lib/portal';

export async function startMerchantLogin() {
  const response = await fetch('/api/v1/developer-login/start', {
    method: 'POST',
    credentials: 'same-origin',
    cache: 'no-store',
    headers: { 'Content-Type': 'application/json' },
    body: '{}',
    signal: AbortSignal.timeout(12000),
  });
  if (!response.ok)
    throw new Error(
      'Login Emisell belum dapat dimulai. Periksa koneksi platform.',
    );
  const data = (await response.json()) as { authorizeUrl: string };
  const url = new URL(data.authorizeUrl);
  if (
    !['http:', 'https:'].includes(url.protocol) ||
    url.pathname !== '/auth/developer'
  )
    throw new Error('Alamat login tidak valid.');
  window.location.assign(url.href);
}

const loginLocationSubscription = (notify: () => void) => {
  window.addEventListener('popstate', notify);
  return () => window.removeEventListener('popstate', notify);
};
export function MerchantLoginButton() {
  const started = useRef(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const failed = useSyncExternalStore(
    loginLocationSubscription,
    () => new URLSearchParams(window.location.search).get('login') === 'failed',
    () => false,
  );
  useEffect(() => {
    if (
      started.current ||
      new URLSearchParams(window.location.search).get('login') === 'failed'
    )
      return;
    started.current = true;
    setBusy(true);
    void startMerchantLogin().catch((error) => {
      setError(error.message);
      setBusy(false);
    });
  }, []);
  return (
    <div className="dev-merchant-login">
      <Button
        disabled={busy}
        onClick={() => {
          setBusy(true);
          setError('');
          void startMerchantLogin().catch((error) => {
            setError(error.message);
            setBusy(false);
          });
        }}
      >
        <UserRound />
        {busy ? 'Menghubungkan…' : 'Masuk dengan akun Emisell'}
      </Button>
      {error && <p role="alert">{error}</p>}
      {!error && failed && (
        <p role="alert">
          Login belum selesai atau tautan kedaluwarsa. Coba lagi dengan akun
          Emisell.
        </p>
      )}
    </div>
  );
}

type Account = {
  profile: null | {
    email: string;
    name: string;
    stores: { id: string; name: string; commonId: string }[];
  };
  sellerOrigin: string;
};

function initials(name: string) {
  return (
    name
      .trim()
      .split(/\s+/)
      .slice(0, 2)
      .map((part) => Array.from(part)[0] || '')
      .join('')
      .toLocaleUpperCase() || 'E'
  );
}

export default function DeveloperAccount({
  api,
  session,
  busy,
  logout,
  navigate,
}: {
  api: PortalAPI;
  session: Session;
  busy: boolean;
  logout: () => void;
  navigate: () => void;
}) {
  const [account, setAccount] = useState<Account | null>(null);
  const [error, setError] = useState('');
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let alive = true;
    api
      .request<Account>('/account')
      .then((data) => {
        if (alive) {
          setAccount(data);
          setError('');
        }
      })
      .catch(() => {
        if (alive) setError('Informasi akun belum dapat dimuat.');
      });
    return () => {
      alive = false;
    };
  }, [api, attempt]);
  const profile = account?.profile;
  const name = profile?.name || session.user.email;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={<Button variant="ghost" className="dev-account-trigger" />}
        aria-label="Menu akun dan toko"
      >
        <span className="dev-account-trigger-icon" aria-hidden="true">
          <Code2 />
        </span>
        <span className="dev-account-trigger-name" title={name}>
          {name}
        </span>
      </DropdownMenuTrigger>
      <DropdownMenuContent
        className="dev-account-menu"
        align="end"
        sideOffset={4}
      >
        <DropdownMenuGroup>
          <DropdownMenuLabel className="dev-account-heading">
            {name}
          </DropdownMenuLabel>
          <DropdownMenuItem
            onClick={navigate}
            className="dev-account-current"
            aria-current="page"
          >
            <Code2 />
            Dev Dashboard
            <Check className="ml-auto" />
          </DropdownMenuItem>
        </DropdownMenuGroup>
        <DropdownMenuGroup>
          <DropdownMenuLabel>Toko Anda</DropdownMenuLabel>
          {profile?.stores.map((store, index) => (
            <DropdownMenuItem
              key={store.id}
              className="dev-account-store"
              title={store.name}
              onClick={() => {
                const target = new URL(account!.sellerOrigin);
                if (!['http:', 'https:'].includes(target.protocol)) return;
                target.pathname = `/store/${encodeURIComponent(store.commonId)}`;
                window.location.assign(target.href);
              }}
            >
              <span
                className="dev-store-avatar"
                data-tone={index % 3}
                aria-hidden="true"
              >
                {initials(store.name)}
              </span>
              <span className="dev-account-store-name">{store.name}</span>
            </DropdownMenuItem>
          ))}
          {!account && !error && (
            <p className="dev-account-help">Memuat akun…</p>
          )}
          {error && (
            <>
              <p className="dev-account-help" role="alert">
                {error}
              </p>
              <DropdownMenuItem onClick={() => setAttempt((v) => v + 1)}>
                Coba lagi
              </DropdownMenuItem>
            </>
          )}
          {account && !profile && (
            <>
              <p className="dev-account-help">
                Sesi merchant perlu diperbarui untuk menampilkan toko.
              </p>
              <DropdownMenuItem
                onClick={() =>
                  void startMerchantLogin().catch((error) =>
                    setError(error.message),
                  )
                }
              >
                <UserRound />
                Masuk kembali dengan akun Emisell
              </DropdownMenuItem>
            </>
          )}
          {profile && (
            <DropdownMenuItem
              className="dev-account-refresh"
              onClick={() =>
                void startMerchantLogin().catch((error) =>
                  setError(error.message),
                )
              }
            >
              <RefreshCw />
              Perbarui daftar toko
            </DropdownMenuItem>
          )}
        </DropdownMenuGroup>
        <DropdownMenuSeparator />
        <div className="dev-account-identity">
          <span
            className="dev-store-avatar dev-account-user-avatar"
            aria-hidden="true"
          >
            {initials(name)}
          </span>
          <div>
            <strong title={name}>{name}</strong>
            <span title={profile?.email || session.user.email}>
              {profile?.email || session.user.email}
            </span>
          </div>
        </div>
        <DropdownMenuItem disabled={busy} onClick={logout}>
          <LogOut />
          Log out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
