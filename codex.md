# Emisell App Platform — Panduan Utama Codex/AI Agent

Dokumen ini adalah sumber aturan utama bagi Codex dan AI agent lain yang mengembangkan **Emisell App Platform**. Baca dokumen ini sampai selesai sebelum menganalisis, merancang, atau mengubah kode.

Kata **WAJIB**, **DILARANG**, **HARUS**, dan **BOLEH** bersifat normatif. Jika implementasi saat ini berbeda dari dokumen ini, jangan melakukan rewrite besar secara otomatis. Catat perbedaannya, tentukan migration path yang aman, lalu lakukan perubahan kecil dan terverifikasi.

## 1. Tujuan Platform

Emisell App Platform adalah platform app store dan extension terpisah yang memungkinkan aplikasi di-publish, direview, di-install, diaktifkan, di-upgrade, dan di-uninstall tanpa membuat Emisell Core bergantung pada implementasi provider tertentu.

Prinsip utamanya:

> **Emisell Core hanya mengetahui dan mengonsumsi capability yang versioned. Emisell Core tidak mengetahui provider yang memenuhi capability tersebut.**

Contoh yang benar:

```text
Emisell Core
    -> shipping/v1.get_rates
    -> Capability Resolver
    -> installation aktif milik merchant
    -> Shipping App terpasang
    -> provider eksternal
```

Contoh yang dilarang:

```text
Emisell Core -> XenditService
Emisell Core -> JNEService
POST /apps/midtrans/create-payment
POST /apps/jne/shipping-rates
```

Mengganti app/provider yang terpasang harus dapat dilakukan melalui konfigurasi installation dan routing capability, tanpa perubahan kode pada Emisell Core.

### 1.1 Kebijakan komersial tahap awal

- **Semua aplikasi gratis pada tahap awal.** Jangan mengaktifkan biaya install, subscription, komisi, payout developer, atau penagihan otomatis.
- Siapkan arah pengembangan, bukan scaffold billing tanpa kebutuhan: pricing, billing, dan entitlement tetap boundary terpisah. Installation memeriksa hak aktivasi melalui contract, bukan memanggil provider pembayaran.
- Dukungan berbayar per install kelak harus incremental melalui ADR, consent harga, idempotent purchase, entitlement, refund policy, dan migration path. Core/capability tidak berubah menjadi provider-specific.
- Instalasi gratis yang sudah ada tidak boleh otomatis ditagih. Kebijakan reinstall, transfer workspace, dan harga versi berikutnya **belum diputuskan**; jangan mengasumsikan pengguna membayar ulang.
- Payment gateway checkout merupakan integrasi internal Emisell (§1.18), bukan aplikasi umum App Platform. `payment/v1` lama hanya kontrak fixture compatibility. DILARANG mencampur pembayaran checkout dengan biaya pemasangan aplikasi.
- Callback pembayaran dan status bisnis berbeda dari keberhasilan pengiriman webhook. Dashboard tidak boleh menyimpulkan pembayaran sukses hanya karena delivery `delivered`.

### 1.2 Tiga antarmuka produk — keputusan final

Platform memiliki **Dashboard Admin, App Store, dan Portal Developer** sebagai tiga frontend terpisah. DILARANG membangun dashboard merchant/workspace sebagai antarmuka utama repository ini atau menyatukan ketiga peran dalam menu workspace merchant.

| Antarmuka | Pengguna dan tanggung jawab | Port development |
|---|---|---|
| Dashboard Admin | Tim internal Emisell: review, approve/reject, publikasi/suspend, pengelolaan developer, audit, dan kesehatan platform | `4317` |
| App Store | Pengunjung/merchant: listing aplikasi yang sudah dipublikasikan, pencarian, kategori, dan detail aplikasi | `4318` |
| Portal Developer | Developer: aplikasi miliknya, manifest, versi/release, konfigurasi integrasi, pengajuan review, dan log aplikasi yang diizinkan | `4319` |

- Semua frontend memakai satu backend Go modular monolith. Beda port/origin frontend tidak berarti microservices. API tetap `8087`, internal RPC `8088`; reserved port frontend tidak boleh dipakai harness pengujian baru.
- Alur publikasi: developer membuat draft dan mengajukan release; admin mereview dan approve/reject; hanya release yang memenuhi policy, signed, dan published yang tampil di App Store. Submission tidak boleh otomatis memublikasikan aplikasi.
- Merchant mengelola tokonya melalui **Emisell Core**, bukan melalui workspace baru dalam Admin/Developer Portal. Alur install/consent dari App Store harus memakai konteks tenant terautentikasi dari Core; jangan membuat workspace merchant atau memilih tenant melalui query URL sebagai sumber authorization.
- “Aplikasi saya” di Portal Developer berarti aplikasi yang dikembangkan/dimiliki developer, **bukan** installation merchant. Identitas organisasi developer tidak sama dengan tenant toko.
- Dashboard Admin menggunakan principal/permission admin platform; Portal Developer memakai ownership organisasi/app; App Store hanya menampilkan data publik yang diizinkan. Cookie/session owner workspace lama DILARANG dianggap sebagai akses admin atau developer.
- Authentication, authorization, cookie audience/name/path, CSRF, CORS, dan allowed redirect harus dirancang per surface. Beda port bukan security boundary yang cukup; cookie browser tidak terisolasi berdasarkan port. Enforcement tetap server-side.
- UI merchant workspace DILARANG: workspace/store selector, membuat/mengganti nama/reset toko, merchant overview, daftar installation “Aplikasi saya”, consent/activate/uninstall milik merchant, dan halaman transaksi merchant dalam portal utama. URL/query lama tidak boleh menghidupkan UI tersebut kembali.
- `tenant_id`, tenant isolation, installation, OAuth, scopes, capability, event, audit, dan data integrasi Core tetap diperlukan di backend. Penghapusan UI workspace **bukan** izin menghapus data, migration, atau proteksi lintas tenant. Data tenant hanya boleh terlihat oleh admin/developer melalui authorization baru yang eksplisit dan scoped.
- Keputusan ini menggantikan arah merchant dashboard dalam ADR/prototipe sebelumnya. Baca ADR 0007 untuk batas removal dan migration path. Jangan menyatakan tiga produk selesai hanya karena port, folder, atau halaman status tersedia.

### 1.3 Milestone akses dan draft–review

- Implementasi lokal awal: Admin `4317` dan Developer `4319` dengan akun/sesi terpisah, ownership organisasi developer, draft versioned, snapshot submission immutable, feedback dan audit keputusan. Lihat ADR 0008 dan `docs/local-development.md`.
- Admin/reviewer dapat memutuskan review; operator hanya-baca. Akun awal diprovision melalui CLI, bukan signup publik. Management role, undangan tim, dan onboarding production belum tersedia.
- `approved` berarti review metadata disetujui, **bukan** artifact telah lolos security scan, ditandatangani, dipublikasikan, atau dapat dieksekusi. Draft authoring bukan executable manifest. Jangan menyambungkan tabel draft/submission langsung ke resolver atau listing publik.
- `changes_requested` memerlukan revisi baru sebelum resubmit. Versi approved/rejected harus memakai nomor versi baru untuk pengajuan berikutnya. Idempotency, optimistic concurrency dan audit berlaku untuk semua mutation draft/review.
- Perluasan katalog dijelaskan pada §1.4. Manifest/runtime self-service lengkap, developer SDK runtime, dan consent dari Core tetap tahap berikutnya; jangan mengklaim admin/portal sudah lengkap secara keseluruhan.

### 1.4 Milestone katalog lokal, tooling metadata, dan akun utama

- Admin `4317`, App Store `4318`, Developer `4319` telah tersedia sebagai frontend terpisah. Backend tetap satu modular monolith. Lihat ADR 0009.
- Akun utama Admin lokal ditetapkan pengguna menjadi `dev@emisell.com`, ID `portal-local-admin`, role `administrator`. Password hanya berada pada hash database dan file credential lokal privat; DILARANG menuliskannya di dokumen/source/frontend. Akun Developer tetap terpisah.
- CLI `update-admin <email>` menerima password dari stdin, memverifikasi credential sebelumnya, mempertahankan ID/role, mencabut seluruh sesi akun tersebut secara atomik, dan mengaudit perubahan. Akun lain tidak berubah. Login yang sedang berjalan dengan hash lama tidak boleh membuat sesi setelah pencabutan.
- Developer tools memvalidasi draft tersimpan dan mengekspor **metadata katalog** `emisell.catalog/v1`, atau v2 untuk deklarasi scope resource (§1.7); bukan executable manifest `emisell.app/v1`. CLI dapat memvalidasi canonical JSON dan memverifikasi signature/checksum terhadap public key tepercaya.
- Rilis katalog dibuat hanya dari snapshot review approved, melalui validasi metadata, SHA-256, dan Ed25519 dengan key acak khusus platform. DILARANG memakai public fixture key atau menyatakan signature katalog sebagai hasil security scan kode aplikasi.
- Hanya `administrator` boleh menandatangani, publish, suspend, atau republish katalog. Reviewer tetap memberi keputusan review; operator hanya-baca. Developer hanya melihat rilis organisasinya. Publication mempunyai revision, idempotency, dan audit reason.
- Signed package immutable dan disimpan sebagai dokumen JSON terstruktur di schema app, bukan file executable. State publikasi terpisah dari isi paket; satu versi published per app. App Store hanya mengambil allowlisted public DTO dari rilis published dengan signature valid. Endpoint integrasi, feedback, actor/session, dan tenant data tidak boleh bocor.
- Semua listing `free` dan `installable: false`. Tidak ada upload binary, pemindaian app-code, executable release self-service, aktivasi runtime umum, atau consent/install dari Core pada milestone katalog ini. Jangan menghubungkan `catalog_releases` ke resolver/installation.
- Katalog bukan pengganti pipeline artifact pada §8. Saat executable releases dibangun, artifact tetap melalui S3/MinIO, integrity/security scan, runtime policy, signing dengan trust domain terpisah, health check, OAuth, dan consent dari Core. Metadata terstruktur kecil tidak memerlukan object-storage adapter kosong.
- Signing key lokal tersimpan privat, gitignored, diprovision eksplisit dengan `init-catalog`, tidak dirotasi otomatis. Hilangnya key membuat publikasi/verifikasi fail closed; suspend tetap tersedia. Production memerlukan secret manager/KMS, rotation/revocation dan trust policy tersendiri.

### 1.5 Milestone backend grant/install intent dari Core

- Kontrak internal `emisell.installation.v1.InstallIntentService` dan SDK menyediakan `Prepare`, `Get`, `Decide` untuk **consent record lokal**, bukan instalasi aktif. Lihat ADR 0010 dan `docs/core-install-intents.md`.
- Key platform full-access memakai `merchantId` per request (`merchant_id` di Protobuf) sebagai assertion backend Core tepercaya; wajib divalidasi dan tidak boleh berasal dari browser tanpa pemeriksaan Core. Tidak ada ID toko kedua; alias lama hanya compatibility §1.14. Key legacy tetap terikat service principal; field merchant boleh dihilangkan, tidak boleh mengganti binding. Scope khusus `apps.install_intents.read/write/consent` tidak otomatis diberikan kepada credential payment/shipping legacy. Jangan memberikan credential Core kepada app pihak ketiga atau frontend.
- Core WAJIB memverifikasi sesi staf, tenant membership, izin mengelola apps, CSRF, dan persetujuan eksplisit sebelum mengirim assertion `core_actor_id`. Platform mengikat intent pada tenant + service ID + actor dan mengaudit assertion; bukan bukti sesi merchant independen.
- Snapshot immutable mengikat versi, manifest digest, scopes, capabilities, dan consent digest, dengan TTL 10 menit. Keputusan sekali pakai, idempotency dan audit atomik. Retry identik mengembalikan state terkini; tidak menghidupkan intent expired atau memperpanjang TTL.
- Sumber release tahap ini hanya registry fixture executable terverifikasi. Katalog metadata tetap `installable:false`; tidak boleh dipromosikan otomatis. Semua response intent `execution_allowed=false`, termasuk `consented`.
- Kontrak consent ini sendiri tidak membuat instalasi/grant/token. Kelanjutannya untuk executable fixture tersedia terpisah pada §1.11; gateway resource dan executable katalog belum tersedia. Consent UI tetap di Core; DILARANG menambah merchant workspace/consent page di Admin/Developer Portal.
- Legacy install HTTP hanya compatibility/reference lokal dan belum memakai intent. Jangan menyatakannya aman untuk jalur consent production; migration path menuju consume atomik dan deprecation legacy wajib sebelum general release.
- Jangan menaikkan consent record lama menjadi active grant secara otomatis ketika pipeline executable selesai. Tidak ada biaya install pada tahap ini.

### 1.6 Dokumentasi API di Dashboard Admin

- Menu **Dokumentasi API** pada Admin menyediakan referensi read-only OpenAPI dan ConnectRPC/Protobuf. Deep link lokal `http://localhost:4317/?view=api-docs`; tetap mengikuti sesi Admin yang ada. Menu ini bukan portal merchant atau API playground.
- Sumber tetap di `api/openapi` dan `api/proto`. `web/dashboard/scripts/api-docs.mjs` membangkitkan referensi frontend, menyelesaikan schema references dan membaca descriptor Protobuf dari Buf. Tidak mengambil token, credential, atau data live untuk contoh.
- Saat kontrak berubah, jalankan `npm run docs:generate` dari dashboard; `npm run docs:check` dalam test mendeteksi drift. Scope RPC baru wajib memiliki metadata authorization di generator. File generated bukan source of truth.
- Dokumentasi portal hanya memuat Admin, Developer, App Store publik, dan Core RPC. Endpoint merchant/workspace legacy DILARANG tampil dalam daftar, pencarian, contoh, maupun unduhan sumber halaman ini. File kontrak backend legacy dipertahankan untuk compatibility internal; penghapusannya perlu migration path terpisah. Example placeholder tidak boleh dijalankan otomatis.

### 1.7 Scope resource aplikasi beracuan Shopify

