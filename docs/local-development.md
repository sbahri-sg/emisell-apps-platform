# Development lokal Emisell App Platform

**Acuan terbaru: `codex.md` bagian 1.2 dan ADR 0007.** Produk terdiri dari Dashboard Admin (`4317`), App Store (`4318`), dan Portal Developer (`4319`). Tidak ada dashboard/workspace merchant dalam frontend repository ini. Merchant tetap menggunakan Emisell Core.

## Status implementasi

- `web/dashboard` menjalankan login Admin, daftar/filter/detail submission, keputusan reviewer, feedback dan history. UI merchant tetap dihapus.
- `web/developer` menjalankan login Developer, draft aplikasi milik organisasi, simpan revisi, submit, feedback, resubmit, validasi/export metadata dan status rilis katalog. Admin dapat menandatangani/publish/suspend katalog. `web/app-store` pada `4318` menampilkan listing/detail publik, pencarian, kategori dan pagination. Batas milestone: ADR 0008–0009. Onboarding publik, management role/tim, SDK runtime dan executable self-service belum tersedia.
- Backend Go/PostgreSQL, capability, installation, OAuth reference, outbox/inbox NATS, service-account client Core reference, dan tests tetap dipertahankan. Data tenant dan instalasi yang sudah ada tidak dihapus atau dicabut.
- Rollout portal menerapkan migration 0006–0007 dan memperbarui Core consumer sebelum worker/API; schema reference remote juga disiapkan tanpa rotasi credential. Source backend tahap 5 ikut termuat pada runtime lokal, tetapi tahap payment belum selesai secara keseluruhan. Tidak ada simulasi/charge/installation yang dijalankan oleh rollout portal. Lihat ADR 0008.
- Semua app gratis; semua payment/shipping reference adalah simulasi. Tidak ada provider produksi, transaksi asli, WASM, atau deployment cloud.

## Prasyarat dan port

Go 1.26.6+, Node.js 24+, npm, Docker Compose. `go.mod` memerlukan patch Go 1.26.6; `GOTOOLCHAIN=auto` dapat mengambil toolchain tanpa mengganti instalasi global.

| Komponen | Port loopback |
|---|---|
| Dashboard Admin | 4317 |
| App Store | 4318 |
| Portal Developer | 4319 |
| HTTP API | 8087 |
| ConnectRPC internal | 8088 |
| Worker readiness/metrics | 8089 |
| Remote reference app | 8091 |
| PostgreSQL dedicated | 55437 |
| NATS dedicated | 54227 |

Jangan mengekspos port ke jaringan publik. Harness browser merchant `TestOperationsBrowserQA` telah dihapus; port 4319 tidak boleh dipakai ulang untuk harness.

## Menjalankan frontend Admin saat ini

Dari `web/dashboard`:

```sh
npm ci
npm run dev:local
```

Buka `http://localhost:4317/`. Query lama seperti `?view=store&workspace=local-store` tidak memilih workspace atau menghidupkan UI merchant. Admin hanya mengonsumsi `/api/v1/admin/*`. Tidak ada login owner workspace yang dijadikan admin secara otomatis.

Proxy `/api/v1` yang sudah ada dipertahankan untuk compatibility. Portal Developer hanya mem-proxy `/api/v1/developer/*`. Jangan publish build lokal ke Sites.

## Akun portal dan alur draft–review

### Dokumentasi API pada Admin

Buka `http://localhost:4317/?view=api-docs` atau menu **Dokumentasi API** setelah login Admin. Referensi read-only mencakup Admin, Developer, App Store publik, dan Core RPC. Endpoint merchant/workspace legacy tidak termasuk daftar, pencarian, contoh, atau unduhan. Pencarian, parameter/header, auth, schema request/response, contoh struktur dan unduhan sumber tersedia tanpa mengirim request bisnis. Halaman tetap dapat dibuka jika sesi valid tetapi daftar review/katalog gagal dimuat.

