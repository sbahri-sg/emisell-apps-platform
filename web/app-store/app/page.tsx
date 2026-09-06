"use client";

import { useEffect, useState } from "react";
import { ArrowLeft, ArrowUpRight, Boxes, CreditCard, Search, Truck } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { NativeSelect, NativeSelectOption } from "@/components/ui/native-select";
import { Skeleton } from "@/components/ui/skeleton";
import { ScopeSummary } from "@/components/scope-summary";
import {
  searchCatalog,
  readApp,
  type StoreApp as App,
  type StorePage as Catalog,
} from "../lib/catalog";
import {
  Empty,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
  EmptyDescription,
} from "@/components/ui/empty";

const category = (value: string) => (value === "payment/v1" ? "Pembayaran" : "Pengiriman");
function AppIcon({ capability }: { capability: string }) {
  return (
    <span className={`app-icon ${capability === "payment/v1" ? "payment" : "shipping"}`}>
      {capability === "payment/v1" ? <CreditCard /> : <Truck />}
    </span>
  );
}
export default function Store() {
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [capability, setCapability] = useState("");
  const [page, setPage] = useState(1);
  const [data, setData] = useState<Catalog | null>(null);
  const [selected, setSelected] = useState<App | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    setData(null);
    searchCatalog(query, capability, page, controller.signal)
      .then((value) => {
        if (!controller.signal.aborted) setData(value);
      })
      .catch(() => {
        if (!controller.signal.aborted)
          setError(
            "Katalog belum dapat dimuat. Pastikan layanan platform tersedia, lalu coba lagi.",
          );
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [query, capability, page, reload]);
  const open = async (app: App) => {
    setLoading(true);
    setError("");
    try {
      const value = await readApp(app.id);
      setSelected(value.app);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Detail tidak dapat dimuat.");
    } finally {
      setLoading(false);
    }
  };
  return (
    <div className="store">
      <header className="store-header">
        <a href="/" className="store-brand" aria-label="Emisell App Store">
          <span>e</span>
          <strong>emisell</strong>
          <i />
          App Store
        </a>
        <span className="store-local">
          Katalog lokal <span />
        </span>
      </header>
      <main>
        {error && (
          <div className="store-error" role="alert">
            {error}
            <Button
              variant="outline"
              onClick={() => {
                setSelected(null);
                setReload((v) => v + 1);
              }}
            >
              Coba lagi
            </Button>
          </div>
        )}
        {selected ? (
          <>
            <Button variant="ghost" onClick={() => setSelected(null)}>
              <ArrowLeft />
              Kembali ke katalog
            </Button>
            <div className="store-detail-heading">
              <AppIcon capability={selected.capability} />
              <div>
                <span className="store-kicker">{category(selected.capability)}</span>
                <h1>{selected.name}</h1>
                <p>{selected.summary}</p>
              </div>
            </div>
            <div className="store-detail-grid">
              <article>
                <h2>Tentang aplikasi</h2>
                <p className="store-description">{selected.description}</p>
                <ScopeSummary value={selected.accessScopes} />
                <h2>Scope fixture capability</h2>
                <p>
                  Kontrak reference shipping, bukan grant resource. Listing ini belum dapat
                  diinstal.
                </p>
                <ul className="scope-list">
                  {selected.scopes.map((scope) => (
                    <li key={scope}>
                      <code>{scope}</code>
                    </li>
                  ))}
                </ul>
              </article>
              <aside>
                <span className="free-label">Gratis</span>
                <h2>Informasi aplikasi</h2>
                <dl>
                  <dt>Versi</dt>
                  <dd>{selected.version}</dd>
                  <dt>Capability</dt>
                  <dd>{selected.capability}</dd>
                  <dt>Developer ID</dt>
                  <dd>{selected.developerId}</dd>
                </dl>
                <div className="store-install-note">
                  <strong>Instalasi belum tersedia</strong>
                  <p>
                    Listing ini sudah dipublikasikan. Integrasi runtime dan persetujuan dari Emisell
                    Core masih dalam pengembangan.
                  </p>
                </div>
              </aside>
            </div>
          </>
        ) : (
          <>
            <section className="store-title">
              <p className="store-kicker">BUILT FOR EMISELL</p>
              <h1>
                Temukan aplikasi.
                <br />
                <span>Perluas kemampuan Emisell.</span>
              </h1>
              <p>Jelajahi aplikasi pembayaran dan pengiriman yang dipublikasikan developer.</p>
            </section>
            <form
              className="store-filters"
              onSubmit={(e) => {
                e.preventDefault();
                setPage(1);
                setQuery(search);
              }}
            >
              <div className="store-search">
                <Search aria-hidden="true" />
                <Input
                  aria-label="Cari aplikasi"
                  placeholder="Cari nama atau kegunaan aplikasi…"
                  value={search}
                  maxLength={120}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </div>
              <NativeSelect
                aria-label="Kategori"
                value={capability}
                onChange={(e) => {
                  setCapability(e.target.value);
                  setPage(1);
                }}
              >
                <NativeSelectOption value="">Semua kategori</NativeSelectOption>
                <NativeSelectOption value="shipping/v1">Pengiriman</NativeSelectOption>
              </NativeSelect>
              <Button type="submit" disabled={loading}>
                Cari aplikasi
              </Button>
            </form>
            <div className="store-results-heading">
              <h2>Jelajahi aplikasi</h2>
              {data && <span>{data.total} aplikasi · Semua gratis</span>}
            </div>
            {loading ? (
              <div className="store-grid" aria-label="Memuat katalog">
                {[1, 2, 3].map((v) => (
                  <Skeleton key={v} className="h-60 w-full" />
                ))}
              </div>
            ) : data?.apps.length ? (
              <div className="store-grid">
                {data.apps.map((app) => (
                  <button className="app-card" key={app.id} onClick={() => void open(app)}>
                    <div className="app-card-top">
                      <AppIcon capability={app.capability} />
                      <ArrowUpRight />
                    </div>
                    <span className="app-category">{category(app.capability)}</span>
                    <h3>{app.name}</h3>
                    <p>{app.summary}</p>
                    <div className="app-card-bottom">
                      <span>Gratis</span>
                      <span>v{app.version}</span>
                    </div>
                  </button>
                ))}
              </div>
            ) : (
              !error && (
                <Empty className="store-empty">
                  <EmptyHeader>
                    <EmptyMedia variant="icon">
                      <Boxes />
                    </EmptyMedia>
                    <EmptyTitle>
                      {query || capability
                        ? "Tidak ada aplikasi yang cocok"
                        : "Belum ada aplikasi dipublikasikan"}
                    </EmptyTitle>
                    <EmptyDescription>
                      {query || capability
                        ? "Coba kata pencarian atau kategori lain."
                        : "Aplikasi akan muncul setelah rilis katalog ditandatangani dan dipublikasikan oleh Admin."}
                    </EmptyDescription>
                  </EmptyHeader>
                  {(query || capability) && (
                    <Button
                      variant="outline"
                      onClick={() => {
                        setSearch("");
                        setQuery("");
                        setCapability("");
                        setPage(1);
                      }}
                    >
                      Tampilkan semua
                    </Button>
                  )}
                </Empty>
              )
            )}
            {data && data.total > 20 && (
              <nav className="store-pagination" aria-label="Halaman katalog">
                <Button
                  variant="outline"
                  disabled={loading || page === 1}
                  onClick={() => setPage((v) => v - 1)}
                >
                  Sebelumnya
                </Button>
                <span>
                  Halaman {page} dari {Math.min(500, Math.ceil(data.total / 20))}
                </span>
                <Button
                  variant="outline"
                  disabled={loading || page >= 500 || page * 20 >= data.total}
                  onClick={() => setPage((v) => v + 1)}
                >
                  Berikutnya
                </Button>
              </nav>
            )}
          </>
        )}
      </main>
      <footer>
        Emisell App Platform<span>Katalog metadata · Instalasi melalui Core belum tersedia</span>
      </footer>
    </div>
  );
}