- Katalog authoritative `pkg/accessscope`, profil immutable `shopify-authenticated-2026-09-05`: 108 handle dari tabel authenticated Shopify. Nama scope disamakan sebagai referensi, BUKAN jaminan API/token kompatibel Shopify. Storefront dan Customer Account dipisahkan karena model principal berbeda.
- Endpoint read-only `/api/v1/admin/access-scopes` dan `/api/v1/developer/access-scopes` mengikuti session, audience, Origin, dan ownership portal. Kedua portal mempunyai menu **Katalog scope**.
- `accessScopes: { profile, required, optional }` opsional pada draft, snapshot review, dan listing. Required/optional disimpan server-side, tidak boleh duplikat, asing, atau optional yang sudah implied oleh required. Dependensi required tidak boleh hanya optional.
- Semua scope resource saat ini `grantable:false`; label `planned`, `reference_only`, dan `future_reference` bukan grant. Review metadata dan signing tidak memberikan akses data. Restricted review Emisell terpisah dari consent merchant dan approval Shopify.
- Deklarasi resource memakai metadata `emisell.catalog/v2` / `catalog-metadata/v2`, tetap gratis dan `installable:false`. Tanpa deklarasi tetap v1, dengan canonical bytes/signature lama tidak berubah. Rilis immutable tidak boleh ditulis ulang atau dimigrasikan diam-diam.
- Scope capability fixture dot-style `orders.read`, `payments.*`, `shipping.*` tetap terpisah, demikian pula service scopes Core `apps.install_intents.*`. Tidak boleh rename atau alias otomatis; `read_shipping` bukan `shipping.read`.
- Sebelum scope aktif: implementasikan mapping operasi gateway versioned, tenant/actor enforcement, restricted-data policy, consume intent + grant atomik, token audience/installation binding, revocation, audit, idempotency dan contract tests. Unknown/planned scope wajib default deny; required unsupported menggagalkan install, optional unsupported tidak diberikan.
- Detail batas, rollback dan migration path: `docs/adr/0011-resource-access-scopes.md`. Integrasi resource gateway aktual ke Core belum tersedia pada milestone katalog ini. Jangan membuat merchant workspace baru di portal.

### 1.8 Handoff gateway resource untuk tim backend Core

- `docs/emisell-gateway-handoff.md` dan ADR 0012 adalah pegangan tim backend. Kontrak additive `emisell.resource.product.v1.ProductService` menyediakan `List`/`Get` produk dasar. Tidak ada endpoint resource live, façade external produk, atau listener baru pada milestone handoff.
- Arah gateway resource adalah Platform → Core; berbeda dari capability payment/shipping Core → Platform. Generated SDK/bindings tidak boleh dianggap sebagai implementasi server atau authorization.
- Mapping dan matriks 108 scope dimiliki `pkg/gatewaycontract`, diekspor dengan `go run ./cmd/cli gateway-contract` tanpa database/credential. Hanya read_products/implied write_products memiliki coverage parsial dua operasi baca. Seluruh grantable tetap false.
- `api/gateway/coverage.v1.generated.json` dan tampilan kelompok **Gateway Emisell · handoff** pada API docs Admin adalah snapshot kontrak, BUKAN health/discovery/support registry live. Endpoint planned memakai placeholder host Core, tidak boleh ditampilkan seolah aktif di port Platform 8088.
- `pkg/gatewaycontract/conformance.Run` dapat dipakai dalam integration test repo Core melalui client dan fixture dua tenant. Reference handler di test bukan gateway production. Lulus reference tests tidak membuktikan backend Core atau seluruh security gates selesai.
- Menambah scope/operasi harus melengkapi schema, authorization/field policy, errors, pagination, limits dan tests. Readiness implementation perlu evidence + code review, terpisah dari profil scope immutable. Dilarang mengubah status dokumentasi untuk membuka grant.
- Transport trust/delegation, grant freshness/revocation, lokasi deployment dan end-to-end consent/consume/token tetap gate lintas tim sebelum adapter diaktifkan. Validasi AccessContext tidak menggantikan verifikasi caller/current grant. Tidak ada DB migration, perubahan akun, atau grant pada pekerjaan handoff.

### 1.9 Tabel scope, verifikasi inventaris, dan API key Core

- Katalog scope Admin/Developer berupa tabel; status implementasi dan blocker berasal dari `/access-scopes/verification`, terpisah dari snapshot nama scope. Pemeriksaan saat ini inventaris build Platform, BUKAN live probe Core. Semua 108 resource tetap planned; kegagalan/mismatch tampil belum terverifikasi, tidak diasumsikan aktif.
- Menu **API Key** Admin hanya untuk administrator dan komunikasi backend Core → Platform. Keputusan final: **platform-level full access**, form hanya nama koneksi; tanpa tenant ID, expiry, atau pemilihan izin layanan. Berlaku sampai dicabut. Tidak berlaku sebagai sesi Admin/Developer atau token app. Lihat ADR 0014 yang menggantikan keputusan generation tenant-bound ADR 0013.
- Endpoint baru `/api/v1/admin/platform-keys` menyimpan credential terpisah (`epk_`, ID `platformkey_`), tanpa binding tenant atau expiry sintetis. Full access berarti akses layanan internal Core yang diimplementasikan, BUKAN melewati consent/grant/installation/runtime policy atau mengaktifkan gateway resource Plan. Resource request tetap membawa tenant context yang diotorisasi oleh backend Core; kosong/asing gagal tertutup. Jangan menambah merchant workspace.
- Key tenant lama (Admin/CLI) tidak dinaikkan privilege, dihapus, atau diperpanjang otomatis. `/api/v1/admin/api-keys` legacy tetap kompatibel untuk list/generate/revoke dengan aturan lama; UI generation memakai endpoint baru. Migrasi consumer harus eksplisit: generate pengganti, validasi koneksi dan tenant assertion, pindahkan consumer, revoke lama. Pending intent terikat service ID lama, tidak dipindah diam-diam.
- Secret random 256-bit hanya ditampilkan sekali; simpan hash saja. Idempotent generation/retry mengembalikan metadata tanpa secret ulang; respons hilang perlu revoke + generate baru. Revoke idempotent, audit sekali, tidak menghidupkan kembali key.
- `ConnectionService/Check` hanya memverifikasi credential Core dan identitasnya sendiri: key platform mengembalikan `platformFullAccess:true` tanpa tenant/scopes/expiry; key legacy mempertahankan metadata lamanya. Tidak membaca data merchant atau membuktikan gateway resource aktif. Internal RPC tetap menolak Origin/cookie browser. Production hardening dan app token/grant pipeline belum tersedia. Key permanen full access wajib server-only secret manager, rotasi operasional, audit dan revoke segera bila bocor.

### 1.10 Satu sumber kesiapan scope dan endpoint gateway

- **Katalog Scope adalah pusat status**, mapping endpoint, kendala, dan kelengkapan izin. Dokumentasi API berfokus pada endpoint, schema, contoh, scope yang diperlukan dan checklist handoff; jangan tampilkan ulang tabel 108 scope di sana.
- Kedua halaman memakai loader read-only yang sama menuju `/api/v1/{admin|developer}/access-scopes` dan `/access-scopes/verification` dengan session surface yang benar. Dokumentasi API tetap hanya Admin; developer tidak memperoleh sesi/tautan Admin. Tidak ada panggilan resource gateway atau probe URL pengguna dari browser.
- Read model verifikasi menambahkan `operations`, `environment`, dan `contractRevision`. Status operasi dan scope berbeda: satu operasi Active tidak cukup untuk seluruh scope Active. Status scope memerlukan coverage lengkap, semua operasi yang dipetakan siap, grant pipeline dan bukti Core sesuai gate keamanan.
- Status tidak diambil dari snapshot generated. Profil/mapping/kontrak tetap versioned di repo; fingerprint kontrak dicocokkan dengan dokumentasi. Response gagal, format tidak valid, versi tidak cocok, atau evidence tidak lengkap berarti **Belum terverifikasi**, bukan fallback Active. Refresh membaca sumber backend yang sama; bukan live push atau monitoring otomatis.
- Kondisi implementasi tetap inventaris build lokal: 108 scope dan dua endpoint produk masih Plan, `coreChecked:false`, `grantable:false`. Tidak ada toggle aktivasi, gateway Core live, DB migration, atau perubahan grant/key dalam milestone ini. Tampilan Active bukan otorisasi.
- Deep link scope ↔ endpoint hanya navigasi/filter lokal. Procedure yang belum dikenal dokumentasi harus tampil tidak ditemukan, tidak diam-diam memilih endpoint lain. URL gateway tetap milik Core walaupun status kelak berubah; jangan fallback ke port Platform 8088. Detail rollout/rollback: ADR 0015.

### 1.11 Lifecycle dari consent untuk executable fixture lokal

- ADR 0016 memperluas milestone consent-only §1.5. Service baru `InstallationService` internal/SDK `client.Installations` menyediakan Consume, GetInstallation, Activate, IssueToken, Uninstall; hanya key platform full-access, terikat tenant + service + actor. Consent UI tetap milik Core.
- Prepare baru mengikat `local-reviewed-fixture/v1` dalam snapshot/digest. Consent historis tanpa policy tidak dapat dikonsumsi. Kontrak Prepare/Get/Decide tetap `executionAllowed:false`; setelah consume, baca lifecycle dari GetInstallation, bukan state consent.
- Consume memeriksa TTL/digest/policy/release, lalu membuat installation dan grant pending atomik bersama receipt single-use, audit, outbox. Activate memverifikasi ulang registry, scope fixture dan runtime/remote handshake sebelum grant/routing active. Metadata catalog tidak menjadi executable; seluruh listing tetap gratis `installable:false`.
- Token opaque `eat_` 256-bit, hash-only, sekali tampil, TTL 15 menit, audience self-check installation lokal. IssueToken baru mencabut token sebelumnya; retry tidak mengungkap secret dan mengembalikan metadata terkini. Tidak ada refresh atau OAuth developer-client/token exchange production pada milestone ini. Jangan mengirim key Core maupun app token ke browser/log/event/query URL.
- GET `/api/v1/app/installation-access` hanya self-check server-to-server dengan app token dan header tenant/app/installation yang cocok. Tidak menerima Origin/Cookie/query; bukan akses resource, capability RPC, portal atau grant delegasi reusable. Dokumentasi ditambahkan pada Admin tanpa menu merchant.
- Uninstall atomik mencabut grant/token/routing; remote cleanup memakai worker retry existing dan tetap disabling selama gagal. Grant/token tidak boleh tetap aktif ketika cleanup gagal. Reinstall memakai consent dan ID baru; retry lama tidak boleh mengubah replacement. Legacy mutation dilarang untuk installation intent-managed.
- Native grant menjadi bagian aggregate installation untuk transaksi atomik; material token dipisahkan di `oauth/apptoken`. Tidak ada dependency/billing baru. Semua 108 resource gateway tetap Plan; fixture dot-scopes BUKAN Shopify handles atau resource grant.
- Migration 0012 additive, tanpa backfill/account/key/installation mutation. Backup sebelum apply; setelah lifecycle baru dipakai, gunakan forward-fix dan jangan rollback ke binary lama yang tidak memahami grants. Panduan consumer: `docs/core-installation-lifecycle.md`.

### 1.12 Rilis konfigurasi Aplikasi Integrasi

- Nama yang terlihat pada UI adalah **Aplikasi Integrasi**, bukan Remote App. Identifier teknis `remote`, enum fixture historis, kontrak wire, dan signed bytes lama WAJIB tetap kompatibel. Label tidak mengubah tempat aplikasi dijalankan.
- ADR 0017 menambahkan menu **Rilis integrasi** di Admin dan Developer. Ini pipeline konfigurasi non-executable `emisell.integration-release/v1`, policy `integration-configuration/v1`, BUKAN persetujuan kode aplikasi atau pipeline artifact production yang lengkap.
- Developer memilih snapshot metadata approved milik organisasinya, lalu mengajukan konfigurasi immutable: endpoint identik dengan snapshot, callback OAuth dan health URL, profil `emisell.capability-http/v1`, versi/capability/scopes dari snapshot. Tidak boleh menerima organization/tenant/role/secret dari input tersebut.
- Validasi tanpa network: HTTPS, hostname DNS, port 443, tanpa userinfo/query/fragment, callback dan health satu origin. Authoring/distribusi baru hanya shipping/v1; schema payment/v1 historis tetap dapat diverifikasi tetapi gagal policy distribusi §1.18. Required resource scope harus grantable; optional Plan tidak diberi grant. Validasi statis BUKAN pemeriksaan DNS/TLS/health, kontrol domain, redirect/egress atau security scan kode.
- Status `submitted → approved/rejected`, `approved → signed/suspended`, `signed → suspended`. Reviewer/administrator boleh review konfigurasi; signing/suspend hanya administrator; operator hanya-baca. Metadata approved tidak otomatis menyetujui konfigurasi. Setiap tindakan memerlukan idempotency, revision, alasan dan audit atomik.
- Snapshot, checksum dan signature immutable; satu konfigurasi per app/version dan submission. Perbaikan setelah submit membutuhkan versi baru dan review metadata baru. Retry mengembalikan status release TERKINI; tidak menghidupkan release yang suspended/rejected. Endpoint developer tidak menyediakan tindakan review/signing.
- Signer Ed25519 memakai key acak khusus integrasi dan domain signature terpisah dari katalog maupun public test fixture. CLI `init-integration-signing` memprovision `.local/integration-signing.json` privat tanpa overwrite/rotasi. Key hilang memblokir signing/verifikasi; rejection/suspension tetap tersedia. Tidak ada auto-provision key saat server start.
- Readiness konfigurasi dihitung ulang dari snapshot + checksum + signature bila tersedia. Status app-client dan bukti kendali endpoint diperiksa terpisah (§1.13). `installable:false` WAJIB dipertahankan: runtime umum, OAuth authorization-code/token exchange dan conformance E2E belum tersedia. DILARANG menambahkan adapter dari integration_releases ke registry executable/simulator hanya agar bisa install. Consume Core tetap menolak app developer yang bukan fixture eligible.
- Migration 0013 additive, tanpa backfill atau perubahan akun/key/instalasi/katalog lama. Backup sebelum apply. Data konfigurasi JSON kecil berada di PostgreSQL; belum ada binary artifact. Saat binary/runtime umum dibangun, ikuti S3/MinIO, scan, review, OAuth, sandbox/egress dan consent §8–9; jangan menganggap konfigurasi signed otomatis executable. Lihat `docs/integration-releases.md`.

### 1.13 App-client dan bukti kendali endpoint