Kontrak sumber berada di `api/openapi` dan `api/proto`. Dari `web/dashboard`, jalankan `npm run docs:generate` setelah mengubah kontrak, lalu `npm test` (termasuk `docs:check`). Generator memakai Buf lokal untuk descriptor Protobuf dan komentar field, bukan parsing field manual; contoh tetap placeholder, tanpa credential/data pengguna. Unduhan Protobuf memakai import standar `google/protobuf/timestamp.proto`. Simpan kontrak OpenAPI bersama untuk resolve schema references. Daftar saat ini 74 operasi dari 18 sumber, termasuk tujuh endpoint rilis konfigurasi integrasi, delapan operasi app-client (termasuk self-check), enam lifecycle RPC, enam endpoint Testing portal, satu RPC distribusi pengujian, satu self-check token instalasi, dan dua gateway planned.

Referensi endpoint utama kini memakai **merchantId saja** (ADR 0019), tanpa ID toko kedua. Backend masih menerima alias legacy yang deprecated; kedua nama dengan nilai berbeda ditolak. Unduhan Protobuf mempertahankan alias secara eksplisit demi kompatibilitas, tetapi schema/contoh utama mengecualikannya. Tiga operasi API key tenant-bound legacy tidak lagi masuk referensi utama; endpoint server dan credential lama tidak dihapus. Caller merchant-only memerlukan backend versi baru, sehingga upgrade server sebelum caller; tidak ada migration data atau aktivasi scope resource.

### Rilis konfigurasi Aplikasi Integrasi

Menu **Rilis integrasi** tersedia pada Admin dan Developer (`?view=integration-releases`). Setelah metadata disetujui, developer memvalidasi/mengajukan konfigurasi endpoint, callback OAuth dan health; reviewer memutuskan review konfigurasi secara terpisah; administrator menandatangani atau menangguhkan. Key lokal khusus diprovision eksplisit dengan `go run ./cmd/cli init-integration-signing` setelah migration 0013 dan backup. Baca `docs/integration-releases.md` dan ADR 0017. Konfigurasi signed selalu non-installable; runtime/OAuth umum dan network conformance belum tersedia. Identifier `remote` tidak berubah, nama UI menjadi **Aplikasi Integrasi**.

### Registrasi app-client dan verifikasi origin

Menu **App clients** tersedia pada Admin (`http://localhost:4317/?view=app-clients`) dan Developer (`http://localhost:4319/?view=app-clients`). Setelah konfigurasi signed, developer mendaftarkan client, memasang challenge JSON pada origin HTTPS miliknya, meminta verifikasi, lalu menerbitkan client secret sekali tampil. Administrator dapat memeriksa/revoke; operator/reviewer hanya membaca. Backup privat dan apply migration 0014 sebelum restart backend; signing key integrasi existing dipertahankan, tidak ada auto-provision client/secret.

Baca `docs/app-clients.md` dan ADR 0018 untuk TTL, renewal, SSRF policy dan self-check server-to-server. Browser tidak melakukan probe endpoint developer. Verifier v1 tidak mendukung localhost/private IP/IPv6-only; pengujian network menggunakan fixture TLS terkontrol, bukan pengecualian security pada server live. `clientReady` bukan izin install, token OAuth atau grant resource; seluruh 108 scope resource tetap Plan. Tidak ada deployment cloud atau merchant workspace.

### API key integrasi Core dan status scope

Admin `http://localhost:4317/?view=api-keys`: hanya administrator. Form **hanya nama koneksi**. Key baru full access untuk backend Emisell → layanan internal App Platform, lintas tenant, tanpa tanggal kedaluwarsa; berlaku sampai dicabut. Endpoint management baru `/api/v1/admin/platform-keys` tidak menerima tenantId, validDays atau scopes. Key tidak membuat tenant/workspace, sesi portal, atau app grant. Secret `epk_…` hanya tampil sekali; setelah tersimpan pada secret manager backend, tutup tampilannya. Jika response pertama hilang, retry menampilkan metadata tanpa secret: cabut lalu buat pengganti. Daftar hanya 200 key platform terbaru. Key scoped Admin lama tetap dikelola melalui `/api/v1/admin/api-keys` dan tidak otomatis ditingkatkan; key CLI tidak disentuh. Revoke idempotent; rotasi dengan generate baru, pindahkan consumer, revoke lama. Lihat ADR 0014.

