# Emisell App Store lokal

Frontend terpisah `http://localhost:4318/`; backend Go tetap `8087`. Acuan codex §1.2–1.4 dan ADR 0009.

Listing/detail publik hanya dari signed catalog package berstatus published dengan verifikasi integritas. Pencarian, capability filter dan pagination 20 item berjalan server-side. Semua gratis dan belum dapat di-install. Tidak ada merchant workspace, portal login, endpoint integrasi, review feedback atau tenant data.

```sh
npm ci
npm run dev
```

Backend membutuhkan migration 0008 dan key dari `go run ./cmd/cli init-catalog`. Ikuti rollout `docs/local-development.md`. Tidak ada deploy/cloud/domain baru. Proxy hanya `/api/v1/store`, fetch selalu `credentials:omit`. Memakai dependency/primitive/token yang sama dengan frontend existing, tanpa framework baru.

```sh
npm run typecheck
npm run lint
npm test
npm run build
```

Test mencakup proxy/public client isolation, query encoding, error/withdrawn response dan input validation. Browser QA mencakup listing, detail, search/category empty state, suspend/republish, serta desktop/mobile tanpa overflow. Listing QA lokal bukan aplikasi production.

ADR 0022: katalog publik hanya menawarkan shipping/v1 saat ini. Payment gateway internal Emisell, tidak tampil di listing/detail publik; data published historis tetap disimpan.