- ADR 0018 menambahkan menu **App clients** di Admin dan Developer. Registrasi confidential app-client hanya dari konfigurasi signed yang masih valid dan milik organisasi developer; satu client per release, binding ID/versi/checksum/origin/callback immutable. Ini fondasi identitas server aplikasi, BUKAN implementasi OAuth authorization server lengkap atau grant merchant.
- Developer dapat register, memperbarui challenge, meminta verifikasi, menerbitkan/merotasi secret dan revoke. Administrator hanya dapat membaca/revoke, reviewer/operator hanya membaca. Semua mutasi memakai idempotency, revision (untuk action), ownership dan audit. Revoked terminal; versi release baru diperlukan. Identitas stabil lintas versi untuk OAuth production memerlukan ADR/migration tersendiri, bukan memindahkan binding/grant diam-diam.
- Challenge acak berlaku 10 menit, response bukti mengikat client + release SHA-256 + nonce. Hanya ambil JSON dari path well-known buatan platform pada origin konfigurasi signed. DILARANG menerima URL probe bebas, mengikuti redirect, memakai proxy environment, mengirim expected nonce/credential/tenant, atau menonaktifkan validasi TLS.
- Verifier v1 memakai DNS A publik, memeriksa seluruh jawaban lalu pin satu IPv4 numerik saat dial. IPv6-only, alamat private/link-local/metadata/reserved, non-443 dan IP literal tidak didukung. Timeout total 5 detik, empat probe bersamaan per process, cooldown 1 menit per client, response maksimum 4096 byte dan schema exact. Tidak ada flag bypass untuk development; pengujian memakai adapter terkontrol di test saja.
- Bukti berhasil berlaku 24 jam. Challenge baru langsung menghapus bukti dan secret lama. Probe tidak menahan lock database saat network I/O; completion memeriksa ulang release dan revision agar revoke/renew/suspend tidak ditimpa. Pool koneksi client terpisah dari pool gate release menghindari lease inversion; tetap satu modular monolith/database, bukan service baru.
- Secret `eacs_` acak 256-bit, hanya hash SHA-256 disimpan; sekali tampil, tidak masuk receipt/audit/event/log/browser storage. UI menghapus tampilan setelah 5 menit atau meninggalkan detail. Rotasi membatalkan secret lama; retry tidak mengungkap ulang secret. Revoke tetap tersedia ketika signing/release verifier bermasalah.
- `clientReady` membutuhkan release signed valid, bukti belum expired dan secret aktif. `oauthEnabled`, `installable`, serta akses resource tetap false. POST `/api/v1/app/client-check` memakai HTTP Basic client ID/secret, tanpa Origin/Cookie/query, body JSON kosong, hanya self-check identitas. DILARANG menerima key Core/token instalasi sebagai client secret atau mengubah self-check ini menjadi grant/token exchange.
- Origin proof BUKAN health check endpoint bisnis, validasi callback OAuth, conformance capability, scan kode atau bukti grant merchant. Semua 108 resource scope tetap Plan. General runtime dan consent/token exchange tetap tahap terpisah. Jangan menambahkan UI merchant.
- Migration 0014 additive tanpa mengubah akun/key/instalasi lama; backup privat dan apply eksplisit sebelum restart. Panduan operasi, batas protokol dan rollback: `docs/app-clients.md` dan ADR 0018.

### 1.14 Identitas merchant tunggal pada kontrak integrasi

- Keputusan pengguna: **cukup `merchantId`**, tidak ada tenant ID kedua atau mapping manual. Nilai berasal dari sesi/otorisasi backend Emisell; identitas actor tetap diperlukan untuk consent dan audit.
- Referensi endpoint, schema utama, contoh request/response dan panduan handoff memakai `merchantId` (`merchant_id` pada Protobuf), serta `X-Emisell-Merchant-ID` pada self-check app. Istilah tenant pada domain/isolation lama bukan identitas bisnis berbeda.
- Field Protobuf `tenant_id` lama dipertahankan deprecated pada nomor field semula. Adapter menerima salah satu nama; dua nilai berbeda WAJIB ditolak sebelum use case. Normalisasi memakai nilai identik tanpa membuat toko/ID atau mengubah hash snapshot/idempotency. Response RPC mempertahankan alias untuk reader v1; schema/example utama tidak mengajarkan alias deprecated sebagai input wajib. Sumber unduhan tetap persis kontrak asli, termasuk penanda deprecation.
- Endpoint API key legacy tenant-bound tetap berfungsi untuk consumer lama, tetapi tidak masuk referensi utama Admin; generation yang direkomendasikan tetap platform full-access. Jangan menghapus credential/data legacy.
- Tidak ada migration database, auto-provision merchant, penghapusan data, pembukaan scope Plan atau perubahan permission. Merchant harus merupakan referensi yang dikenal Platform dan diotorisasi Core. Penamaan `merchantId` tidak boleh dipakai untuk melewati validasi keberadaan atau isolasi merchant.
- Rollout server-first untuk caller baru yang hanya mengirim merchantId; consumer lama tetap kompatibel. Jangan rollback server lama setelah caller baru bergantung field ini tanpa migrasi caller. Lihat ADR 0019.

### 1.15 Daftar Installed Apps di Dashboard Core

- ADR 0020 menambahkan `InstallationService/ListInstallations` sebagai **read model merchant-wide** untuk key Core full-access setelah Core memverifikasi sesi dan izin apps setiap halaman. Request memakai merchantId, actor, pageSize (maksimum 20), dan afterId; bukan tenant ID kedua atau UI workspace Platform.
- Daftar membaca current intent-managed installation dari backend, lintas actor/key pemasang, hanya metadata display tanpa token/digest/actor. Pending, Active dan Disabling berbeda; riwayat uninstalled dan data PluginApp legacy tidak dicampur. Error bukan empty/sukses palsu.
- Pengecualian read-only ini tidak mengubah ownership GetInstallation/consent/mutation pada §1.11. Daftar tidak memberikan hak aktivasi, token, reinstall atau uninstall lintas actor/service. Dashboard Core kembali ke daftar setelah aktivasi terverifikasi; URL hanya navigasi, bukan sumber status sukses.

### 1.16 Detail dan tindakan Installed Apps di Dashboard Core

- Dashboard Core menyediakan menu **Open app, View details, Uninstall** dan halaman detail installation. Milestone fixture memakai GetInstallation + Get intent untuk metadata, requested/granted scopes serta timestamp consent/consume; bukan membaca database Platform dari Core. Ownership detail dan uninstall tetap merchant + service + actor pemasang; daftar merchant-wide tidak memperluas authority.
- Open app hanya boleh aktif jika terdapat launch contract/URL UI yang terverifikasi. Fixture simulator saat ini belum mempunyai UI aplikasi; tampilkan disabled dengan alasan. Endpoint capability, callback OAuth, katalog metadata dan query browser bukan launch URL. Jangan membuat URL, data aktivitas, klaim akses pelanggan atau kebijakan privasi fiktif.
- Uninstall dilakukan setelah konfirmasi tindakan destruktif (berbeda dari consent Install tanpa checkbox). Core memeriksa kembali sesi/izin; server menurunkan key stabil namespace `uninstall` per installation dan memanggil RPC Uninstall existing. Status wajib revoked dengan uninstalled/disabling; timeout bukan sukses atau kegagalan pasti. UI membaca ulang status sebelum retry manual, tidak mengulang mutation otomatis.
- Grant/token/routing dicabut oleh lifecycle Platform, receipt/audit dipertahankan. UI memperbarui daftar dari backend setelah uninstall dan membedakan cleanup pending. Reinstall membutuhkan intent/ID baru, bukan menghidupkan receipt lama. Tidak ada perubahan schema, key, scope Plan, runtime umum atau ownership contract pada milestone ini.

### 1.17 Distribusi aplikasi uji (Testing)

- Developer mengajukan release integrasi signed milik organisasinya ke `merchantId` yang diketahui; tidak boleh menyediakan pencarian seluruh merchant. Administrator memverifikasi merchant terdaftar, lalu menyetujui/menolak/mencabut assignment. Reviewer/operator hanya membaca.
- Assignment mengikat release, checksum dan merchant secara immutable, dengan revision, idempotency dan audit alasan. State `requested → approved|rejected`, `approved → revoked`; versi/merchant baru memerlukan pengajuan baru. Revoke assignment tidak melakukan uninstall.
- Core membaca assignment approved melalui `emisell.testing.v1.TestDistributionService/ListAssignments` menggunakan full-access key, `merchantId` dan actor terverifikasi. Facade browser `GET /v1/app-platform/core/test-apps` wajib memeriksa sesi/izin terkini; tanpa identity override atau secret frontend. Pagination maksimal 20.
- Settings → Apps di Dashboard Core menampilkan daftar dinamis tersebut, menggantikan kartu fixture hardcoded. Installed apps dan lifecycle fixture tetap kompatibel. Jangan menambahkan merchant workspace ke portal Platform.
- Persetujuan pengujian **bukan** consent/grant/install. Configuration/scope readiness dibaca ulang dari release/katalog sumber. `installable=false` sampai runtime developer, OAuth instalasi, endpoint contract/egress dan uji lifecycle nyata selesai. Jangan membuat jembatan metadata→fixture untuk menampilkan sukses palsu. App-client readiness tetap terpisah.
- Lihat ADR `docs/adr/0021-test-app-distribution.md`. Migration `0015` additive; backup sebelum apply, tanpa seed/reset atau perubahan instalasi live.

### 1.18 Pemisahan payment gateway internal dan aplikasi umum — keputusan 6 September 2026

- **Payment gateway checkout dikelola dalam modul pembayaran internal Emisell melalui Settings → Payments.** Aktivasi provider tidak melalui App Store atau pengajuan developer umum. Adapter provider berada di boundary modul pembayaran; logika bisnis Core tetap provider-neutral. Pekerjaan ini tidak memindahkan/mengimplementasikan payment processor baru.
- **Shipping tetap aplikasi integrasi publik** melalui kontrak `shipping/v1`; API-Kurir/provider lain menjalankan layanan dan mapping payload di aplikasi/adapternya, bukan endpoint provider-specific pada Core. Platform menangani authorization merchant, routing, validasi, idempotency, timeout dan audit.
- App Platform bukan khusus shipping: arah jangka panjang mencakup CRM, sinkronisasi produk, marketing dan integrasi resource/webhook. Saat ini hanya `shipping/v1` tersedia untuk authoring/distribusi baru; aplikasi generik tanpa capability belum diimplementasikan. Jangan mengklaim schema/runtime generik sudah tersedia.
- Kebijakan authoring/distribusi berada di `internal/app/service/distribution.go`, terpisah dari schema/canonicalization/signature historis. Backend menolak payment pada save/update draft, submit/approve metadata, signing/publish katalog, konfigurasi/review/signing integrasi, app-client readiness/actions non-revoke, serta pengajuan/approval Testing. UI bukan security boundary.
- Draft/snapshot/package/payment historis tetap dapat dibaca oleh pemilik/admin, checksum dan signature tetap dapat diverifikasi. Reject, suspend dan revoke tetap tersedia. Jangan mengonversi draft payment menjadi shipping diam-diam. Listing payment tidak tampil pada App Store atau daftar distribusi Testing merchant; filter dijalankan sebelum pagination.
- **Emisell Pay dan reference payment yang sudah terpasang tidak dihapus, dipindahkan, dicabut atau diubah oleh perubahan kebijakan ini.** RPC payment, field Protobuf, scope fixture, registry, consent dan lifecycle fixture lama dipertahankan untuk compatibility/test lokal, bukan fasilitas payment gateway developer publik. Deprecation/hentikan instalasi fixture kelak perlu migration terpisah dan keputusan eksplisit.
- Katalog 108 scope resource tetap immutable dan tidak dihapus/diaktifkan hanya karena ada kata payment. Deklarasi scope akses data bukan izin memproses pembayaran. `grantable:false` tetap; perubahan restricted scope/implementasi gateway perlu gate tersendiri.
- CLI developer direncanakan memakai library Go bersama dan template aplikasi generik + contoh shipping; **tanpa template payment gateway publik**. MCP menjadi antarmuka opsional AI atas logika yang sama. CLI developer/MCP belum dibangun oleh keputusan ini; keduanya tidak boleh memakai key full-access Core.
- Billing instalasi/subscription aplikasi merupakan boundary App Platform terpisah. Semua aplikasi tetap gratis; tidak ada perubahan billing atau tagihan.
- Bagian milestone lama yang menyebut payment/shipping menggambarkan reference historis dan tunduk pada pembatasan ini. Detail rollout, retry dan migration path: ADR `docs/adr/0022-internal-payment-boundary.md`. Tidak ada migration data/credential/installation pada increment ini; rollout backend dahulu lalu frontend. Jangan rollback ke backend yang membuka kembali pengajuan payment.

### 1.19 Referensi integrasi API-Kurir lokal — keputusan 6 September 2026

Catatan: model agregator di bagian ini hanya menggambarkan fixture historis. **Arah produk terbaru adalah aplikasi per-provider sesuai §1.21**, bukan satu aplikasi API-Kurir yang dipasang merchant. Fixture/snapshot lama tidak dikonversi diam-diam.

- ADR 0023 menambahkan referensi tertutup `api-kurir-reference`, profil `local-kurir-reference`, **hanya di komposisi pengujian eksplisit**. Tidak di-seed saat startup/CLI normal, tidak menghubungkan katalog/integration_releases ke executable registry, tidak membuka Testing installability atau runtime developer umum.
- Reference memakai consent/Consume/Activate/ListInstallations/Uninstall existing dengan `merchantId`, service dan actor. Binding durable adalah installation ID + merchant + app/version + digest/profile dalam snapshot consent; tidak membuat tenant ID kedua atau salinan credential/provider database.
- API-Kurir tetap pemilik konfigurasi provider/credential. Satu aplikasi agregator membungkus akun API-Kurir merchant; pilihan provider di dalamnya bukan aplikasi terpisah pada increment ini. Pemeriksaan readiness hanya membaca provider efektif aktif, available dan installed. Install/activate tidak mengubah pilihan provider; uninstall tidak menonaktifkan provider, menghapus key, atau membatalkan pengiriman upstream.
- Scope reference hanya `shipping.read`; capability hanya `shipping/v1.get_rates`. Booking, tracking, pickup, label/cancel, scope resource Plan dan pembayaran tetap ditolak. Nama scope API-Kurir dan Shopify tidak di-alias otomatis. `shipping.read` di profile ini hanya mengizinkan operasi yang diimplementasikan, bukan semua operasi baca.
- `GetRates.origin_zone` / JSON `originZone` additive dan wajib untuk bridge; simulator historis boleh menghilangkannya. Origin/destination adalah zona netral yang dipetakan oleh konfigurasi server bridge ke ID kecamatan API-Kurir, bukan input provider ID dari Core. Hash request lama tidak berubah ketika field kosong.
- `internal/runtime/kurir` hanya menerima origin HTTP IP literal loopback, port non-reserved, key fixture publik tetap, no redirect/proxy, response maksimal 32 KiB, timeout 3 detik, empat request bersamaan dan marker fixture. Tidak menerima credential production atau URL dari merchant. Readiness diperiksa ulang sebelum invocation **termasuk replay**, setelah active grant gate. Tidak ada hidden retry.
- Jalur cek ongkir memakai `include_group=true` agar hasil membawa canonical service. Ini bukan `shipping-quotes` yang mengunci harga untuk booking; response tetap `simulation:true`. API-Kurir tiruan menguji payload kontrak, bukan membuktikan koneksi production.
- Uninstall menggunakan lock/transaksi existing untuk mencabut grant/token/routing; invocation yang sudah memegang lock diselesaikan dahulu, invocation setelah revocation ditolak sebelum upstream. Reinstall mendapat ID/consent baru; receipt lama tidak boleh menghidupkan instalasi atau memberi akses replacement.
- Tidak ada migration schema/data/secret, perubahan repo API-Kurir/Core, deploy atau restart layanan pengguna pada increment ini. Testing memakai database `emisell_local_test` terisolasi. Production memerlukan kontrak zona lengkap, service identity/delegation, live grant enforcement, konfigurasi/egress terverifikasi, policy penanganan shipment berjalan, conformance dan persetujuan rollout terpisah.

### 1.20 Gunakan UI instalasi yang sudah ada — koreksi 6 September 2026