Uji koneksi dari backend dengan generated `emisell.integration.v1.ConnectionService/Check` atau SDK `client.Connection.Check` di listener `8088`, Authorization Bearer, tanpa Origin/cookie. Pada Connect JSON gunakan POST, `Content-Type: application/json`, `Connect-Protocol-Version: 1`, body `{}`. Key platform menghasilkan service ID dan `platformFullAccess:true`; tenant/scopes/expiry absent. Key legacy tetap mengembalikan binding sebelumnya. Payment/shipping dan intent dengan key platform wajib `merchantId` per request setelah Core memeriksa otoritas tenant/actor. Koneksi berhasil tidak mengaktifkan resource Plan atau melewati installation/grant. Jangan memakai key ini di browser atau third-party app.

Katalog Scope adalah pusat status Plan/Active/Belum terverifikasi, filter, detail, endpoint terkait dan pemeriksaan ulang. Dokumentasi endpoint membaca sumber verifikasi yang sama. Endpoint `/api/v1/admin/access-scopes/verification` dan `/api/v1/developer/access-scopes/verification` memeriksa inventaris build: 108 scope dan dua endpoint produk Plan, Core belum diverifikasi, tidak melakukan network probe. Read model memisahkan status operasi dari kelengkapan scope dan menyertakan environment lokal serta fingerprint kontrak. Status active key tidak berarti active scope. Lihat ADR 0013/0015; integrasi gateway/grant belum diaktifkan.

### Handoff tim backend Emisell

Buka `http://localhost:4317/?view=api-docs&api_group=gateway` setelah login Admin, atau klik endpoint pada tabel Katalog Scope. Tidak ada matriks 108 scope kedua di dokumentasi; halaman hanya menampilkan endpoint, scope yang diterima beserta tautan kembali ke katalog, request/response, checklist dan unduhan `emisell-gateway-handoff.md`, `product.proto`, serta `gateway-coverage.v1.generated.json`. Parameter `api_operation` memilih procedure exact; parameter `scope` pada katalog mengisi pencarian. Keduanya bukan authorization.

Untuk implementasi awal, ikuti kontrak List/Get produk dasar, generated Connect interfaces dan validator `pkg/gatewaycontract`. Tim Core dapat memakai `pkg/gatewaycontract/conformance.Run` dengan server/credential/fixture test-nya sendiri; jangan memakai data live. Reference tests repository ini tidak membuktikan gateway Core sungguhan sudah tersedia. Tidak ada perubahan server aktif, credential, grant atau instalasi untuk menampilkan handoff.

Snapshot kontrak dihasilkan dari `go run ./cmd/cli gateway-contract`, digabung dengan Protobuf oleh generator docs. Update sumber Go/proto/Markdown lalu `npm run docs:generate --prefix web/dashboard`; jangan mengedit file generated. Saat membuka halaman atau menekan Periksa ulang status, kesiapan dibaca dari backend, bukan snapshot. Jika backend tidak tersedia atau fingerprint kontrak berbeda, status menjadi Belum terverifikasi. Tidak ada live push atau aktivasi otomatis.

### Katalog scope dan rencana akses data

- Admin: `http://localhost:4317/?view=scopes`; Developer: `http://localhost:4319/?view=scopes`. Katalog 108 handle authenticated mengacu snapshot Shopify 2026-09-05, tidak menyatakan seluruh API Shopify sudah tersedia.
- Buka aplikasi pada Portal Developer → cari scope pada **Akses data aplikasi** → pilih Wajib/Opsional → simpan draft → ajukan review. Pilihan persisten masuk snapshot immutable; perubahan draft berikutnya tidak mengubah pengajuan lama. Implikasi/dependensi dan konflik ditampilkan, lalu divalidasi ulang server-side.
- Admin memeriksa rencana pada detail pengajuan. Tooling/export dan signing menghasilkan katalog v2 bila ada deklarasi resource. Listing publik menampilkan required/optional beserta peringatan belum aktif. Draft lama tanpa deklarasi tetap katalog v1.
- Semua scope baru belum grantable, termasuk setelah review/sign/publish. Token akses resource, API resource gateway dan consume resource intent belum diimplementasikan. Lifecycle fixture ADR 0016 memakai namespace scope terpisah dan tidak mengaktifkan resource Plan.
- Tidak perlu migrasi database: JSONB yang ada menampung field opsional. Reader v2 harus tersedia sebelum authoring. Jangan rollback ke reader lama setelah draft/rilis v2 disimpan tanpa mengikuti ADR 0011.

