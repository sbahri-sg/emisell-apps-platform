# Emisell Docs dan Portal

## Pengelolaan dokumentasi

Sebelas artikel publik ditulis di `content/docs/*.md`, dengan metadata/kategori dan urutan pada `content/docs/navigation.json`. Panduan penulis ada di `content/docs/README.md`. Pencarian isi, heading/daftar isi, estimasi baca, dan navigasi antarartikel dibuat dari sumber yang sama; tidak ada CMS/database baru.

Dev server memantau perubahan Markdown dan memvalidasi ulang. Build menjalankan generator secara otomatis. Gunakan `npm run docs:content` untuk regenerasi manual dan `npm test` untuk memeriksa tautan, markup aman, serta konsistensi konten generated. Parser hanya menjadi dependency tooling. Komponen React merender AST yang diizinkan; HTML/JSX tidak dieksekusi.

Referensi endpoint publik tetap dikurasi mengikuti implementasi Emisell; kontrak internal tidak diekspor otomatis. Tidak ada perubahan endpoint, sesi, role, maupun data seller pada pekerjaan dokumentasi ini.

## Halaman

Entrypoint utama project Apps Platform sekarang:

- `/`: dokumentasi publik, tanpa request sesi atau data organisasi.
- `/docs/getting-started`, `/docs/cli`, `/docs/authentication`, `/docs/scopes`, `/docs/submissions`: panduan developer.
- `/development`: dashboard developer; menu utama Aplikasi, Credential, dan Pengajuan. Konfigurasi rilis/pengujian tetap tersedia dari aplikasi.
- `/admin`: dashboard internal, termasuk dokumentasi kontrak backend.

Kedua dashboard memakai `/api/v1/portal/session` dan `/api/v1/portal/login` yang sudah ada. Peran berasal dari backend, bukan URL. Akun dengan peran berbeda diarahkan melalui tautan ke dashboard yang tepat; endpoint tetap memeriksa audience/organisasi. Tidak ada endpoint bisnis, key, grant, atau konfigurasi origin baru.

Untuk lokal, jalankan backend seperti biasa dan `npm run dev:local` di sini; seluruh halaman utama tersedia pada `http://localhost:4317`. Developer dedicated `4319` tetap tersedia untuk kompatibilitas pengembangan, bukan URL portal utama. Produksi tetap memakai `EMISELL_DASHBOARD_ORIGIN` yang sudah dikonfigurasi; reverse proxy harus meneruskan route frontend `/`, `/docs/*`, `/development`, dan `/admin` ke frontend, sedangkan `/api/v1/*` tetap ke Go HTTP API.

Dokumentasi publik dikurasi dari fitur/template yang tersedia dan tidak mengimpor bundle kontrak admin/Core, credential, atau data organisasi. Navigasi publik tidak menampilkan branding/referensi platform lain. Perubahan ini belum dipublikasikan ke server.

Frontend lokal Admin pada `http://localhost:4317/admin`. Acuan: `codex.md` bagian 1.2–1.3, ADR 0007 dan ADR 0008.

Milestone tersedia: login Admin/reviewer/operator, daftar/filter submission, snapshot immutable, keputusan/feedback/history, signing dan publish/suspend katalog oleh Administrator. Operator hanya-baca. Tidak ada management role/tim, executable self-service, billing atau merchant workspace. Approval bukan publikasi; signature katalog bukan sertifikasi kode. Lihat ADR 0009 dan codex §1.4.

Menu **Rilis integrasi** (`?view=integration-releases`) menambahkan review konfigurasi Aplikasi Integrasi, signing/suspension terpisah, audit serta pemeriksaan integritas/readiness (ADR 0017). Developer mengajukan endpoint/callback/health dari metadata approved. Konfigurasi tetap `installable:false`; tidak mengeksekusi endpoint atau menerbitkan OAuth client/token. Dokumentasi Admin mencakup tujuh endpoint baru dari kontrak `integration-releases.v1.json`.

## Menjalankan

Siapkan akun melalui `go run ./cmd/cli init-portals` dari root setelah meninjau migration/rollout dalam `docs/local-development.md`. Password privat ada di `.local/portals.json`. Akun `owner@emisell.local` tidak memiliki akses Admin.

```sh
npm ci
npm run dev:local
```

Setelah login terpadu, Admin memakai `/api/v1/admin/*`, bukan owner/dashboard API legacy. Backend memeriksa audience admin/developer meskipun portal berbagi domain; App Store `4318` hanya katalog publik. Akun utama checkout ini `dev@emisell.com`, password privat tidak didokumentasikan. Query lama `view/workspace` tidak memilih surface atau menghidupkan merchant UI. Tidak ada deployment; konfigurasi hosting existing dipertahankan.

## Verifikasi

```sh
npm run typecheck
npm run lint
npm test
npm run build
VITE_EMISELL_LOCAL=true npm run build
```

Primitive, komponen presentasi portal, API client dan tema di sini juga dipakai build Developer. State/sesi tidak dibagikan. Perubahan shared harus diuji pada kedua frontend. Regression tests menjaga source merchant tetap dihapus, entrypoint fixed, audience dan retry-key yang benar. Browser QA mencakup login, draft–review dua surface, desktop/mobile dan error/empty states.