- UI merchant tetap di Dashboard Core pada `/store/[merchantId]/settings/apps`, termasuk daftar, review izin, detail dan tindakan instalasi. **Jangan membuat `/sandbox/settings/apps`, merchant shell baru, atau kontrol simulator sebagai UI produk.** Pengujian terisolasi berjalan melalui fixture/test runner, bukan menu baru yang harus dipakai merchant.
- Integrasi API-Kurir mempertahankan alur install credential, konfigurasi dan pemilihan provider yang sudah tersedia. API-Kurir adalah pemilik state operasional tersebut; Dashboard/Core/App Platform meneruskan permintaan melalui boundary yang terverifikasi, bukan menyimpan salinan konfigurasi atau membuat pilihan provider tiruan.
- Halaman Shipping dan komponen konfigurasi provider existing digunakan kembali bila diperlukan dari Apps. Jangan menggandakan form credential, mengambil alih aktivasi provider secara otomatis, atau menganggap grant App Platform sama dengan status provider API-Kurir.
- Merchant ID berasal dari sesi Core terverifikasi; service key upstream tetap server-only. Reuse boundary existing bukan izin membuka proxy arbitrary atau melewati pemeriksaan permission/grant. Migrasi routing ke App Platform harus bertahap dan mempertahankan kontrak yang sedang dipakai.
- Fixture `api-kurir-reference` tetap bukti kontrak backend, bukan app production/Testing yang otomatis tersedia di Dashboard. Pengujian Core → Platform memakai database disposable; tidak mengubah instalasi, credential atau layanan pengiriman merchant nyata. Endpoint forwarding production dan handoff konfigurasi belum dinyatakan selesai hanya karena fixture lulus.

### 1.21 API-Kurir sebagai engine; provider sebagai aplikasi terpisah — keputusan 6 September 2026

- **API-Kurir adalah engine/gateway bersama menuju checkout Emisell, bukan aplikasi agregator yang dipasang merchant.** RajaOngkir, KiriminAja, Mengantar dan provider berikutnya memiliki identitas aplikasi, release, instalasi, consent, grant dan uninstall masing-masing. Terpisah sebagai app tidak mewajibkan service/deployment baru.
- App Platform mengelola registry/review/distribusi dan lifecycle akses. API-Kurir tetap mengelola adapter, credential, readiness per environment, pilihan provider efektif aktif, ongkir, shipment, pickup, label dan tracking. Core checkout tetap memakai kontrak shipping netral; jangan memindahkan business logic provider ke Core atau menduplikasi engine di App Platform.
- Binding app → engine/provider adalah konfigurasi release immutable yang ditandatangani, terikat snapshot/digest consent. Bukan input URL/browser, tebakan dari nama app, atau mapping tenant baru. Merchant ID tetap dari sesi Core terverifikasi. Jangan menyimpan credential API-Kurir di App Platform.
- **Install aplikasi tidak memilih provider checkout.** Beberapa aplikasi dapat terpasang/grant aktif; maksimal satu provider shipping efektif aktif tetap keputusan API-Kurir. `installed`, grant aplikasi dan `active_provider_code` adalah state berbeda. Membaca readiness saat install tidak boleh memanggil activate/deactivate/disable credential upstream. Shipment lama tetap memakai provider/environment snapshot asalnya.
- Increment backend awal memakai profile uji tertutup `local-kurir-provider`, dengan manifest `shippingProvider: {engine: "api-kurir", providerCode: ...}`. ID berakhiran `-provider-reference` adalah material pengujian, bukan listing production atau bukti provider sudah terintegrasi. Constructor komposisi `InternalHandlerWithLocalShippingProviders` tidak dipakai server normal. Tidak ada seed/startup otomatis, perubahan Testing installability atau pemindahan metadata katalog menjadi executable.
- Consume/Activate/Get/List/Uninstall menggunakan lifecycle persistent existing. Scope fixture tepat `shipping.read`, binding tersimpan dalam snapshot. Activate memeriksa detail provider **yang terikat** dan mensyaratkan `installed=true`, `available=true`, serta flag `built_in` yang cocok: true hanya untuk kode `emisell` (§1.22), false untuk provider eksternal. Provider lain yang aktif tidak dapat menggantikan persyaratan ini. Readiness ini hanya pemeriksaan instalasi, bukan bukti kesiapan ongkir/fulfillment live.
- Profile provider saat ini **lifecycle-only**: capability manifest tetap `shipping/v1` sebagai deklarasi, tetapi `InstallationAccess.capabilities` (routing yang benar-benar aktif) kosong, termasuk saat pending/active. Dengan demikian beberapa instalasi tidak berebut resolver shipping lama atau mengubah checkout. Token issuance ditolak; tidak ada invocation, bypass ke simulator, atau callback app baru. Perilaku profile historis dan byte/hash snapshot tanpa binding tetap kompatibel.
- Frontend tetap halaman Apps dan konfigurasi Shipping existing (§1.20). Increment ini tidak menambahkan menu, route sandbox, tombol Install palsu atau kartu app hardcoded. Belum ada publikasi provider yang bisa dipasang lewat UI; runtime produksi, grant gate pada **seluruh** jalur engine/legacy, credential service-to-service, rollout merchant pilot, conformance engine nyata dan policy shipment berjalan wajib selesai sebelum cutover.
- Uninstall fixture hanya mencabut grant/routing/token aplikasi yang dipilih secara idempotent. Tidak menghapus key, menonaktifkan provider lain, mengubah pilihan checkout, atau membatalkan shipment. Untuk production, pencabutan harus benar-benar menutup operasi baru pada jalur existing juga; endpoint lama tidak boleh menjadi fallback ketika grant ditolak atau Platform unavailable.
- ADR 0024 mencatat pemisahan state, implementasi increment, batas keamanan, rollout dan rollback. Tidak ada migration database/live data, perubahan repo API-Kurir/Dashboard, aktivasi key, restart atau deployment dalam increment ini.

### 1.22 Pengujian Emisell Kurir dan RajaOngkir dengan developer demo — 6 September 2026

- Pengguna mengizinkan akun developer demo untuk dua aplikasi uji: **Emisell Kurir** dan **RajaOngkir**. Emisell Kurir berarti provider built-in API-Kurir dengan canonical code `emisell`, bukan aplikasi agregator engine dan bukan alias publik `biteship`.
- Factory fixture lifecycle menambahkan kode exact `emisell`; readiness wajib `built_in=true`. Provider eksternal tetap wajib `built_in=false`. Jangan memindahkan pengecualian built-in ke provider lain, mengambil credential pool platform, atau mengubah selection/credential API-Kurir. Built-in upstream selalu installed tidak otomatis menciptakan grant aplikasi.
- Dua draft lokal dibuat melalui API Developer existing pada organisasi Emisell Developer, akun `developer@emisell.local`: `Emisell Kurir · Test` dan `RajaOngkir · Test`. Draft meminta `shipping.read` saja, belum memiliki endpoint runtime, belum diajukan/disetujui/signed/published/assigned. Daftar/detail hasil pembacaan ulang membuktikan persistensi, bukan kesiapan install. Referensi ID ada di `docs/shipping-demo-testing.md`.
- Draft milik developer **tidak sama** dengan fixture `*-provider-reference`. Nama app/deskripsi yang menyebut provider bukan binding otorisasi. Jangan memasukkan draft ke executable registry, mengubah `installable` menjadi true, membuat consent/install otomatis, atau menganggap fixture lifecycle sebagai runtime app developer.
- Test persistent menjalankan pasangan RajaOngkir–Emisell Kurir dan RajaOngkir–KiriminAja dengan API-Kurir tiruan. Consent, isolasi, activation, uninstall kedua app saat engine outage, replay, audit dan checkout legacy diuji tanpa menyentuh data/credential produksi. Tidak ada migration, restart layanan, deploy, atau perubahan Dashboard/Core/API-Kurir dalam increment ini.
- Tahap berikutnya memerlukan kontrak **managed shipping extension**: release signed dengan binding engine/provider, assignment eligibility dan grant enforcement di engine. Jangan mengarang callback OAuth/health endpoint agar draft lulus pipeline Aplikasi Integrasi umum. Auth engine server-to-server berbeda dari OAuth app developer; produksi tetap mengikuti gate ADR 0024. URL API-Kurir lokal/staging dan batas operasi uji harus dikonfirmasi sebelum koneksi memakai credential/saldo nyata.
- Pengguna memilih Emisell Kurir lebih dahulu; credential RajaOngkir diisi sendiri setelah aman. Uji nyata dibatasi readiness/koneksi/ongkir, bukan shipment/booking/pickup/transaksi. Inspeksi source menemukan jalur proxy shipping legacy di Core memakai merchant context dari `jwt.decode`/domain resolution tanpa current app grant. Jangan menghubungkan distribusi testing ke jalur itu sebelum boundary auth dan migration legacy ditangani; jangan mengubah shared middleware global atau merusak checkout publik. Detail evidence dan batas temuan ada di `docs/shipping-demo-testing.md`; belum dilakukan eksploit/request nyata.

### 1.23 Boundary Shipping Core dan release provider terkelola — 6 September 2026

- Pengguna menyetujui pengamanan pengaturan Shipping dan persiapan release Emisell Kurir. ADR 0025 menetapkan pipeline `emisell.managed-shipping-release/v1`, policy `managed-kurir-provider/v1`, terpisah dari generic integration dan fixture.
- Developer mengajukan snapshot draft miliknya dengan revision; reviewer/admin review, hanya administrator sign/suspend. Binding pertama exact `api-kurir/emisell`, capability `shipping/v1`, requested scope `shipping.read`, free, tanpa endpoint/credential/OAuth fiktif atau scope resource. Satu release per app/version, source checksum dan binding immutable; receipt/revision/audit atomik. Key signing privat independen, tidak memakai key fixture atau mengubah key lama.
- API `/api/v1/{admin|developer}/managed-shipping-releases` tersedia untuk list/detail dan tindakan sesuai role. Migration 0016 additive. Signed configuration **belum installable**: assignment managed dan engine grant enforcement belum terhubung. Jangan menginjeksi release ini ke fixture registry atau menampilkan sukses instalasi dari signature saja.
- Core Shipping management kini memeriksa sesi JWT/device/current membership/izin `storeSettings.shippingAndDelivery`, custom header dan trusted Origin/Referer, metode/path allowlist, redaksi dan limit upstream. Public domain resolution memakai customer-only; shared middleware global, webhook verifier dan service checkout/order existing tidak diubah. Scope legacy ops yang tidak di-allowlist tidak boleh kembali melalui wildcard. Dashboard header dan canonical credential path di-rollout bersama guard.
- Perubahan ini tidak memasang aplikasi, mengganti provider checkout, menghapus credential/pengiriman, atau membuka grant resource Plan. Target UI berikutnya tetap Settings → Apps dan Additional shipping methods existing. Rollout/compatibility, gate tahap berikutnya dan test ada di ADR 0025 serta `docs/shipping-demo-testing.md`.

### 1.24 Distribusi provider terkelola melalui Testing — 6 September 2026

- ADR 0026 menambahkan `releaseKind=managed_shipping` pada assignment existing; field kosong tetap integration historis. Dua foreign key dengan constraint tepat satu sumber, signature/checksum, ownership, shared release lock, revision/idempotency dan audit wajib. Jenis tidak boleh ditebak dari nama/prefix ID.
- Portal Developer dapat memilih signed managed release di Testing; administrator menyetujui merchant terdaftar. Core RPC dan Settings → Apps membaca assignment approved yang sama, dengan blocker managed aktual. Revoke distribusi bukan uninstall; suspended release tidak tetap dianggap configuration ready.
- Migration 0017 additive, backup sebelum apply. Replay request mengembalikan state terkini meskipun release sudah suspended. Managed record tidak boleh masuk registry fixture untuk membuat instalasi palsu.
- **Belum membuka installability/grant engine.** Blocker `managed_installation_not_available` dan `engine_grant_enforcement_not_available` dipertahankan hingga lifecycle dan seluruh jalur engine pilot benar-benar diimplementasikan/diverifikasi. Additional shipping methods hanya mengikuti installation/grant aktif, bukan assignment.
- Target engine lokal terisolasi atau deployment operator harus ditentukan sebelum mengubah akses operasional. Jangan menafsirkan approval kode sebagai izin deploy produksi, memilih provider checkout atau melakukan booking/pengiriman.

### 1.25 Instalasi dan grant engine lokal terisolasi — 6 September 2026

- Pengguna memilih engine lokal terisolasi. Gunakan binary API-Kurir `apps/local-managed`, database `api_kurir_managed_local`, tanpa memuat konfigurasi/credential produksi atau adapter kurir eksternal. Entry point produksi tidak diubah.
- Release managed signed + assignment approved menjadi sumber consent tersendiri (`managed-shipping-local/v1`), bukan fixture registry. Snapshot mengikat release, assignment, merchant, hash, versi, binding dan environment. Tahan shared lock release/assignment sampai commit mutation; replay membaca receipt terkini walau sumber suspended.
- Alur existing Core → consent → consume → activate → Installed Apps berlaku untuk app ID asli. Klik Install merupakan persetujuan. Jangan menginstal pada akun merchant pengguna atas nama mereka untuk pengujian; gunakan merchant sintetis.
- API-Kurir memeriksa `EngineGrantService/Check` melalui credential engine independen, bukan full Core key/developer token. Hanya `emisell`, `local-isolated`, `rates.read/settings.read`; grant efektif memerlukan source valid dan native `shipping.read` aktif. Setiap operasi diperiksa kembali, termasuk sebelum singleflight/cache. Outage, suspend, revoke, mismatch, uninstall: fail closed. Operasi berbatas waktu yang sudah terotorisasi boleh selesai saat revocation berlangsung.
- Install tidak memilih provider checkout dan tidak mendaftarkan routing capability legacy. Pilihan provider/layanan tetap di Settings → Shipping existing. Additional shipping methods menampilkan instalasi aktual; Open app menuju konfigurasi shipping existing. Satu grant aktif untuk binding provider yang sama per merchant.
- Pilot Core menggunakan enrollment merchant lokal eksplisit; kegagalan engine/grant tidak fallback ke produksi. Tarif lokal adalah sample, bukan tarif resmi provider. Booking, pickup, tracking, pembayaran, credential RajaOngkir dan checkout/order production belum dipindahkan. Perlu rollout terpisah sebelum mengaktifkan gateway produksi. Lihat ADR 0027.

### 1.26 Pemetaan izin engine dan resource — klarifikasi pilot lokal

- `shipping/v1` adalah capability; `shipping.read` adalah native grant instalasi; `rates.read` adalah operasi pemeriksaan engine. Ketiganya bukan sinonim scope resource `read_shipping`/`write_shipping`.
- Pilot tarif kini memakai jalur existing Core → API-Kurir utama lokal dengan pemeriksaan grant yang dikonfigurasi eksplisit, bukan tarif sample `apps/local-managed` pada §1.25. Kontrak grant masih `local-isolated`; ini bukan rollout produksi.
- Katalog Admin/Developer hanya menampilkan izin resource, kegunaan dan readiness; jangan menampilkan tabel capability/operasi engine seperti `shipping/v1` atau `rates.read` di halaman scope. Pemetaan teknis berada di dokumentasi integrasi. Status resource tetap dari verification backend; jangan menaikkan Active karena tarif lokal berhasil. `settings.read` yang dikenali kontrak bukan bukti seluruh endpoint pengaturan engine utama sudah dilindungi grant.
- Detail scope, batas enforcement legacy, dan syarat perluasan: `docs/shipping-scope-mapping.md`. Tidak ada perubahan grant, manifest immutable, credential atau instalasi oleh pembaruan dokumentasi ini.