### Distribusi aplikasi uji

Menu **Testing** pada Admin/Developer (`?view=testing`) memisahkan persetujuan pengujian dari kesiapan instalasi. Developer memilih release signed milik organisasinya, memasukkan merchant ID yang diketahui dan tujuan pengujian. Hanya administrator dapat approve/reject/revoke; approval memerlukan merchant terdaftar. Tidak ada pencarian merchant, instalasi otomatis atau grant dari assignment.

Settings → Apps di Dashboard Core membaca daftar approved melalui facade `GET /v1/app-platform/core/test-apps` dan internal `TestDistributionService/ListAssignments`. Runtime developer masih belum tersedia, sehingga `installable=false`; release/scope readiness tidak disamakan dengan izin akses. Baca ADR 0021. Migration 0015 additive, backup privat sebelum apply, tanpa seed/reset. Dokumentasi API kini mencakup **74 operasi dari 18 sumber**, termasuk enam endpoint Testing portal, satu RPC distribusi, serta enam lifecycle RPC existing.

### Akun dan workflow portal

Dari root, setelah database tersedia dan migration telah ditinjau:

```sh
go run ./cmd/cli init-portals
```

Command ini mengikuti migrator existing (semua migration urut). Untuk upgrade dari 0005, ikuti consumer-first rollout ADR 0008; jangan restart hanya producer di depan consumer whitelist lama.

Credential acak disimpan pada **`.local/portals.json`** (0600, gitignored). Buka file secara lokal, jangan commit atau menyalin password ke frontend/log. `init-portals` dapat diulang tanpa mengubah password, role atau instalasi yang sudah ada.

| Akun lokal | Surface | Akses |
|---|---|---|
| `dev@emisell.com` | Admin utama checkout ini | Review, signing/publish/suspend katalog |
| `reviewer@emisell.local` | Admin | Baca dan putuskan review |
| `operator@emisell.local` | Admin | Hanya-baca |
| `developer@emisell.local` | Developer | Owner organisasi Emisell Developer |
| `developer-two@emisell.local` | Developer | Owner organisasi terpisah Independent Developer |

Akun `owner@emisell.local` tetap milik tenant reference, **bukan** akun portal. Tidak ada pendaftaran publik atau fitur undangan/reset password pada milestone ini; provisioning/recovery harus eksplisit, jangan mengganti file password secara diam-diam.

Fresh provisioning menggunakan default `admin@emisell.local`; checkout ini sudah diubah sesuai permintaan pengguna menjadi `dev@emisell.com`. Penggantian berikutnya memakai `go run ./cmd/cli update-admin <email>` dengan satu baris password melalui stdin yang tidak ditampilkan (misalnya pipe secret manager). Jangan memasukkan password ke argumen atau shell history. Command tidak menjalankan migration; hanya primary Admin, revokasi sesi target, audit dan sinkronisasi file privat. Bila file gagal tersimpan setelah database commit, ulangi input identik untuk rekonsiliasi, bukan `init-portals`. Ganti password yang pernah dibagikan dalam percakapan sebelum production.

Jalankan Developer pada terminal terpisah:

```sh
cd web/developer
npm ci
npm run dev
```

Buka `http://localhost:4319/`. Satu akun developer memiliki satu organisasi pada milestone ini; tidak ada pemilih tenant/store merchant. Data app/submission berasal dari PostgreSQL, bukan browser storage.

