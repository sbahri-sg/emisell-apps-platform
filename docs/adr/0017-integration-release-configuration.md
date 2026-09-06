# ADR 0017 — Rilis konfigurasi Aplikasi Integrasi

Status: diterima untuk development lokal, 2026-09-05.

## Konteks

Metadata katalog approved/signed/published tidak memvalidasi kontrak executable. Lifecycle ADR 0016 hanya menerima fixture eligible. Diperlukan langkah developer → validasi → review konfigurasi → signature yang nyata, tanpa mengubah batas tersebut atau mengklaim kode remote telah dipindai.

## Keputusan

1. **Aplikasi Integrasi** adalah nama UI untuk tipe teknis `remote`. Jangan mengganti enum wire/fixture atau bytes paket lama. Tidak ada migrasi nama pada database.
2. Buat trust domain `integration-configuration/v1`, schema `emisell.integration-release/v1`. Manifest mengikat snapshot metadata approved dari port `CatalogSource` review, ID submission, konfigurasi endpoint/callback/health, versi, capability, scopes, serta checksum sumber. Paket kecil ini disimpan di `platform_app.integration_releases`, tidak masuk registry executable atau katalog publik.
3. Module app memiliki lifecycle release/configuration. Review tetap memasok immutable approved metadata melalui port existing (tanpa app mengakses tabel review atau cyclic import). Signer berada di module review, storage adapter di app repository, transport tipis di HTTP; wiring hanya bootstrap. Tidak menambah framework/dependency.
4. Endpoint integrasi harus persis sama dengan metadata approved; callback dan health satu origin. Policy v1 membatasi HTTPS, DNS hostname ASCII, port 443, tanpa query/fragment/userinfo, menolak alamat literal dan suffix lokal/invalid. **Tidak ada outbound request**. Pemeriksaan ini bukan jaminan tidak ada SSRF ketika runtime nanti dibuat; resolved IP, DNS rebinding, private IP, redirects, TLS, timeout dan egress harus diverifikasi runtime sebelum koneksi.
5. Required resource scope yang belum grantable memblokir pengajuan. Optional Plan boleh tetap sebagai deklarasi, selalu ditampilkan sebagai blocker akses dan tidak diberi token/grant. Payment/shipping dot-scopes adalah reference contract terpisah. Nama scope yang menyerupai Shopify bukan bukti kompatibilitas implementasi.
6. Review konfigurasi terpisah: submitted → approved/rejected oleh reviewer/administrator; approved → signed oleh administrator; approved/signed → suspended oleh administrator. Terminal rejected/suspended tidak diaktifkan ulang. Satu versi satu snapshot; perbaikan membutuhkan versi baru dan review metadata baru.
7. Mutasi memakai key actor-scoped + hash operasi/payload, advisory lock request, row lock release, optimistic revision dan audit dalam satu transaksi. Retry membaca state terkini, bukan receipt berisi snapshot status usang. Ownership dan role tetap diperiksa saat retry. Gagal signing tidak menghasilkan audit/status baru.
8. Checksum SHA-256 dari canonical JSON Go struct; signature Ed25519 atas `integration-configuration/v1` + newline + canonical JSON. Key ID `integration-<sha256 public key>`. Private key acak khusus, disimpan privat `.local/integration-signing.json` pada development saja. Tidak menggunakan key katalog maupun public test key fixture. Signed bytes tidak boleh berubah. Verifikasi hanya terhadap trusted public key melalui jalur pengelola tepercaya, bukan key yang diklaim aplikasi.
9. Detail Admin/Developer menampilkan validation, signature, audit, dan blocker. Selalu `installable:false`: belum ada general remote runtime, app-client OAuth, ownership proof, network health/contract conformance, maupun scan app-code. Signing **konfigurasi**, bukan sertifikasi kode atau pemberian izin. No public listing, installation, routing, credential, webhook atau token dibuat oleh pipeline ini.

## Konsekuensi dan tahap lanjutan

- Developer dapat mempersiapkan konfigurasi dan menerima hasil review persisten sekarang. Tidak ada data demo hasil review di database live.
- Metadata approved, konfigurasi approved, signature verified, runtime ready, consent dan installed adalah fakta berbeda. Tidak boleh digabung menjadi satu flag.
- Tidak memperkenalkan adapter S3 kosong untuk JSON konfigurasi. Binary artifact tetap memerlukan S3/MinIO, checksum, security scan, retention dan release provenance sebelum executable pipeline umum tersedia.
- Registrasi OAuth/app-client, proof of endpoint ownership dan conformance harness dengan server test terkontrol menjadi tahap berikutnya. Pengaktifan runtime memerlukan policy/manifest baru, install eligibility yang fail-closed dan consent yang mengikat digest release tepat. Tidak mengubah signed konfigurasi v1 menjadi executable secara diam-diam.
- Production memerlukan KMS/secret manager, trust registry versioned, rotasi/revokasi signing key dan revocation policy. Key development tidak dianggap production-ready.

## Migration / rollback

Migration 0013 membuat tabel release/request/audit dan trigger immutable/state-transition. Tidak mengubah migration 0001–0012, akun, key Core, instalasi, scopes, catalog atau events. Backup privat sebelum apply; jalankan migration eksplisit lalu provision key integrasi eksplisit dan restart backend. Frontend baru setelah backend baru tersedia.

Rollback aplikasi dapat menyembunyikan menu/rute baru tanpa drop tabel atau menghapus signature/audit; tidak memerlukan memundurkan migration 0012. Pertahankan binary yang memahami grant lifecycle ADR 0016. Bila key rusak/hilang, sign fail-closed dan suspend/reject tetap dapat dilakukan; jangan regenerate otomatis untuk menutupi kegagalan verifikasi.

## Bukti verifikasi yang diwajibkan

Unit test canonical/signature/tampering/trust-domain, URL policy dan scope Plan. Integration test PostgreSQL untuk dua capability, ownership/origin/role, duplicate submission/key conflict, concurrent review, key outage, audit atomicity, immutable snapshot/package, suspension/replay, rejection terminal dan penolakan Core install intent. UI typecheck/test/lint/build serta dokumentasi generated drift check. Pengujian hanya memakai database `emisell_local_test`.
