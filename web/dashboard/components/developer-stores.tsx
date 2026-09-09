'use client';

import { useEffect, useState } from 'react';
import { ArrowUpRight, RefreshCw, Search, Store } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { startMerchantLogin } from '@/components/developer-account';
import {
  developerStoreURL,
  searchDeveloperStores,
  type DeveloperAccountData,
} from '@/lib/developer-stores';
import type { PortalAPI } from '@/lib/portal';

export default function DeveloperStores({ api }: { api: PortalAPI }) {
  const [account, setAccount] = useState<DeveloperAccountData | null>(null);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [attempt, setAttempt] = useState(0);
  const [refreshing, setRefreshing] = useState(false);
  useEffect(() => {
    let current = true;
    api
      .request<DeveloperAccountData>('/account')
      .then((value) => {
        if (current) {
          setAccount(value);
          setError('');
        }
      })
      .catch(() => {
        if (current) setError('Daftar toko belum dapat dimuat. Coba lagi.');
      });
    return () => {
      current = false;
    };
  }, [api, attempt]);
  const stores = searchDeveloperStores(account?.profile?.stores ?? [], query);
  return (
    <section className="dev-stores">
      <div className="page-heading">
        <div>
          <h1>Stores</h1>
          <p>Toko yang dapat Anda kelola dengan akun Emisell.</p>
        </div>
        <Button
          variant="outline"
          disabled={refreshing}
          onClick={() => {
            setRefreshing(true);
            void startMerchantLogin().catch(() => {
              setError('Daftar toko belum dapat diperbarui. Coba lagi.');
              setRefreshing(false);
            });
          }}
        >
          <RefreshCw />
          {refreshing ? 'Memperbarui…' : 'Perbarui toko'}
        </Button>
      </div>
      <div className="dev-search">
        <Search />
        <Input
          aria-label="Cari toko"
          placeholder="Cari nama atau alamat toko"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
        />
      </div>
      {error ? (
        <div className="portal-message error" role="alert">
          {error}
          <Button
            variant="outline"
            onClick={() => {
              setError('');
              setAttempt((value) => value + 1);
            }}
          >
            Coba lagi
          </Button>
        </div>
      ) : !account ? (
        <Skeleton className="h-40 w-full" aria-label="Memuat toko" />
      ) : !account.profile ? (
        <div className="dev-empty">
          <Store />
          <h2>Perbarui sesi merchant</h2>
          <p>
            Gunakan Perbarui toko untuk memeriksa toko yang dapat Anda kelola.
          </p>
        </div>
      ) : (
        <div className="dev-directory-table">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Toko</TableHead>
                <TableHead>Akses</TableHead>
                <TableHead>
                  <span className="sr-only">Tindakan</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {stores.map((store) => {
                const home = developerStoreURL(account, store.id);
                const apps = developerStoreURL(account, store.id, 'apps');
                return (
                  <TableRow key={store.id}>
                    <TableCell>
                      <div className="dev-directory-store">
                        <span className="dev-store-avatar" aria-hidden="true">
                          <Store size={16} />
                        </span>
                        <div>
                          <strong>{store.name}</strong>
                          <span>
                            {home
                              ? new URL(home).host + new URL(home).pathname
                              : store.commonId}
                          </span>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <span className="dev-access-label">Kelola aplikasi</span>
                    </TableCell>
                    <TableCell>
                      <div className="dev-directory-actions">
                        {home && (
                          <a href={home}>
                            Buka toko <ArrowUpRight size={14} />
                          </a>
                        )}
                        {apps && (
                          <a href={apps}>
                            Apps di toko <ArrowUpRight size={14} />
                          </a>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
              {!stores.length && (
                <TableRow>
                  <TableCell colSpan={3}>
                    <div className="dev-empty">
                      <Store />
                      <h2>
                        {query ? 'Toko tidak ditemukan' : 'Belum ada toko'}
                      </h2>
                      <p>
                        {query
                          ? 'Coba nama atau alamat toko lainnya.'
                          : 'Perbarui sesi merchant untuk memeriksa akses toko Anda.'}
                      </p>
                    </div>
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      )}
      <p className="dev-footnote">
        Daftar berasal dari pemeriksaan akses saat login merchant. Membuka toko
        tidak memasang aplikasi atau memberikan izin data.
      </p>
    </section>
  );
}