Alur uji: login Developer → Buat aplikasi → lengkapi nama, ringkasan, deskripsi, versi, capability dan endpoint HTTPS → Simpan draft → Ajukan review → login reviewer di Admin → buka submission → beri catatan dan minta perbaikan/setujui/tolak. Developer membaca feedback; simpan revisi baru untuk resubmit setelah permintaan perbaikan. Versi approved/rejected memerlukan versi baru.

`approved` hanya keputusan metadata. Tidak menandatangani, memublikasikan, menginstal atau mengeksekusi aplikasi. Endpoint hanya metadata review; contoh `example.invalid` tidak dipanggil. Satu app `QA Portal Shipping` pada organisasi developer kedua adalah data uji lokal/audit, bukan aplikasi siap pakai. API contract: `api/openapi/portals.v1.json`.

## Tooling dan publikasi katalog lokal

Setelah migration 0008 ditinjau dan backup dibuat:

```sh
go run ./cmd/cli migrate
go run ./cmd/cli init-catalog
```

Restart API agar key dimuat. `.local/catalog-signing.json` adalah key privat khusus katalog, bukan fixture key. Command init tidak pernah mengganti key yang sudah ada. Jangan menghapus/merotasinya otomatis; rilis lama memerlukan key yang sama. Tanpa key, signing/publish gagal; suspend tetap tersedia.

Jalankan App Store secara terpisah:

```sh
cd web/app-store
npm ci
npm run dev
```

Store `http://localhost:4318/` hanya memakai `/api/v1/store/apps`; tidak mengirim session, menerima merchant tenant, atau mem-proxy API portal. Semuanya tetap localhost; tidak ada deployment Sites.

Developer → Developer tools → pilih draft tersimpan → Validasi draft → Ekspor metadata. File ini **unsigned, non-executable**, schema `emisell.catalog/v1` tanpa deklarasi resource, atau `emisell.catalog/v2` bila ada `accessScopes`. Administrator → Rilis & publikasi → Validasi & tanda tangani (hanya snapshot approved) → isi alasan → Publikasikan di App Store. Signing tidak otomatis publish. Reviewer/operator tidak dapat melakukan tindakan publikasi.

Detail rilis berisi signed package/public key dan history. CLI offline:

```sh
go run ./cmd/cli catalog-validate <catalog-draft.json>
go run ./cmd/cli catalog-verify <catalog-package.json> <trusted-key.txt>
```

Public key adalah base64 satu baris, didapat melalui jalur pengelola yang dipercaya. Checksum/signature melindungi metadata, bukan memindai kode app. Listing `free`, `installable:false`. Satu versi published per app; suspend versi lama sebelum publish pengganti. Suspend menghapus listing dari pembacaan publik baru, tidak menghapus paket/audit atau menyentuh installation. Cache halaman yang sudah terbuka diperbarui dengan reload. Data QA katalog lokal dipertahankan sebagai contoh, tidak di-install.

Contract: `api/openapi/catalog.v1.json`. Lihat ADR 0009 untuk atomicity, batas pagination, recovery key/file, dan jalur menuju executable releases + S3/MinIO/security scan/consent Core.

## Backend reference yang dipertahankan

Jalankan perintah Go dari root repository. Untuk setup baru atau environment yang memang belum disiapkan:

```sh
go run ./cmd/cli init-events
docker compose -f deploy/compose.local.yaml up -d --wait
go run ./cmd/cli init-local
go run ./cmd/cli init-core
```

`init-local` menjalankan migration dengan checksum, membuat akun reference `owner@emisell.local`, tenant fixture, dan reference release. Credential privat `.local/login.json` berizin 0600 adalah **akun tenant reference, bukan admin/developer**. Jangan menyalin credential ke frontend, commit, atau hosting. Init mempertahankan instalasi/password yang ada. Jika file credential hilang atau token expired, lakukan recovery/rotation eksplisit, jangan menimpa otomatis.

Database development: `emisell_local`, port 55437, user `emisell_local`, password khusus container local `local-development-only`. Database test: `emisell_local_test`. Tidak menyentuh container gateway/payment-proxy atau database lama.

Untuk migration yang telah ditinjau, tanpa reset data:

```sh
go run ./cmd/cli migrate
```

Jalankan masing-masing proses dalam terminal terpisah:

```sh
go run ./cmd/server
```

```sh
go run ./cmd/worker
```

```sh
go run ./cmd/core-reference --operation consume
```

Untuk remote reference yang sudah disetujui/dikonfigurasi:

```sh
go run ./cmd/remote-reference
```

Server menolak database selain `emisell_local` di loopback. Variabel yang tersedia: `EMISELL_DATABASE_URL`, `EMISELL_ADDRESS`, `EMISELL_RPC_ADDRESS`, `EMISELL_ORIGIN`. Jangan mengisi `.env` gateway lama untuk backend ini.

## Kontrak legacy dan integrasi Core

Backend owner-tenant APIs masih tersedia untuk compatibility/reference tests; UI yang memakainya sudah dihapus. `/api/v1/dashboard`, login tenant, workspace/installation APIs, dan operasi merchant bukan kontrak Admin/Developer Portal final. Jangan menyebutnya API admin hanya dengan mengganti label.

Browser consent/setup merchant tidak lagi tersedia. URL OAuth callback reference masih mengikuti kontrak lama; implementasi surface consent yang tepat harus dilakukan bersama integrasi Core. Tests HTTP/OAuth otomatis tetap menguji backend tanpa bergantung pada dashboard merchant.

Capability dipanggil oleh Core melalui ConnectRPC + Protobuf, bukan endpoint provider-specific. Jika installation reference yang sesuai sudah aktif:

```sh
go run ./cmd/core-reference --operation payment-create --key core-payment-order001 --reference order001
go run ./cmd/core-reference --operation shipping-rates --key core-rates-order001
```

Hasil memuat `simulation: true`. Resource lama tetap terikat installation asal; penggantian provider tidak memindahkan resource. Gunakan key/body sama untuk retry yang hasilnya ambigu, key baru untuk operasi berbeda. Jangan mengganti/uninstall instalasi pengguna hanya untuk menjalankan contoh.

Credential Core RPC privat `.local/core.json` berumur 24 jam. Perintah `init-core` mempertahankan credential yang valid. `rotate-core` dan `revoke-core` hanya boleh dipakai atas keputusan eksplisit; lifecycle broker credential terpisah. Core consumer reference hanya untuk tenant-nya, bukan Core produksi.

## Operasional melalui CLI reference

### Backend grant/install intent

RPC `InstallIntentService` tersedia melalui SDK `client.InstallIntents` setelah migration `0009_install_intents.sql`. Operasi `Prepare/Get/Decide` menyimpan persetujuan lokal dengan actor binding, immutable snapshot, TTL intent 10 menit, idempotency dan audit. Key platform (migration `0011_platform_full_access_keys.sql`) wajib `merchantId` per request dari backend Core terotorisasi; key legacy tetap memakai binding credential. TTL intent tidak berubah walaupun credential platform tidak kedaluwarsa. Lihat `docs/core-install-intents.md`, ADR 0010 dan 0014.

Prepare/Get/Decide tetap bukan instalasi/aktivasi dan respons `execution_allowed=false`. Listing katalog tetap tidak installable. Credential legacy tidak mendapat scope baru; jangan merotasi credential live hanya untuk mencoba fitur ini.

### Consume, grant dan token fixture lokal

Migration `0012_consent_installation_access.sql` menambahkan `InstallationService`, SDK `client.Installations`, serta GET `/api/v1/app/installation-access`. Lihat `docs/core-installation-lifecycle.md`/ADR 0016. Consume memerlukan fresh consent dengan installationPolicy baru dan key platform full-access yang sama; grant pending kosong hingga Activate lolos. IssueToken mengeluarkan token app self-check sekali tampil/15 menit; issuance baru mencabut yang lama. Uninstall mencabut token/grant/routing atomik, remote cleanup memakai retry worker existing.

Admin API docs kelompok Core memuat lifecycle; kelompok **Aplikasi · akses lokal** memuat self-check. Tidak ada consent/merchant UI baru, OAuth client production, executable katalog atau gateway resource Active. App token tidak boleh memakai audience Core maupun sesi portal.