### 1.27 Embedded Apps universal — fondasi identitas

- Arah embedded adalah UI aplikasi milik developer di iframe Core, bukan website login Komerce yang dibungkus iframe. RajaOngkir consumer awal; protokol tidak spesifik shipping.
- Identity token berbeda dari resource access token dan credential provider. Token identitas mengikat merchant, staf, app-client, app dan installation; short-lived, signature Ed25519, current-access check wajib. Jangan menjadikan JWT valid sebagai grant resource.
- Fondasi Go `pkg/embedded` dan `internal/oauth/embedded` tersedia, tetapi adapter current-access, launch metadata signed, endpoint issuance, bridge browser dan token exchange belum terhubung. DILARANG menampilkan endpoint atau Open app seolah aktif dari library ini saja.
- Panduan API dan gate rollout: `docs/embedded-apps.md`. Release tanpa verified launch binding tetap tidak launchable; jangan menggunakan health/API/OAuth URL sebagai iframe URL. Tidak ada grant/credential/instalasi lama berubah otomatis.

### 1.28 Reference launch dan bridge embedded

- `pkg/embedded.Launch` mengikat URL, parent origin, app-client, app dan release digest dengan signature domain terpisah. URL API/health tidak otomatis launch URL. Resolver wajib mencocokkan current installation/release, bukan request browser.
- `internal/oauth/embedded.LaunchHandler` tersedia sebagai adapter reference BFF Core POST dengan no-store, exact Origin, body installationId saja dan mandatory session/CSRF callback. Belum mounted live. Callback test tidak boleh dijadikan adapter produksi.
- `pkg/embedded/bridge.mjs` menyediakan shell iframe, parent bridge dan client identity request dengan exact origin/window, nonce, timeout dan invalidasi respons setelah navigasi. Token hanya di memori, tidak di URL atau storage persisten.
- Reference tests bukan kesiapan seller: storage/review launch persisten, real Core auth/permission resolver, audit/rate-limit, key provisioning, browser QA dan token exchange belum selesai. Tidak ada perubahan Open app merchant atau grant existing pada increment ini.

### 1.29 Penyimpanan review launch persisten

- Migration 0018 menambahkan launch immutable keyed app/client/release digest dan audit keputusan. Tidak membackfill atau mengaktifkan instalasi lama.
- Repository PostgreSQL launch tersedia dengan submitted/approved/rejected/revoked, approval signed atomik, retry actor/reason/revision yang cocok dan resolver current-state tanpa cache. Hanya diaplikasikan/test pada `emisell_local_test` dalam increment ini.
- API submit/review masih memerlukan application gate ownership, current signed release dan app-client. Repository tidak boleh diekspos langsung ke browser. Penyimpanan ini belum mengaktifkan Open app atau listener sesi Core.
- Backup/apply eksplisit sebelum binary baru dipakai dengan database development karena verifikasi startup mensyaratkan migration terbaru; jangan restart layanan pengguna otomatis atau drop data untuk rollback.

### 1.30 API review launch dengan sesi portal

- API embedded-launches tersedia pada komposisi opt-in `HandlerWithEmbeddedReviews`; default listener tetap kompatibel. Key dan parent origin wajib dari operator, tidak diprovision otomatis.
- Submit developer dan approval administrator memakai current signed integration release + app-client siap dengan lock, ownership organisasi, exact origin endpoint terverifikasi, serta immutable digest. Reject/revoke tidak memerlukan private key. Status tetap `launchable:false`.
- App-client integration release bukan app-client untuk installation managed shipping. Jangan menghubungkan sesi Core atau menerbitkan identity token hanya karena launch approved; binding installation–release–client dan grant aktual wajib tersedia. Gap ini belum ditutup oleh endpoint review.
- OpenAPI `api/openapi/embedded-launches.v1.json` dan `docs/embedded-apps.md` membedakan endpoint opt-in dari server default. Pengujian persistent menggunakan sesi portal asli dan proof endpoint test, tidak mengubah instalasi toko pengguna.

### 1.31 Demo embedded lokal terisolasi

- Pengguna memilih aplikasi demo lokal yang hanya menampilkan identitas toko sintetis. `cmd/embedded-demo` melayani parent loopback 4320 dan app loopback 4321, memakai bridge/identity/signature existing tanpa database atau credential pengguna.
- Ini harness protokol, BUKAN merchant shell baru, login seller, instalasi sebenarnya atau bypass proof pada listener utama. State revocation demo bukan Uninstall persistent. Callback identitas sintetis tidak boleh dipasang ke Core/portal production.
- Identitas/token 60 detik, renewal, verifikasi backend, CSRF/origin/frame isolation dan pencabutan diuji; tidak mengubah grant, tarif, order atau instalasi Emisell Kurir. Dokumentasi endpoint demo terpisah di `docs/embedded-demo.md`, tidak dicampur dengan API production Admin.
- Gap binding installation–release–client, real Core session, dan Open app Dashboard pada §1.30 tetap belum selesai. Jangan menganggap demo yang berhasil sebagai aktivasi embedded seller.

### 1.32 Mode pembukaan aplikasi

- Mode launch `embedded` dan `external` mengikuti ADR 0028. Ini mode tampilan, bukan runtime/provider atau permission baru. Field kosong mempertahankan canonical bytes/signature embedded historis; mode explicit ikut signature dan immutable.
- Detail App clients menyediakan pengajuan URL/mode Developer dan review Administrator pada komposisi API opt-in. Error endpoint bukan konfigurasi kosong; approval tetap `launchable:false` sampai binding instalasi dan sesi Core selesai.
- External mengembalikan URL terverifikasi setelah current authorization, tanpa identity token atau SSO otomatis; embedded token/bridge menolak external. Kedua mode tetap memerlukan grant yang sesuai untuk akses data. Tidak mengaktifkan instalasi seller lama atau mengubah tarif API-Kurir.

### 1.33 Pilot embedded di Dashboard seller existing

- ADR 0029 memperbolehkan demo `embedded-local-demo` khusus localhost dengan enrollment merchant eksplisit, scope/capability kosong dan policy development terpisah. Bukan bypass review untuk public app.
- Install tetap melalui consent/lifecycle Platform. Open app berada di Dashboard seller existing, bukan merchant shell baru. Token pendek diterbitkan Core berdasarkan sesi staf serta keputusan ephemeral `local_embedded_access` Platform; introspeksi memeriksa ulang keduanya.
- Cookie/API key tidak dikirim ke backend app, token tidak disimpan browser. Key/cache in-memory hanya pilot. Production dilarang. Tarif/API-Kurir/order tidak berubah.
- Jangan menekan Install atas nama seller; siapkan preview dan uji lifecycle pada database terisolasi. Halaman 4320 tetap demo sintetis; integrasi seller menggunakan 4321/seller di iframe localhost:3000.

### 1.34 Standar UI aplikasi embedded

- Embedded app WAJIB menggunakan Emisell UI Kit untuk komponen tersedia; komponen khusus mengikuti token/pola interaksi dan review desain. UI dimiliki developer, shell Dashboard milik Emisell; CSS tidak diwariskan dari parent iframe.
- UI Kit terpisah dari Bridge, identity dan grants. Tidak membuat renderer API-to-UI atau menggandakan sidebar/login seller di app. External app boleh memakai desain sendiri; extension tanpa dashboard tetap Settings dan tidak muncul sidebar Apps.
- Fondasi lokal `pkg/appui` 0.1.0 dan consumer `cmd/embedded-demo` tersedia tanpa dependency baru. Panduan developer/checklist: `docs/embedded-ui-kit.md`, ADR 0030. Enforcement review UI otomatis, distribusi SDK publik dan sinkronisasi token lintas repo belum tersedia; jangan mengklaim sebaliknya.
- Rollout incremental, evidence review versioned kelak additive; jangan menulis ulang signed release atau memutus grant instalasi lama karena perubahan style.
- Paket `@emisell/app-ui` private dapat di-pack lokal; belum dipublish. Regression browser `pkg/appui/browser-qa.cjs` hanya menguji dan mencabut sesi demo sintetis, tidak installation seller. QA desktop/mobile/teks 200% dan keyboard tidak menggantikan review publikasi atau audit aksesibilitas menyeluruh.

### 1.35 Binding instalasi UI yang direview — lifecycle opt-in

- Update terbaru: komposisi runtime opt-in via `.local/reviewed-ui.json`, distribusi RPC dengan executionProfile UI, sesi Core development dan consumer Dashboard existing telah diimplementasikan. Default tetap nonaktif tanpa konfigurasi HTTPS/key eksplisit. Embedded identity 60 detik dengan current seller/source introspection, external tanpa token. Belum rollout/uji live release UI umum pada toko pengguna; demo dan shipping tetap kompatibel. Catatan ini menggantikan keterangan integrasi belum diimplementasikan di milestone historis berikutnya; lihat `docs/ui-release-api.md` dan Core `CORE_REVIEWED_UI.md`.

- Adapter current source persisten `bootstrap.ReviewedUIInstallSource` kini tersedia dan diuji untuk embedded/external, tanpa mounting listener/Core. Memegang lock release/assignment/client/launch; `uiBinding.assignmentId` mengikat consent agar replacement tidak menghidupkan instalasi lama. Testing tetap blocked sampai RPC/Core disambungkan. Tidak ada perubahan installation live atau key.

- Update terbaru: App clients dan Testing existing menerima release UI signed, dengan sumber/FK `ui` terpisah (migration 0020), signature/ownership/current status diverifikasi. Tidak menambah menu atau mengonversi sumber lama. UI assignment tetap `installable:false` dan belum didistribusikan ke Core; penghubung instalasi/launch umum masih tahap berikutnya. Catatan ini menggantikan keterangan assignment/app-client UI belum tersedia di milestone historis di bawah.

- Increment portal terbaru: menu **Aplikasi dengan UI** di kedua portal memakai API release UI existing. Server merangkai rute tersebut tanpa mengganti pilot/shipping. Migration 0019 telah diterapkan lokal setelah backup; key UI privat diprovision terpisah. Assignment UI, app-client UI dan launch umum tetap belum tersedia. Detail rollout terkini ada pada `docs/ui-release-api.md`, menggantikan status opt-in/menu belum tersedia pada catatan milestone sebelumnya.

- Kontrak UI-only `pkg/uirelease` dan CLI `ui-release-validate` tersedia sesuai ADR 0032. Mendukung mode eksplisit embedded/external tanpa scope bisnis; validator bukan authoring persisten atau izin install. Assignment UI masih belum tersedia.
- Authoring/review UI persisten tersedia pada komposisi API opt-in `UIReleaseHandler`, migration 0019, sesuai `docs/ui-release-api.md`. App baru server-generated, ownership sesi, snapshot immutable, revision/idempotency/audit dan signing key eksplisit. Default server/menu portal belum disambungkan; assignment/launch UI tetap belum tersedia. Jangan restart binary baru sebelum backup/apply migration development secara eksplisit.

- ADR 0031 menambahkan `reviewed-ui/v1` tanpa scope/capability bisnis. Snapshot consent mengikat `uiBinding` immutable: release ID, app-client, digest, URL, mode, parent dan signature. Field kosong mempertahankan snapshot lama.
- Activate dan `WithReviewedUIAccess` memerlukan current source yang cocok dan signature valid dengan key eksplisit. IssueToken bisnis ditolak; uninstall tetap bekerja ketika source menolak/tidak tersedia.
- Default server belum memiliki adapter current source policy ini. Source terkontrol dalam test PostgreSQL bukan review portal production. DILARANG menyambungkan DTO katalog langsung ke port untuk membuka installability.
- Authoring UI-only, assignment, adapter current review/client/release, RPC/Core dan sidebar generik masih perlu rollout terkoordinasi. Instalasi demo/managed shipping existing tidak dikonversi atau diubah otomatis.

## 2. Definisi Engine Emisell

Framework HTTP **bukan** engine utama. `net/http` dan Chi hanya transport adapter yang harus dapat diganti tanpa mengubah aturan bisnis.

Engine Emisell terdiri dari:

- domain model dan invariant;
- capability contract dan resolver;
- app manifest;
- installation lifecycle;
- permission dan scope grant;
- event contract;
- runtime dan security boundary;
- versioning, compatibility, review, dan signing.

Business/domain layer DILARANG bergantung pada `chi.Context`, framework-specific context, detail HTTP, SQL row, NATS message, Temporal workflow, atau tipe SDK provider. Transport dan infrastruktur harus menjadi adapter di batas luar sistem.

## 3. Keputusan Arsitektur Utama

### 3.1 Bentuk aplikasi

- Gunakan **Go 1.26+**.
- Mulai sebagai **modular monolith dalam satu repository**.
- Terapkan boundary modul yang keras agar modul dapat diekstrak menjadi service jika kebutuhan skala dan operasional sudah terbukti.
- Jangan memulai dengan microservices. Ekstraksi service harus didorong oleh bukti kebutuhan, ownership, scaling, isolation, atau deployment yang independen; bukan spekulasi.

### 3.2 HTTP dan kontrak API

- HTTP server: Go `net/http` dengan `github.com/go-chi/chi/v5` sebagai router tipis.
- Komunikasi internal: **ConnectRPC + Protobuf + Buf**.
- API developer/external: **HTTPS REST/JSON**, OAuth, dan webhook.
- Kontrak internal maupun eksternal harus explicit, versioned, dan dapat diuji.
- Jangan bocorkan tipe transport ke domain/application layer.
- GraphQL bukan kebutuhan awal dan hanya boleh ditambahkan jika ada use case yang terukur.

### 3.3 Data dan storage

- Database utama: **PostgreSQL**.
- Driver/toolkit: **pgx** dengan SQL explicit; `sqlc` boleh dipakai sebagai implementation detail jika bermanfaat.
- Repository interface dimiliki oleh domain/application module yang menggunakannya. Domain tidak boleh mengetahui pgx, sqlc, atau PostgreSQL.
- Perubahan schema harus melalui migration yang versioned, dapat di-review, dan memiliki migration path yang aman.
- Artifact dan object storage: **S3-compatible storage**; gunakan S3 pada production atau MinIO untuk local/self-hosted.

### 3.4 Events dan workflow

- Gunakan **NATS JetStream** untuk application events, durable messaging, fan-out, dan subscription.
- Event contract harus versioned, tenant-aware, idempotent untuk dikonsumsi ulang, serta memiliki metadata correlation/causation yang memadai.
- Gunakan outbox/inbox atau mekanisme konsistensi setara saat transaksi database dan publish/consume event harus selaras.
- Gunakan **Temporal hanya untuk durable workflow** yang benar-benar multi-step, long-running, menunggu callback eksternal, atau memerlukan retry/recovery terkoordinasi.
- Jangan mengirim setiap event melalui Temporal. Event fan-out biasa tetap melalui NATS JetStream.

