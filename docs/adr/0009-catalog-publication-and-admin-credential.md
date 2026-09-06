# ADR 0009 — Katalog bertanda tangan, tooling metadata, akun utama Admin

Status: diterapkan lokal, 5 September 2026. Melanjutkan ADR 0008 tanpa mengubah capability atau executable registry.

## Keputusan dan batas keamanan

Pengguna meminta kelanjutan Admin/Developer dan akun utama Admin `dev@emisell.com`. Review milestone sebelumnya hanya metadata, belum verifikasi artifact atau runtime app umum. Karena itu publikasi pertama adalah **katalog metadata non-executable**, bukan jalan pintas mengaktifkan provider arbitrary.

Alur lengkap: draft developer → immutable review snapshot → approved → Administrator memvalidasi dan menandatangani metadata → aksi publish dengan alasan → listing publik. Administrator dapat suspend/republish; isi signed package tidak berubah. Publisher pada signature adalah organisasi developer pada snapshot, signer adalah platform lokal. Ini attestation metadata, bukan self-signing developer atau sertifikat keamanan aplikasi.

`emisell.catalog/v1` / `catalog-metadata/v1` mengikat app ID, developer ID, versi, nama, deskripsi, capability, scopes yang diurutkan, digest sumber review, `remote`, `free`, dan `installable:false`. Tidak mempunyai endpoint, code, webhook, secret atau extension payload. Validasi membatasi ukuran, format/version/ID, control characters, capability/scopes, pricing dan installability. Frontend merender teks secara escaped, tidak memakai HTML dari developer. Tidak ada fetch ke endpoint draft.

SHA-256 dihitung dari JSON canonical berurutan field struct Go (bukan klaim RFC 8785); Ed25519 menandatangani byte canonical. Key ID SHA-256 public key. CLI menerima whitespace/urutan field/escaping JSON yang ekuivalen, kemudian canonicalize; menolak unknown/duplicate field, nesting berlebih dan trailing data. Ini penting untuk browser export dengan ampersand atau karakter yang di-escape berbeda. Verifikasi signed package mengikat nilai manifest canonical, digest, key ID dan signature terhadap key yang dipercaya caller. Public key yang disertakan penyerang tidak boleh menjadi sumber trust. Signature ini DILARANG diterima oleh `emisell.app/v1` atau runtime.

## Boundary dan persistence

- `review.Service.CatalogCandidate` memeriksa akses dan approved state, mengembalikan snapshot melalui port milik app service. App repository tidak membaca tabel review/developer.
- App service memakai signer sempit, adapter `review.CatalogSigner` memakai key khusus; deterministic fixture signing key tidak disentuh.
- Migration `0008_catalog_publication.sql`: `platform_app.catalog_releases`, `catalog_requests`, `catalog_audit`. Trigger melindungi signed package dan identity fields. Revision/state bisa berubah lewat use case. Unique published app memastikan hanya satu versi terlihat; admin harus suspend versi lama dahulu.
- Metadata canonical kecil adalah dokumen relasional JSON, bukan artifact upload. Tidak ada penyimpanan binary atau object-storage scaffold. Target S3/MinIO tetap wajib untuk artifact runtime selanjutnya.
- Tidak ada cross-module writes, event broker baru, tenant installation mutation, OAuth/grant baru, atau biaya install. Audit ditulis atomik dengan status dan idempotency record. Event publikasi lintas consumer belum dibutuhkan; jika ditambahkan wajib outbox.
- Public list pagination 20, urutan nama+ID, literal substring search, capability filter. Query parameter page 1–500; search maksimal 120 byte. Count/page memakai snapshot database yang sama. Portal maksimal 200 rilis terbaru; riwayat 200 tindakan awal. Pagination lanjut portal/audit masih batas yang perlu diperluas.
- Public DTO hanya app/release/developer ID dan metadata listing; tidak ada endpoint/feedback/submission/actor/tenant/secret. Public request tidak memakai cookie. Signature diperiksa pada publish dan public reads. Key hilang/invalid membuat reads yang memerlukan verifikasi dan publish gagal, bukan diam-diam mempercayai database.
- `signed → published → suspended → published`; semua mutation memerlukan Administrator, idempotency key, dan publication expected revision (sign menggunakan immutable submission). Retry tidak menduplikasi audit atau menyalakan kembali listing yang telah suspended.