Jalankan pengujian `TestConsentInstallation` hanya pada `emisell_local_test`. Tidak ada live app token/key/installation yang dibuat untuk smoke test. Rollout lokal: backup privat → CLI migrate → binary baru. Tabel additive tanpa backfill. Setelah ada intent yang dikonsumsi, jangan rollback binary pra-0012; pertahankan audit/data dan gunakan forward-fix.

### CLI recovery

```sh
go run ./cmd/cli outbox-status
go run ./cmd/cli webhook-status <tenant-id>
go run ./cmd/cli replay-event capability <event-id> 'broker sudah pulih dan penyebab diperiksa'
go run ./cmd/cli replay-webhook <tenant-id> <delivery-id> 'receiver sudah pulih dan penyebab diperiksa'
go run ./cmd/cli retry-cleanup <tenant-id> <installation-id> 'pencabutan akses remote siap dicoba ulang'
```

Replay/recovery hanya untuk target exact yang terverifikasi dan scope operator yang diizinkan; alasan disimpan sebagai audit tanpa secret/PII. Jangan menjalankannya sebagai bagian removal frontend. Worker memakai retries bounded dan durable storage; delivery `delivered` bukan status pembayaran. Readiness/metrics ada pada port API/RPC/worker; lihat ADR 0003–0006 untuk kontrak dan batas operasional backend, ADR 0007 menggantikan bagian UI-nya.

## Verifikasi

Frontend:

```sh
cd web/dashboard
npm run typecheck
npm run lint
npm test
npm run build
VITE_EMISELL_LOCAL=true npm run build
```

Jalankan `typecheck`, `lint`, `test`, dan `build` juga dari `web/developer`. Tests shared portal memeriksa API audience, retry key, permission baseline dan penolakan state merchant. Browser QA mencakup login, draft, review, resubmit, feedback, filter, reload, serta desktop/mobile; UI tidak boleh mengambil data merchant.

Backend, dari root:

```sh
make tools
EMISELL_TEST_DATABASE_URL='postgres://emisell_local:local-development-only@127.0.0.1:55437/emisell_local_test?sslmode=disable' EMISELL_NATS_SERVER="$PWD/bin/nats-server" make verify
```

`make tools` memasang Buf/nats-server terpin di `bin/`, bukan instalasi global. `make verify` memeriksa kontrak Buf/breaking baseline, race tests, vet, dan build. Integration tests membutuhkan DB terpisah `emisell_local_test`; broker test memakai credential/port/storage sementara. Tanpa variabel yang diperlukan, tests terkait skipped, bukan terverifikasi. Test tidak menghentikan broker development atau menghapus instalasi pengguna.

## Menghentikan

Ctrl+C pada proses milik increment lokal yang ingin dihentikan. Untuk PostgreSQL/NATS dedicated:

```sh
docker compose -f deploy/compose.local.yaml stop
```

Volume dipertahankan. Jangan gunakan `down -v` atau menghapus schema/data untuk membersihkan UI merchant. Tidak ada container lama lain yang termasuk scope ini.

## Payment internal dan distribusi aplikasi umum

ADR 0022/codex §1.18: authoring dan distribusi baru hanya shipping/v1. Payment gateway checkout dikelola internal Emisell melalui Settings → Payments; increment ini tidak mengimplementasikan payment module baru. Payment draft/release/package lama tetap terbaca, tetapi pengajuan, approval/sign/publish, app-client readiness dan Testing dibatasi. Public catalog dan merchant test list menyaring payment sebelum pagination. Fixture Emisell Pay, kontrak/RPC dan instalasi existing tidak diubah.

Tidak ada migration data, seed, rotasi key atau uninstall. Rollout backend dahulu lalu tiga frontend; gunakan forward-fix agar backend lama tidak membuka kebijakan lama. Uji `TestInternalPaymentDistributionBoundary` pada database disposable dan jalankan suite lifecycle fixture existing. CLI developer/MCP belum dibuat; template payment gateway publik tidak menjadi rencana.
