# Emisell Dashboard Admin

Frontend lokal Admin pada `http://localhost:4317/`. Acuan: `codex.md` bagian 1.2–1.3, ADR 0007 dan ADR 0008.

Milestone tersedia: login Admin/reviewer/operator, daftar/filter submission, snapshot immutable, keputusan/feedback/history, signing dan publish/suspend katalog oleh Administrator. Operator hanya-baca. Tidak ada management role/tim, executable self-service, billing atau merchant workspace. Approval bukan publikasi; signature katalog bukan sertifikasi kode. Lihat ADR 0009 dan codex §1.4.

Menu **Rilis integrasi** (`?view=integration-releases`) menambahkan review konfigurasi Aplikasi Integrasi, signing/suspension terpisah, audit serta pemeriksaan integritas/readiness (ADR 0017). Developer mengajukan endpoint/callback/health dari metadata approved. Konfigurasi tetap `installable:false`; tidak mengeksekusi endpoint atau menerbitkan OAuth client/token. Dokumentasi Admin mencakup tujuh endpoint baru dari kontrak `integration-releases.v1.json`.

## Menjalankan

Siapkan akun melalui `go run ./cmd/cli init-portals` dari root setelah meninjau migration/rollout dalam `docs/local-development.md`. Password privat ada di `.local/portals.json`. Akun `owner@emisell.local` tidak memiliki akses Admin.

```sh
npm ci
npm run dev:local
```

Admin memakai `/api/v1/admin/*`, bukan owner/dashboard API legacy. Cookie/audience terpisah dari Developer `4319`; App Store `4318` hanya katalog publik. Akun utama checkout ini `dev@emisell.com`, password privat tidak didokumentasikan. Query lama `view/workspace` tidak memilih surface atau menghidupkan merchant UI. Tidak ada deployment; konfigurasi hosting existing dipertahankan.

## Verifikasi

```sh
npm run typecheck
npm run lint
npm test
npm run build
VITE_EMISELL_LOCAL=true npm run build
```

Primitive, komponen presentasi portal, API client dan tema di sini juga dipakai build Developer. State/sesi tidak dibagikan. Perubahan shared harus diuji pada kedua frontend. Regression tests menjaga source merchant tetap dihapus, entrypoint fixed, audience dan retry-key yang benar. Browser QA mencakup login, draft–review dua surface, desktop/mobile dan error/empty states.
