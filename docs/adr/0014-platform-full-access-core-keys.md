# ADR 0014 — Key platform full access untuk backend Emisell

Status: diterima, 2026-09-05. Menggantikan keputusan generate tenant-bound pada ADR 0013 sesuai koreksi pemilik produk; bukan mengubah credential yang sudah terbit.

## Keputusan

- Admin → API Key hanya meminta nama koneksi. Credential first-party Emisell backend → App Platform memiliki akses penuh ke layanan internal Core yang tersedia, tidak terikat tenant, tidak mempunyai expiry otomatis, berlaku sampai dicabut.
- Credential ini bukan sesi portal Admin/Developer, token OAuth app atau grant merchant. Tidak membuka route resource planned, melewati consent atau membuat instalasi aktif. API payment/shipping tetap memerlukan active installation, grant capability dan runtime policy. Install intent tetap consent record dengan `executionAllowed:false`.
- Credential baru terpisah dari service account legacy: table `platform_identity.core_platform_keys`, ID `platformkey_…`, secret `epk_` diikuti 256-bit random base64url. Prefix membedakan format, bukan bukti otorisasi. Autentikasi tetap lookup hash DB dan memeriksa revocation; server tidak menerima flag full-access dari header/body caller.
- Management administrator-only memakai `/api/v1/admin/platform-keys` GET/POST dan `/{id}/revoke` POST, mengikuti session audience, Origin dan no-store. Body generation hanya `{ "name": "Emisell backend" }`; field tenant/scopes/expiry ditolak. Key bearer sendiri tidak boleh mengelola key di Admin REST.
- Hash SHA-256 saja disimpan. Secret hanya respons pertama, disembunyikan UI setelah 5 menit/navigasi; tidak disimpan browser storage. Penerbitan idempotent per administrator + request key + trimmed name, diserialisasi transaksi, audit atomik. Replay memberi metadata terkini tanpa secret. Response hilang: revoke lalu generate request baru. Revoke idempotent/audit sekali; request berikutnya ditolak, request in-flight mungkin sudah dieksekusi.

## Tenant dan kontrak Core

Core wajib memvalidasi sesi staf, membership, izin mengelola apps/data dan CSRF sebelum mengirim tenant/actor. Platform mempercayai assertion dari backend first-party yang sudah diautentikasi, bukan sesi browser independen.

- Payment/shipping memakai `tenantId` yang sudah ada. InstallIntent `Prepare/Get/Decide` menambah field Protobuf `tenant_id` secara additive (masing-masing 5/3/6), wajib untuk key platform.
- Tenant kosong, format salah atau belum terdaftar ditolak. Query/idempotency/resource lookup tetap scoped tenant; key yang sama tidak dapat membaca ID resource A ketika request menyatakan tenant B. Intent tetap terikat tenant + service ID + actor.
- Key tenant legacy boleh menghilangkan field intent tenant; jika diisi harus sama dengan binding. Scopes dan expiry lama tetap diberlakukan.
- `ConnectionService.Check` menambah `platform_full_access` field 5. Key platform menghasilkan true tanpa tenant/scopes/expiry. Legacy mempertahankan field sebelumnya. SDK local menerima kedua format dan menyediakan `client.Connection`. Loopback-only, tidak mengikuti redirect/proxy dan menolak Cookie/Origin browser tetap dipertahankan.

## Migration dan rollback

1. Jalankan migration additive `0011_platform_full_access_keys.sql` sebelum binary baru. Jangan mengubah checksum migration lama atau menaikkan hak key lama.
2. Deploy backend + generated SDK + UI. Key platform tidak dibuat otomatis; administrator melakukan generate secara eksplisit.
3. Pindahkan consumer secara terencana: simpan secret server-side, Check identitas, kirim tenant assertion, verifikasi operasi tenant, baru cabut key lama. Pending intent terikat service ID lama dan tidak dipindahkan; mulai consent baru bila beralih credential.
4. `/api/v1/admin/api-keys` tetap tersedia sebagai legacy deprecated untuk list/generate/revoke sesuai kontrak tenant/scopes/1–30 hari sebelumnya. UI baru menampilkan platform keys saja. CLI keys tidak diubah. Jangan menghapus key untuk membersihkan tampilan.
5. Rollback mempertahankan schema/audit. Binary lama tidak bisa memakai key platform; koordinasikan consumer kembali ke credential legacy yang masih valid (tanpa pemulihan credential revoked) sebelum rollback. Jangan drop data.

## Risiko dan verifikasi

Blast radius full-access lintas tenant lebih besar, dan credential tidak habis otomatis. Wajib secret manager server-side, rotasi operasional, audit review dan pencabutan segera bila bocor. HTTPS/internal network identity, quota/rate limiting dan tata kelola production masih gate deployment; localhost saat ini bukan kesiapan production. Jangan membagikan key pada developer aplikasi, frontend atau App Store publik.

Tests wajib: admin/origin/unknown-field denial; hash-only/secret sekali/retry conflict/concurrent issuance/revoke/audit/persistence; satu key memakai payment/shipping dan intent pada dua tenant; tenant kosong/asing, foreign resource dan actor ditolak; installation/grant gate tetap berlaku; legacy tidak memperoleh tenant/scope baru, expiry dan revocation tetap berfungsi. Generated contracts harus lolos Buf breaking check, build, race tests dan UI typecheck/lint/test/build.