## Akun Admin dan recovery

`update-admin <email>` adalah maintenance lokal, bukan endpoint browser. ID target tetap `portal-local-admin`, surface admin, role administrator. Password masuk stdin, di-hash PBKDF2 melalui implementation existing, tidak muncul di argv/source/log/audit. Semua sesi target dicabut dalam transaksi yang sama. Login mengambil account row lock dan mencocokkan hash yang sudah diverifikasi agar old-password login yang in-flight tidak lolos setelah rotasi. Principal lain dipertahankan.

File `.local/portals.json` tetap 0600, direktori 0700. Commit database mendahului penulisan file atomik. Jika file gagal ditulis setelah commit, command memberi error recovery: ulangi input identik untuk rekonsiliasi; `init-portals` jangan dijalankan sebelum file pulih. Retry update yang telah committed tidak mencabut sesi baru lagi. Maintenance CLI dijalankan serial; file `.next` yang tertinggal harus diperiksa pengelola, bukan dihapus/ditimpa otomatis. Hash DB dan password plaintext development-only di file privat mengikuti mekanisme provisioning lokal existing; production tidak menyimpan password plaintext seperti ini.

Key katalog dibuat eksplisit `init-catalog` dengan CSPRNG, disimpan di `.local/catalog-signing.json` (0600), tidak dalam database/browser/source. Startup tidak menciptakan key baru. Re-init mempertahankan key. Recovery memakai backup key yang sama; rotation otomatis akan membuat rilis lama unverifiable dan dilarang. KMS, trust keyring, revocation, serta retensi/rotasi merupakan prasyarat sebelum production. Password Admin dari percakapan perlu diganti sebelum sistem diekspos di luar development lokal.

## Frontend dan migration path

Frontend tetap React/Vinext dan primitive/token yang sudah tersedia. Admin/Developer berbagi komponen tanpa berbagi session. App Store memiliki entry dan proxy read-only sendiri `4318` → `/api/v1/store`. Tidak ada merchant workspace atau UI install. Developer tools mencakup validasi/export metadata serta download paket/public key; belum SDK atau executable manifest.

Rollout additive: backup development DB dan binary API → jalankan migration 0008 eksplisit → init key katalog → restart API dengan signer. Tidak perlu restart worker/Core karena tidak ada event/capability contract baru. Migrasi 0001–0007, data reference, credential Core/remote dan installations tidak berubah.

Rollback aplikasi: hentikan API baru dan kembali ke binary sebelumnya; biarkan schema additive dan credential Admin baru. Rilis katalog tidak akan dilayani binary lama. Jangan rollback DB atau mengembalikan password lama otomatis. Jangan menghapus key karena paket yang sudah ditandatangani bergantung padanya. Perluasan executable pipeline nantinya adalah contract/trust domain baru, bukan toggle `installable:true` pada katalog ini.

## Verifikasi

Tests: signature/key/checksum/publisher tamper, policy/version/scope/unknown/trailing data; primary-admin credential update/retry/session revocation/in-flight login; draft tooling ownership, review-before-sign, Administrator-only signing/publication, idempotency/audit, immutable package trigger, public allowlist/search/pagination validation, concurrent suspend, key-unavailable publishing, dan penolakan install katalog melalui executable registry. Semua dijalankan pada database test loopback terpisah.

Browser QA menggunakan app yang bernama `QA Portal Shipping` dengan deskripsi simulasi. Signing/publish/suspend/re-publish dan ekspor tidak pernah memanggil endpoint, memasang app, atau menghasilkan transaksi. Listing QA dipertahankan sebagai contoh lokal berlabel QA; bukan release production.

## Belum termasuk

Executable manifest self-service, SDK runtime, artifact upload/S3 scanner, third-party runtime/OAuth client self-service, deployment production, Core consent/install, team invitation/role management, billing berbayar, WASM/UI execution, moderation otomatis konten dan pagination portal skala besar. Jangan menyebut Admin/Developer sudah lengkap hanya karena milestone katalog selesai.
