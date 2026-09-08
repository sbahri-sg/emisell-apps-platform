import { Links, Meta, NavLink, Outlet, Scripts, useLoaderData, useRouteLoaderData, useRouteError, isRouteErrorResponse } from 'react-router';
import type { LoaderFunctionArgs } from 'react-router';
import type { ReactNode } from 'react';
import './styles/emisell-ui.css';
import './styles/app.css';

export function loader({ context }: LoaderFunctionArgs) {
  const config = context.publicConfig as { name: string; parentOrigin: string; runtime: string };
  return { name: config.name, parentOrigin: config.parentOrigin, runtime: config.runtime, nonce: context.nonce as string };
}
export type AppConfig = ReturnType<typeof loader>;

export function Layout({ children }: { children: ReactNode }) {
  const config = useRouteLoaderData<AppConfig>('root');
  return <html lang="id"><head><meta charSet="utf-8"/><meta name="viewport" content="width=device-width, initial-scale=1"/>
    <link rel="icon" href="/favicon.svg" type="image/svg+xml"/><Meta/>
    {/* Local Vite styles use style-src unsafe-inline; script nonces remain required. */}
    <Links nonce=""/></head><body>{children}<Scripts nonce={config?.nonce}/></body></html>;
}

export default function App() {
  const config = useLoaderData<typeof loader>();
  return <main className="starter-shell">
    <header className="starter-header"><div><p className="eyebrow">EMISELL DEVELOPER</p><h1>{config.name}</h1></div><span className="environment">{config.runtime === 'production' ? 'Hosted app' : config.runtime === 'container-preview' ? 'Container preview · no store access' : 'Local development'}</span></header>
    <nav className="starter-nav" aria-label="Navigasi aplikasi">
      <NavLink to="/" end>Koneksi</NavLink><NavLink to="/products">Produk</NavLink>
      <NavLink to="/resources/orders">Pesanan</NavLink><NavLink to="/resources/shipping">Pengiriman</NavLink>
      <NavLink to="/resources/catalogs">Katalog</NavLink><NavLink to="/resources/collections">Koleksi</NavLink>
      <NavLink to="/resources/inventory">Stok</NavLink><NavLink to="/resources/locations">Lokasi</NavLink>
    </nav>
    <Outlet/>
    <footer>React Router · TypeScript · Vite <span>Data toko hanya dapat diakses setelah persetujuan seller.</span></footer>
  </main>;
}

export function ErrorBoundary() {
  const error = useRouteError();
  return <main className="starter-shell"><h1>{isRouteErrorResponse(error) && error.status === 404 ? 'Halaman tidak ditemukan' : 'Halaman belum dapat dibuka'}</h1>
    <p>Periksa koneksi aplikasi, kemudian muat ulang. Detail dan credential backend tidak ditampilkan.</p><a href="/">Kembali ke koneksi</a></main>;
}