### 3.5 Runtime extension

- Runtime code extension: **wazero**.
- WASM dijalankan dengan deny-by-default: tanpa filesystem, process execution, environment, clock, atau network mentah kecuali secara eksplisit diberikan oleh host capability.
- Terapkan limit memory, execution time/fuel, concurrency, payload, dan outbound access.
- Remote app berkomunikasi melalui HTTPS dan webhook yang terautentikasi serta ditandatangani.
- UI extension awal harus menggunakan boundary terisolasi, misalnya sandboxed iframe, contract message yang versioned, Content Security Policy, dan allowlist extension point.

### 3.6 Authorization dan secret

- Mulai dengan **native scopes** dan grant per installation/tenant.
- Gunakan **OpenFGA hanya ketika relasi authorization benar-benar menjadi kompleks**, misalnya organization, workspace, store, staff, role, developer, app, dan resource tidak lagi aman atau efisien dimodelkan dengan scope sederhana.
- Secret dan credential harus dienkripsi saat disimpan, tidak boleh muncul pada log/event/audit payload, dan harus dapat dirotasi serta dicabut.

### 3.7 Observability

- Structured logging: `log/slog`.
- Tracing dan context propagation: **OpenTelemetry**.
- Metrics: **Prometheus**.
- Log, trace, dan metrics harus membawa identifier yang aman dan relevan seperti tenant, installation, app, request, correlation, dan workflow ID; jangan memasukkan secret atau data pribadi yang tidak diperlukan.

## 4. Capability-First Architecture

Capability adalah kontrak platform, bukan nama provider. Setiap capability WAJIB:

- memiliki ID dan versi yang stabil;
- mendefinisikan request, response, error model, timeout, idempotency, dan compatibility;
- bersifat tenant-aware dan diselesaikan melalui installation aktif;
- mempunyai authorization check sebelum invocation;
- menghasilkan audit trail dan telemetry;
- tidak mengekspos istilah, endpoint, credential, atau tipe khusus provider;
- memiliki contract test yang dapat dijalankan terhadap setiap implementation.

Capability Resolver harus mempertimbangkan paling sedikit `tenant_id`, capability ID/version, status installation, grant, compatibility, dan routing policy. Tidak boleh ada fallback lintas tenant.

### 4.1 Reference capability pertama

Gunakan **`shipping/v1`** sebagai reference capability aplikasi umum. `payment/v1` dipertahankan hanya sebagai reference historis/local fixture; bukan pilihan developer atau jalur payment gateway checkout (§1.18). Alur install -> activate -> resolve -> invoke -> switch provider -> uninstall tetap harus generik.

Baseline `payment/v1` historis (compatibility, bukan template publik):

- `create`;
- `capture`;
- `refund`;
- `status`.

Baseline `shipping/v1`:

- `get_rates`;
- `create`;
- `track`.

Nama operasi final harus ditetapkan di Protobuf/OpenAPI dan tidak boleh berubah diam-diam. Perubahan breaking harus membuat versi capability baru, bukan mengubah semantik versi lama.

## 5. Jenis App

Platform mendukung tiga jenis app berikut.

### 5.1 Aplikasi Integrasi (identifier teknis: remote)

Jenis default untuk shipping, CRM, ERP, accounting, marketplace, messaging, analytics, AI, dan fulfillment. Payment gateway checkout bukan aplikasi umum (§1.18). App berjalan di luar process platform dan dipanggil melalui HTTPS. Callback masuk melalui webhook yang diverifikasi. Daftar use case ini merupakan arah arsitektur, bukan klaim seluruh kontrak authoring/runtime sudah tersedia.

### 5.2 WASM extension

Digunakan untuk logic berlatensi rendah yang aman dijalankan dekat dengan platform, misalnya checkout validation, discount rule, fraud rule, shipping rule, price/order/product transformer. Semua akses dilakukan melalui host functions yang sempit dan dikendalikan scope.

### 5.3 UI extension

Digunakan untuk menyisipkan UI pada extension point yang terdaftar, misalnya:

- `admin.order.sidebar`;
- `admin.order.actions`;
- `admin.product.sidebar`;
- `admin.product.actions`;
- `admin.customer.sidebar`;
- `admin.dashboard.widget`;
- `checkout.payment` (reserved internal; bukan extension point publik pemrosesan pembayaran);
- `checkout.delivery`;
- `settings.apps`.

UI extension tidak boleh memperoleh akses ke session, DOM, API, atau data tenant di luar contract dan scope yang diberikan.

## 6. Modul Platform

Boundary awal modular monolith:

- **app/registry**: identitas app, version, release, listing, compatibility, dan publication state;
- **installation**: install, activation, upgrade, suspension, dan uninstall per tenant;
- **capability**: contract registry, implementation registration, resolution, routing, dan invocation;
- **permission**: catalog scope, requested scope, consent, grant, revocation, dan enforcement;
- **oauth**: authorization flow, callback, token lifecycle, rotation, dan revocation;
- **event**: event contract, publisher, subscription, consumer policy, dan delivery state;
- **webhook**: endpoint registration, signing, retry, replay protection, dead letter, dan delivery audit;
- **runtime**: adapter remote dan sandbox WASM;
- **extension**: registration dan delivery UI extension/extension point;
- **developer**: developer identity, organization, ownership, dan publish access;
- **review**: validation, security scan, approval, artifact checksum, dan signing;
- **billing**: plan, entitlement, subscription, usage, dan settlement bila diperlukan.

Setiap modul memiliki domain, use case/service, port, dan adapter sendiri sesuai kebutuhan. Jangan membuat abstraksi kosong hanya demi menyeragamkan folder.

## 7. Dependency dan Boundary Rules

Aturan berikut WAJIB dipatuhi:

1. Domain tidak bergantung pada transport, database driver, message broker, workflow engine, object storage SDK, atau provider SDK.
2. Application/use-case layer mengorkestrasi domain melalui port/interface yang sempit.
3. Adapter HTTP, ConnectRPC, PostgreSQL, NATS, Temporal, S3, remote HTTP, dan wazero mengimplementasikan port pada batas luar.
4. Handler hanya melakukan authentication context, validation/decoding, pemanggilan use case, dan pemetaan response/error. Business rule tidak boleh hidup di handler.
5. Modul tidak boleh membaca atau menulis tabel milik modul lain secara langsung. Gunakan public module service/port atau event contract.
6. Jangan membuat shared package yang menjadi tempat campuran domain. Kode bersama hanya untuk primitive yang benar-benar stabil dan lintas modul.
7. Dependency mengarah ke contract/domain yang stabil, bukan ke implementation detail.
8. Semua query dan mutation tenant-scoped harus membawa tenant context terotorisasi. Admin platform dan ownership developer memiliki scope principal tersendiri; jangan mewajibkan workspace merchant untuk operasinya. Tenant ID dari payload eksternal tidak boleh dipercaya tanpa dicocokkan dengan principal terautentikasi.
9. Tidak boleh ada import provider SDK pada domain/business layer Core atau endpoint provider-specific pada public capability contract.
10. Provider adapter boleh mengetahui provider; resolver, contract, dan business layer Core tidak boleh mengetahuinya. Integrasi pembayaran internal boleh memiliki adapter provider di boundary modul pembayaran (§1.18), bukan di App Platform publik.
11. Cross-module transaction harus diminimalkan. Jika tidak dapat atomik, gunakan event/outbox dan state machine yang dapat direkonsiliasi.
12. Retry hanya aman untuk operasi idempotent atau yang memiliki idempotency key dan deduplication state.

## 8. App Manifest, Versioning, Review, dan Signing

Manifest adalah kontrak declarative sebuah app. Schema awal adalah `emisell.app/v1`. Manifest minimal harus mendefinisikan:

- ID, nama, version, dan developer/owner;
- runtime type dan configuration yang diizinkan;
- capability beserta versinya;
- permission required/optional;
- event subscription;
- webhook declaration;
- UI extension dan target extension point;
- settings schema;
- compatibility range terhadap Emisell;
- artifact checksum dan status/signature security.

Contoh konseptual:

```yaml
schema: emisell.app/v1
id: example-shipping
name: Example Shipping
version: 1.0.0

developer:
  id: example

runtime:
  type: remote
  endpoint: https://app.example.com/emisell/v1

capabilities:
  - id: shipping
    version: v1

permissions:
  required:
    - orders.read
    - shipping.read
    - shipping.write

events:
  subscribe:
    - order.created.v1
    - order.cancelled.v1

webhooks:
  endpoint: /webhooks/emisell

extensions:
  - target: admin.order.sidebar
    type: iframe
    path: /extensions/order

compatibility:
  emisell: ">=1.0.0 <2.0.0"

security:
  signed: true
```

Aturan release:

- Version app dan capability menggunakan semantic versioning yang konsisten.
- Release yang sudah dipublish bersifat immutable.
- Artifact harus divalidasi, dipindai, diberi checksum, direview sesuai policy, lalu ditandatangani.
- Signature mengikat manifest, version, checksum artifact, dan publisher identity.
- Runtime hanya memuat release yang lolos compatibility, integrity, signature, dan policy check.
- Breaking contract membutuhkan versi baru dan migration path.
- Upgrade tidak boleh memperluas scope tanpa consent baru dari tenant.

## 9. Installation Lifecycle

Installation adalah resource tenant-scoped dan harus memiliki state machine eksplisit. Nama state final dapat berkembang, tetapi transisi tidak boleh implisit.

### 9.1 Install dan activate

Alur minimum:

1. Resolve app release yang immutable dan compatible.
2. Validasi manifest, signature, integrity, policy, dan tenant eligibility.
3. Tampilkan requested scopes dan minta consent yang eksplisit.
4. Buat installation dengan idempotency key dan state awal.
5. Simpan grant per tenant dan installation.
6. Jalankan OAuth/credential setup bila diperlukan; simpan secret terenkripsi.
7. Daftarkan capability implementation, event subscription, webhook, dan UI extension yang diizinkan.
8. Inisialisasi remote/WASM runtime dan lakukan health/handshake check.
9. Aktifkan installation secara atomik atau melalui workflow yang dapat dilanjutkan kembali.
10. Terbitkan event installation aktif dan tulis audit trail.

Keputusan UX consent di Dashboard Core: **klik Install setelah daftar izin ditampilkan adalah persetujuan eksplisit**. Jangan menambahkan checkbox atau dialog persetujuan terpisah sebelum Install. UI memberi keterangan singkat bahwa klik Install menyetujui izin; backend tetap memvalidasi sesi/merchant/digest dan mencatat consent sebelum consume/activate. Membuka halaman atau reload tidak boleh memberikan consent otomatis.

Install harus aman di-retry, dapat melanjutkan kegagalan parsial, dan memiliki compensating action. Gunakan Temporal hanya bila durasi, callback, atau koordinasi multi-step memang membutuhkannya.

### 9.2 Upgrade

- Validasi compatibility dan migration sebelum mengubah active version.
- Pertahankan versi lama sampai cutover berhasil jika desain runtime memungkinkan.
- Perubahan scope memerlukan consent baru.
- Sediakan rollback atau forward-fix plan untuk perubahan berisiko.
- Jangan memodifikasi release lama di tempat.

### 9.3 Uninstall

Urutan minimum uninstall:

1. Tandai installation sedang dinonaktifkan agar invocation baru berhenti.
2. Cabut token, scope, grant, dan credential.
3. Nonaktifkan capability routing.
4. Hapus event subscription dan webhook registration/delivery yang tertunda sesuai retention policy.
5. Hentikan dan hapus instance/state WASM yang tidak lagi diperlukan.
6. Jalankan cleanup/compensation yang aman dan berbatas waktu.
7. Tandai installation ter-uninstall; simpan audit record dan data minimum sesuai retention/compliance.
8. Terbitkan event uninstall yang versioned.

Uninstall harus idempotent. Uninstall berulang tidak boleh menghidupkan kembali resource, gagal karena resource sudah tidak ada, atau menyisakan akses aktif.

## 10. Event dan Webhook Contract

- Nama event harus bernamespace dan versioned, misalnya `emisell.order.created.v1` dan `emisell.app.installed.v1`.
- Envelope minimal mencakup event ID unik, version, occurred-at, tenant ID, subject/resource ID, correlation ID, causation ID, producer, dan payload.
- Consumer wajib idempotent dan menyimpan deduplication state bila side effect tidak boleh berulang.
- Tetapkan retry dengan exponential backoff, batas attempt/age, dead-letter handling, dan mekanisme replay yang diaudit.
- Webhook harus ditandatangani, timestamped, dapat diverifikasi, dilindungi dari replay, memiliki timeout pendek, dan dikirim ulang secara aman.
- Jangan menjamin ordering global. Jika ordering diperlukan, definisikan key dan scope ordering secara eksplisit.
- Perubahan breaking pada payload menghasilkan event version baru.

## 11. Security dan Reliability Invariants

Security, tenant isolation, idempotency, retry safety, dan auditability adalah kebutuhan inti, bukan pekerjaan tahap akhir.

- Setiap access dan side effect harus diotorisasi terhadap principal, tenant, installation, capability, dan scope yang benar.
- Fail closed ketika tenant context, permission, signature, compatibility, atau installation state tidak valid.
- Terapkan least privilege dan deny-by-default pada scopes, host functions, network egress, dan extension points.
- Jangan pernah mempercayai manifest, artifact, webhook, OAuth callback, UI message, event, atau response remote app tanpa validation.
- Lindungi OAuth dengan state, PKCE bila relevan, redirect URI allowlist, expiry, dan single-use semantics.
- Enkripsi credential per installation; dukung rotation/revocation; redaksi log dan error.
- Semua operasi mutating external harus mendukung idempotency key atau memiliki semantik setara.
- Tetapkan timeout, bounded retry, circuit breaking/concurrency limit, dan backpressure pada komunikasi eksternal.
- Audit install, consent, scope change, activation, invocation sensitif, secret lifecycle, publish/review/sign, upgrade, suspension, dan uninstall.
- Jangan simpan data tenant di cache/global state tanpa key tenant yang eksplisit dan test isolation.

## 12. Struktur Repository Target

Gunakan satu repository terlebih dahulu dengan struktur target berikut. Struktur boleh bertumbuh secara incremental; jangan membuat semua direktori sebagai scaffold kosong.

```text
emisell-app-platform/
├── api/
│   ├── proto/
│   ├── openapi/
│   └── manifest/
├── cmd/
│   ├── server/
│   ├── worker/
│   └── cli/
├── internal/
│   ├── app/
│   │   ├── domain/
│   │   ├── service/
│   │   └── repository/
│   ├── installation/
│   ├── capability/
│   ├── permission/
│   ├── oauth/
│   ├── event/
│   ├── webhook/
│   ├── runtime/
│   │   ├── remote/
│   │   └── wasm/
│   ├── extension/
│   ├── developer/
│   ├── review/
│   └── billing/
├── pkg/
│   ├── appmanifest/
│   └── sdk/
├── migrations/
├── web/
│   ├── dashboard/             # Dashboard Admin, port 4317
│   ├── app-store/             # target frontend listing terpisah, port 4318
│   └── developer/             # target Portal Developer terpisah, port 4319
└── go.mod
```

Panduan penempatan:

- `api/proto`: kontrak internal ConnectRPC/Protobuf, termasuk capability yang distandardkan.
- `api/openapi`: kontrak REST/JSON external.
- `api/manifest`: JSON Schema atau schema lain untuk `emisell.app/v1`.
- `cmd`: composition root dan executable; business logic tidak ditempatkan di sini.
- `internal/<module>`: domain, use case, port, dan adapter privat platform.
- `pkg`: hanya package yang memang dirancang stabil untuk dipakai di luar module/repository. Jangan memindahkan kode ke `pkg` karena sekadar dipakai oleh dua folder.
- `migrations`: migration PostgreSQL yang immutable setelah dipakai di environment bersama.
- `web/dashboard`: frontend **Dashboard Admin**, tetap memakai checkout/tooling yang sudah ada; bukan merchant dashboard. `web/app-store` dan `web/developer` adalah target penempatan frontend terpisah, dibuat saat implementasinya benar-benar dikerjakan. Shared primitive boleh diekstrak jika diperlukan, tanpa mencampur session, state, atau permission antar-surface. Lihat ADR `docs/adr/0007-admin-store-developer-boundaries.md`.

## 13. Teknologi yang Sengaja Tidak Menjadi Fondasi

Tanpa keputusan arsitektur baru yang terdokumentasi, DILARANG menjadikan hal berikut sebagai fondasi:

- microservices sejak awal;
- Fiber atau framework HTTP lain sebagai pusat domain/platform;
- GORM sebagai core data layer;
- Go `.so` plugins untuk third-party/public apps;
- arbitrary Node.js runtime di process atau infrastructure inti;
- endpoint atau tipe provider-specific pada public capability contract, serta SDK provider pada domain/business layer Core (adapter pembayaran internal terpisah sesuai §1.18);
- Kafka sebagai kebutuhan awal;
- Kubernetes sebagai prasyarat development atau deployment awal.

Redis boleh ditambahkan sebagai cache/coordination aid jika ada kebutuhan terukur, tetapi tidak boleh menjadi source of truth domain. Teknologi yang dilarang sebagai fondasi bukan berarti selamanya tidak boleh dipakai; penggunaannya membutuhkan problem nyata, analisis trade-off, migration/exit strategy, dan Architecture Decision Record (ADR).

## 14. Aturan Kerja Codex/AI Agent

### 14.1 Sebelum coding

1. Baca `codex.md` ini sampai selesai.
2. Inspeksi status repository dan perubahan yang sudah ada. Anggap perubahan di luar task adalah milik pengguna; jangan ditimpa, dibersihkan, atau dipulihkan.
3. Pahami module boundary, contract, test, migration, dan caller yang terdampak sebelum mengedit.
4. Jangan menanyakan ulang keputusan yang sudah ditetapkan di dokumen ini.
5. Jika ambiguity minor, ambil keputusan yang paling sederhana dan paling konsisten dengan dokumen ini, lalu catat asumsi penting.
6. Jika ambiguity mengubah contract publik, data, security boundary, atau scope pekerjaan secara material, laporkan dan minta keputusan sebelum melakukan perubahan yang sulit dibalik.

### 14.2 Selama implementasi

- Kerjakan perubahan kecil, incremental, dan mudah di-review.
- Jangan melakukan rewrite besar tanpa alasan yang kuat dan migration path yang jelas.
- Pertahankan backward compatibility untuk API, capability, event, manifest, database, dan SDK. Breaking change harus disengaja, versioned, dan terdokumentasi.
- Jangan menambah dependency, framework, infrastructure, atau abstraction baru tanpa kebutuhan jelas. Utamakan standard library dan komponen yang sudah dipilih.
- Jangan menyentuh modul di luar scope kecuali benar-benar dibutuhkan untuk implementasi yang benar; jelaskan dampaknya.
- Jangan mencampurkan refactor tidak terkait dengan perubahan fitur/fix.
- Dokumentasikan keputusan arsitektur penting sebagai ADR atau dokumentasi setara, termasuk konteks, pilihan, trade-off, konsekuensi, dan migration/exit path.
- Untuk perubahan besar, buat migration path bertahap dan jelaskan compatibility window, rollout, rollback/forward-fix, serta observability-nya.
- Prioritaskan security, tenant isolation, idempotency, retry safety, auditability, dan failure recovery pada setiap desain.
- Update progress secara berkala dan singkat: apa yang selesai, apa yang sedang diverifikasi, dan blocker yang nyata.
- Laporkan temuan, risiko, kontradiksi contract, kerentanan, data migration risk, atau scope expansion segera; jangan menunggu akhir pekerjaan.

### 14.3 Verifikasi wajib

Setiap perubahan harus melewati verifikasi yang relevan terhadap scope:

- formatting dan generated-code consistency;
- build package/binary yang terdampak;
- unit test dan contract test;
- integration test bila menyentuh PostgreSQL, NATS, Temporal, object storage, OAuth, webhook, remote runtime, atau WASM;
- lint/static analysis yang dikonfigurasi repository;
- migration validation untuk perubahan schema;
- security/isolation/idempotency/retry test untuk jalur sensitif;
- pemeriksaan diff agar tidak ada perubahan di luar scope.

Jangan menyatakan test berhasil jika tidak dijalankan. Jika verifikasi tidak dapat dijalankan, sebutkan dengan jelas apa yang belum diverifikasi, alasannya, risikonya, dan perintah/check yang harus dilakukan berikutnya.

### 14.4 Definition of done

Pekerjaan baru selesai jika:

- perilaku yang diminta sudah diimplementasikan, bukan hanya scaffold atau TODO;
- contract dan boundary tetap benar;
- error path, retry, idempotency, tenant isolation, permission, dan audit path yang relevan sudah ditangani;
- migration dan compatibility path tersedia bila diperlukan;
- dokumentasi/ADR/contract diperbarui bila perilaku atau keputusan berubah;
- build, test, lint, dan verifikasi relevan sudah dijalankan dan hasilnya dilaporkan;
- diff akhir hanya berisi perubahan yang diperlukan.

Scaffold, kode yang hanya compile, happy-path tanpa verifikasi, atau dokumentasi tanpa implementasi **bukan** completion untuk task implementasi.

## 15. Urutan Prioritas Saat Aturan Bertabrakan

Catatan implementasi reviewed UI: gate current app-client/launch persisten tersedia sesuai ADR 0031. Pemanggil wajib memegang signed release/assignment lock sebelum gate; gate bukan izin install. Authoring dan assignment UI-only serta RPC/Core umum belum aktif. Jangan mengubah release shipping menjadi UI-only dengan membuang capability atau scope.

Gunakan prioritas berikut:

1. Keamanan, tenant isolation, dan integritas data.
2. Capability/domain contract dan backward compatibility.
3. Kebenaran lifecycle, idempotency, retry, dan auditability.
4. Boundary modular monolith dan kemudahan migration.
5. Operability dan observability.
6. Kesederhanaan implementasi dan developer experience.
7. Optimasi performa yang sudah terukur.

Jika keputusan baru perlu menyimpang dari dokumen ini, jangan menyimpang secara diam-diam. Dokumentasikan alasan dan konsekuensinya, buat ADR, serta sediakan migration path sebelum mengubah fondasi.

## 16. Arah Desain Dashboard

Dashboard WAJIB memiliki desain SaaS modern, rapi, dan terasa matang, mengikuti pola antarmuka produk terkini. Arahan ini berlaku untuk **Dashboard Admin, App Store, dan Portal Developer** sesuai bagian 1.2, bukan dashboard merchant workspace. Kualitas visual, keterbacaan, dan kemudahan menyelesaikan pekerjaan adalah bagian dari acceptance criteria.

### 16.1 Karakter visual

- Gunakan permukaan netral, kontras teks yang jelas, dan satu warna aksen utama sesuai identitas Emisell. Pertahankan token brand yang sudah ada; bila belum tersedia, tetapkan baseline token secara konsisten sebelum membuat halaman.
- Gunakan tipografi sans-serif yang bersih dengan hierarki judul, isi, dan metadata yang tegas. Utamakan font yang sudah tersedia atau system font; angka pada tabel dan metrik menggunakan tabular numerals jika tersedia.
- Gunakan spacing berbasis kelipatan 4px, alignment yang presisi, serta kepadatan informasi yang konsisten dalam satu halaman. Ruang kosong harus membantu pembacaan tanpa membuat pengguna terlalu banyak menggulir.
- Sebagai baseline desain, gunakan radius 8–12px, border halus, dan shadow ringan pada permukaan yang perlu dibedakan. Kelola warna, spacing, radius, tipografi, dan elevation melalui design tokens.
- Gunakan satu keluarga ikon yang konsisten. Tombol berikon tanpa teks harus memiliki accessible name; emoji bukan ikon navigasi utama.
- Gunakan light theme sebagai baseline, dengan token semantik yang siap mendukung dark theme. Jika dark theme disediakan, verifikasi seluruh komponen dan state dalam kedua tema.
- Gunakan animasi singkat dan fungsional, dengan baseline transisi 120–200ms untuk feedback atau perpindahan panel. Hormati `prefers-reduced-motion`.
- Hindari gradient berlebihan, glassmorphism pada area data, shadow berat, kartu dekoratif berulang, dan animasi yang mengganggu pembacaan. Tampilan harus tetap ringan dan relevan untuk pekerjaan operasional.

### 16.2 Layout dan navigasi

- Gunakan application shell konsisten: sidebar ringkas, header halaman dengan judul dan aksi utama yang jelas, serta area konten yang adaptif. Sidebar menjadi drawer pada layar kecil.
- Admin menampilkan konteks administrasi platform; developer menampilkan organisasi/app miliknya; App Store menampilkan katalog publik. Jangan menambahkan selector tenant/store merchant pada shell portal. Bedakan sandbox dan production ketika keduanya tersedia.
- Susun navigasi sesuai surface: admin berfokus pada review, developer, publikasi, dan operasional platform; developer pada app miliknya, release, submission, dan integrasi; App Store pada penemuan dan detail aplikasi. Bukan tiga tab di dashboard merchant yang sama.
- Tampilkan detail teknis seperti webhook delivery, runtime error, dan contract version pada konteks developer/admin atau detail lanjutan yang relevan.
- Utamakan satu aksi utama per halaman atau panel. Gunakan tab untuk bagian setara, drawer untuk detail singkat, dan halaman khusus untuk alur yang kompleks.
- Gunakan komponen dan pola interaksi bersama agar form, filter, status, tabel, dialog, dan navigasi terasa konsisten di seluruh dashboard.

### 16.3 Penyajian data dan alur app store

- Overview Admin menampilkan antrean review, release/publication state, dan kesehatan platform berdasarkan data terotorisasi. Overview Developer menampilkan app miliknya dan progres submission/release. Jangan memakai installation merchant sebagai metrik utama. Metrik dan grafik hanya ditampilkan jika datanya tersedia dan berguna.
- Gunakan katalog grid pada App Store dan tabel/list app/release/submission pada Admin/Developer Portal. Listing wajib berasal dari release published yang diizinkan; jangan menampilkan draft, rejected, atau suspended sebagai listing aktif. Search, filter, sorting, dan pagination harus bekerja ketika ditampilkan.
- App card menampilkan nama, ikon, kegunaan singkat, kategori/capability, dan status atau harga bila tersedia. Gunakan data terverifikasi untuk rating, badge, dan klaim review.
- Tabel harus mudah dipindai, dengan alignment angka yang tepat, status berlabel, dan aksi baris yang ringkas. Pertahankan filter dan posisi navigasi saat pengguna kembali dari detail jika memungkinkan.
- Tampilkan publication/review lifecycle dengan label yang sesuai state backend, seperti "Draft", "Diajukan", "Dalam review", "Ditolak", "Dipublikasikan", dan "Ditangguhkan". Jangan mencampurkan state release dengan status installation tenant.
- Detail App Store menjelaskan capability dan izin aplikasi. Jika install/consent diintegrasikan, arahkan ke konteks Core yang terautentikasi. Lifecycle install/activate/uninstall tetap milik layanan backend/Core, bukan fitur pengelolaan workspace dalam portal ini.
- Setiap tampilan data memiliki loading, empty, error, dan success state yang dirancang. Empty state harus menawarkan langkah berikutnya; data yang belum tersedia tidak boleh ditampilkan sebagai angka nol atau hasil sukses palsu.

### 16.4 Responsivitas, aksesibilitas, dan verifikasi

- Dashboard harus nyaman digunakan di desktop, tablet, dan mobile. Hindari overflow halaman; bila tabel lebar memerlukan scroll horizontal, batasi scroll pada tabel dan pertahankan konteks kolom penting.
- Semua kontrol harus dapat dioperasikan melalui keyboard, memiliki focus indicator, label yang jelas, dan target interaksi yang memadai. Status tidak boleh bergantung pada warna saja.
- Modal/drawer interaktif harus mengelola fokus dengan benar, mendukung penutupan yang sesuai, dan mengembalikan fokus ke pemicunya. Form menampilkan error di dekat field terkait.
- Sebelum menyatakan implementasi UI selesai, inspeksi hasil render pada desktop dan mobile; periksa alignment, overflow, kontras, teks panjang, serta loading/empty/error state. Jalankan alur utama menggunakan browser dan verifikasi hasilnya terhadap state aplikasi.
- Semua tombol, filter, navigasi, dan form yang ditampilkan harus menjalankan fungsi yang dijanjikan. Preview dengan data contoh harus ditandai jelas dan tidak boleh dilaporkan sebagai integrasi yang selesai.
- Arahan desain ini tidak menetapkan framework frontend atau dependency UI baru. Gunakan fondasi proyek yang tersedia dan tambahkan dependency hanya jika ada kebutuhan jelas sesuai bagian 14.

### 16.5 Referensi desain

Referensi ditinjau pada 5 September 2026. Gunakan [Shopify App Design — Layout](https://shopify.dev/docs/apps/design/layout) sebagai acuan kualitas layout adaptif, serta [Vercel Geist](https://vercel.com/geist/introduction) sebagai acuan konsistensi komponen, tipografi, dan warna. Keduanya bukan perintah mengadopsi library, menyalin brand, atau membangun merchant workspace. Detail visual di atas merupakan keputusan desain Emisell dan dapat disempurnakan melalui verifikasi UI.

### 16.6 Baseline redesign Admin disetujui — 6 September 2026

Pengguna menyetujui seri mockup berikut sebagai **arah visual Dashboard Admin**, dan meminta keputusan dicatat sebelum penggantian desain. Status saat pencatatan: gambar konsep saja, **belum implementasi UI**. Tidak memperluas pekerjaan ke Dashboard seller, App Store, atau Portal Developer. Halaman Developer di bawah adalah pengelolaan developer oleh admin, bukan Portal Developer.

Referensi PNG disalin ke repository agar tidak bergantung pada cache percakapan. Gambar dihasilkan dengan imagegen bawaan; seluruh nama contoh, angka, tanggal, status, peran, endpoint ilustratif, dan aksi tambahan bukan bukti fitur tersedia. Kontrak backend, izin dan data aktual mengalahkan detail gambar.

| Halaman | Referensi | Fokus implementasi |
|---|---|---|
| Ringkasan | [Mockup](docs/design/admin-2026-09-06/ringkasan.png) | Ringkasan platform, antrean review, kesehatan dan aktivitas; metrik/grafik hanya jika sumber tersedia. |
| Aplikasi | [Mockup](docs/design/admin-2026-09-06/aplikasi.png) | Tabel, filter, versi, status publikasi; pisahkan Embedded/External/Tanpa UI. |
| Developer | [Mockup](docs/design/admin-2026-09-06/developer.png) | Organisasi developer, status dan aplikasi berdasarkan akses admin. |
| Pengajuan review | [Mockup](docs/design/admin-2026-09-06/pengajuan-review.png) | Antrean, hasil pemeriksaan, detail dan tindakan sesuai permission. |
| Rilis UI | [Mockup](docs/design/admin-2026-09-06/rilis-ui.png) | Versi antarmuka aplikasi, mode launch dan verifikasi; bukan versi Dashboard Emisell. |
| Katalog scope | [Mockup](docs/design/admin-2026-09-06/katalog-scope.png) | Tabel resource, akses, mapping gateway dan evidence readiness. |
| API keys | [Mockup](docs/design/admin-2026-09-06/api-keys.png) | Key backend full-access, masking, generate/revoke sesuai kontrak. |
| Dokumentasi API | [Mockup](docs/design/admin-2026-09-06/dokumentasi-api.png) | Kelompok endpoint, schema, autentikasi, request/response dan error. |
| Aktivitas | [Mockup](docs/design/admin-2026-09-06/aktivitas.png) | Audit terotorisasi, filter aktor/kategori/hasil dan detail. |
| Login Admin | [Mockup](docs/design/admin-2026-09-06/login.png) | Email/password, tanpa signup publik atau credential contoh yang berfungsi. |
| Pengaturan → Staf | [Mockup](docs/design/admin-2026-09-06/kelola-staf.png) | Tim internal, berbeda dari developer; peran/status berdasarkan kontrak aktual. |

#### Keputusan visual dan navigasi

- Pertahankan brand Emisell Apps, light theme, sidebar putih dengan selection lavender, aksen ungu, teks gelap, border halus, radius 8–12px dan ikon konsisten. Gunakan token/component existing, bukan CSS salinan per halaman.
- Sidebar konseptual: Ringkasan, Aplikasi, Developer, Pengajuan review, Rilis UI, Katalog scope, API keys, Dokumentasi API, Aktivitas; Pengaturan di bawah. Jangan menghapus akses fitur existing (misalnya App clients/Testing/Rilis integrasi) hanya karena tidak tergambar; petakan penempatannya sebelum perubahan navigasi.
- Usulan mengganti nama Rilis UI menjadi “Rilis antarmuka aplikasi” atau tab Rilis di detail aplikasi **belum keputusan final**. Jangan memindahkan route atas dasar usulan tersebut saja.
- Prioritaskan tabel mudah dibaca, filter ringkas, status dengan teks, navigasi detail dan satu aksi utama. Responsive drawer/sidebar, keyboard, focus, dialog, dan state loading/empty/error mengikuti §16.4.

#### Guardrail agar mockup tidak mengubah bisnis

- DILARANG memakai angka/status sintetis gambar sebagai data live, seed, fallback sukses atau indikator readiness. Angka instalasi hanya agregat terotorisasi, bukan pengelolaan merchant; bila belum tersedia, tampilkan alasan/empty state.
- Review, signing, publikasi, assignment, consent, installation/grant dan provider aktif tetap state terpisah. “Terverifikasi” tidak otomatis installable/published. Extension tanpa UI tetap tidak membutuhkan rilis antarmuka.
- Scope Active/Plan memakai sumber verification backend yang sama dengan dokumentasi. Jangan menampilkan capability `shipping/v1` di katalog scope atau menyalin status hijau dari mockup.
- API key tetap backend full-access, tanpa tenant ID, expiry atau pilihan izin layanan; secret sekali tampil dan server-only. Masking bukan tombol untuk mengambil kembali secret.
- Dokumentasi memakai kontrak generated existing; bedakan Core REST, ConnectRPC, gateway, port/origin serta batas development. Kode dalam raster adalah ilustrasi, bukan sumber endpoint atau kode siap salin.
- Undang staf/developer, lupa password, ekspor, role “Pemilik/Viewer”, last-active, metrik dan pengelompokan log pada mockup WAJIB diaudit terhadap kemampuan backend. Tidak boleh membuat tombol mati, otorisasi hanya frontend atau mengklaim fitur tersedia. Label role existing tidak boleh dipetakan ke privilege baru tanpa kontrak.
- Ringkasan sesi rutin pada gambar Aktivitas bukan izin membuang audit keamanan. Redaksi secret wajib; agregasi/log retention memerlukan implementasi dan verifikasi tersendiri.
- Redesign tidak mengubah API-service/API-Kurir, routing ongkir, grant, akun/password, key, data atau instalasi. Jangan mengganti framework, membuat dashboard baru, melakukan seed/migration atau aktivasi production untuk menyesuaikan mockup.

#### Urutan kerja dan acceptance

Unified dashboard (2026-09-07): entrypoint web/dashboard memakai UnifiedPortal pada 4317, satu login backend /api/v1/portal/login dan discovery /session. Cookie emisell_portal_session path /api/v1 tetap bound surface di tabel sesi; akun dipilih melalui verifikasi credential, bukan request role. Credential cocok dua akun ditolak. Logout mencabut sesi, view/komponen remount saat akun berubah. Legacy API Admin/Developer dan ACL organisasi dipertahankan. Compose hanya dashboard + store, domain DASHBOARD_DOMAIN dan STORE_DOMAIN; alias env legacy masih diterima tanpa mixing. Local HTTP tests: Admin dan Developer login/session 200, akses silang /session 401; logout dilakukan pada sesi pengujian. Tests Go, 48 frontend tests, typecheck/lint/build lulus. 94 operasi docs. Belum commit/push/deploy; server 192.168.0.50 belum diubah. Paket worker/engine production tetap belum termasuk.

Docker/domain preparation (2026-09-07): public origins Admin/Developer/Store dari env tervalidasi HTTPS/domain distinct, cookie Secure host-only, exact Host/Origin tanpa mempercayai forwarded headers. Production control-plane memakai database eksplisit dan API container; RPC loopback, legacy merchant/simulator HTTP tidak diekspos. Dockerfile Go dan Node, Compose PostgreSQL/3 portal/Caddy, volume privat, migration eksplisit dan bootstrap-admin satu akun tanpa fixture. CLI production menolak init-local/init-portals reference. Konfigurasi pilot/remote lokal ditolak. Empat image Docker berhasil dibangun; Compose/Caddy validation, tes config/http/CLI dan 48 tes frontend lulus. Worker/NATS/engine production belum termasuk; tidak deploy ke domain nyata, tidak menjalankan migration/provision pada DB pengguna. Lihat docs/docker-deployment.md; jangan mengklaim production end-to-end siap.

Increment Login/Staf (2026-09-07): Login Admin mengikuti komposisi dua kolom mockup, kartu pengantar, form responsif, dan kontrol tampil/sembunyikan kata sandi. Alur login/session tetap existing; tidak menambah reset password palsu. Menu staff menampilkan direktori akun Admin asli dengan pencarian, filter peran/status, cursor pagination dan refresh manual. GET /api/v1/admin/staff hanya administrator, query surface admin eksplisit, tanpa password/hash/token. Status enabled bukan online; tidak mengarang nama, last active, owner, atau viewer. Undangan dan perubahan role/status belum diimplementasikan. OpenAPI/dashboard docs kini 92 operasi; tes otorisasi identity, 47 tes frontend, typecheck/lint dan build lulus. Backend lokal direstart. Visual QA login/staf lintas viewport belum dijalankan pada increment ini. Tetap lokal, tidak deploy.

Increment Aktivitas Review: route Admin activity membaca history dari GET submissions/{id} existing, satu pengajuan dipilih per permintaan, dengan filter action/aktor serta detail audit. Tidak merekayasa aktivitas dari timestamp status, tidak polling, tidak menggabungkan audit seluruh layanan atau mengekspos token. Label eksplisit bahwa audit login/key/publikasi belum tercakup. API dan permission tidak diubah. Typecheck/lint, 47 tes, build lulus. Login/Staf dan konsolidasi audit platform tetap belum selesai.

Increment Developer Admin: GET /api/v1/admin/developers (50 rows, cursor afterId) dan /developers/{id} memakai session/origin middleware existing serta service guard administrator-only. DTO hanya id/name/memberCount; tanpa credential, email atau mutation. Repository query parameterized pada tabel organisasi existing; tidak ada migration. UI read-only memuat/search daftar dan detail; status verifikasi/jumlah aplikasi belum tersedia. Kontrak portal dan generated API docs diperbarui. Unit test memastikan developer/reviewer/operator/anonymous tidak mencapai repository. Backend lokal dimuat ulang, daftar dua organisasi berhasil dibaca dengan sesi Admin existing.

Increment Aplikasi Admin (7 September): route apps kini menampilkan inventaris terbatas dari pengajuan dan rilis katalog yang termuat, dideduplikasi berdasarkan appId. Publikasi dan review terpisah; aplikasi dianggap memiliki publikasi bila ada versi published dalam daftar termuat. Detail lokal dan tautan pengajuan/publikasi tersedia. Tidak mengklaim inventaris lengkap UI/draft atau menampilkan instalasi/nama organisasi yang tidak tersedia. Backend inventaris lengkap masih gap sebelum mockup Aplikasi dapat dianggap selesai penuh. Developer route apps existing tidak berubah; 47 tes/typecheck/build lulus.

Increment Rilis UI (7 September): daftar Admin memakai kartu jumlah rilis termuat, tabel enam baris per halaman, pencarian nama/versi, filter status dan mode, serta load-more server existing. Label signed tetap Ditandatangani (bukan Terverifikasi), suspended tetap Ditangguhkan. Timestamp/developer ilustratif tidak ditambahkan; revisi aktual ditampilkan. Detail dan mutation review/signing tetap existing, Developer tetap memakai daftar/form sebelumnya. QA fixture desktop/mobile, pagination dan empty filter lulus; typecheck, lint, 47 tes dan build lulus. Tidak ada signing/publikasi/assignment pengguna yang dilakukan untuk pengujian.

Increment Dokumentasi API (7 September): tab kelompok, panel daftar endpoint, detail Request/Response, blok kode gelap dan shortcut sumber unduhan mengikuti arah mockup Admin. Panduan existing dipertahankan dalam disclosure; seluruh 89 operasi/22 kontrak tetap generated, tanpa eksekusi request atau endpoint baru. QA terisolasi desktop/mobile memeriksa overflow, pencarian kosong dan tab response; 47 tes, typecheck, lint dan build lulus. Contoh JavaScript/cURL pada raster tidak disalin karena kontrak existing menyajikan struktur JSON/skema, bukan code generator HTTP. Kesamaan pixel penuh tidak diklaim.

Increment Katalog Scope (7 September): Admin read-only mengikuti layout mockup dengan tiga kartu jumlah dari katalog/verifikasi aktual, tabs Active/Plan/Unknown, filter resource/akses, dan pagination lokal 6 baris. Editor deklarasi dan portal Developer tetap memakai alur sebelumnya. Detail blocker, mapping endpoint, pemeriksaan ulang dan status fail-closed dipertahankan; tidak membuat scope aktif atau mengubah grant. Build, typecheck dan 47 tes lulus. QA visual memakai fixture terisolasi, bukan bukti health backend. Redesign halaman lain tetap bertahap.

Increment API Keys: halaman Admin kini mengikuti mockup dengan banner keamanan, tabel masked secret + ID publik, pencarian nama/ID, filter Semua/Aktif/Dicabut, form buat yang dibuka dari aksi utama, kartu penyimpanan/rotasi, dan tautan dokumentasi. Generate/revoke, administrator-only, idempotency dan secret sekali tampil tetap memakai implementasi existing. Kolom terakhir digunakan tidak ditampilkan karena DTO belum menyediakannya. QA terisolasi desktop/mobile memeriksa filter, empty state dan buka/batal form tanpa mutasi key pengguna; 47 tes dan build lulus. Ini bukan klaim kesamaan pixel seluruh dashboard atau uji mutasi backend.

QA visual berikutnya: `web/dashboard/scripts/admin-visual-qa.cjs` memakai browser fresh dengan seluruh API diintersep fixture (tanpa cookie pengguna/mutasi), screenshot desktop 1536px/mobile 390px, uji overflow/filter/empty state/drawer. Logo tidak lagi menyusut dan baris tabel dipadatkan setelah inspeksi screenshot. Evidence lokal `/tmp/emisell-admin-visual-qa`; bukan bukti E2E backend atau pixel-identical seluruh mockup.

Increment berikutnya: Pengajuan Review Admin memakai tabel, kartu hitungan dari daftar termuat, tabs status backend, search, sorting dan pagination lokal 6 baris. Detail/decision existing tidak berubah. Status “Sedang diperiksa”, nama developer, dan jenis pengajuan ilustratif tidak direkayasa bila DTO belum menyediakan. Filter antrean/empty state diperiksa pada browser existing. Kesamaan pixel desktop/mobile belum diklaim.

Status implementasi increment pertama: Ringkasan Admin (`?view=overview`) dan shell berkelas `admin-redesign` tersedia pada frontend existing. Data review memakai API existing; agregat instalasi/developer dan health belum tersedia sehingga ditampilkan unavailable, bukan angka/grafik mockup. Menu existing tetap dipertahankan. Halaman lain, Login dan Staf belum direimplementasikan sesuai mockup. Build dan 47 tes lulus; render/navigasi desktop diperiksa, perbandingan screenshot 1:1 dan QA mobile belum selesai. Jangan menyatakan keseluruhan redesign selesai.

1. Inventaris route, permission, komponen dan API existing; catat gap antara mockup dan fitur nyata.
2. Implementasikan token/shared shell dan Login, lalu halaman tabel secara incremental pada frontend Admin existing.
3. Hubungkan setiap halaman ke sumber data dan action terotorisasi existing. Gap fitur dilaporkan sebagai pekerjaan terpisah, bukan diisi data hardcoded.
4. Verifikasi desktop/mobile, teks panjang, keyboard, error/empty/loading, deep link, pagination/filter serta regression login/review/key/scope. Tidak menjalankan mutation sensitif pada data pengguna hanya untuk QA visual.
5. Jalankan test/lint/build relevan; laporkan error existing terpisah. Dokumentasikan halaman yang benar-benar selesai dan yang masih konsep. Persetujuan mockup bukan bukti implementasi atau kesiapan production.
