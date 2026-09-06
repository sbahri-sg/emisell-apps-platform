# Rilis konfigurasi Aplikasi Integrasi

Panduan development lokal ADR 0017. Nama UI **Aplikasi Integrasi** berarti aplikasi berjalan di server developer. Kontrak teknis tetap `remote`.

Kebijakan terbaru ADR 0022: authoring/distribusi baru hanya `shipping/v1`. Payment gateway checkout internal Emisell, bukan aplikasi umum. Konfigurasi payment historis tetap dapat dibaca/diverifikasi, tetapi check `public_distribution` gagal; submit/approve/sign, app-client dan Testing tidak dapat melanjutkan. Reject/suspend/revoke tetap tersedia. Ini tidak mencabut installation fixture Emisell Pay.

## Menjalankan

1. Backup database lokal ke lokasi privat, lalu `go run ./cmd/cli migrate` (migration 0013 additive).
2. Provision satu kali `go run ./cmd/cli init-integration-signing`. Command idempotent, tidak mengganti key atau menampilkan secret. Simpan `.local/integration-signing.json` privat dan di luar Git. Restart backend untuk membaca key.
3. Developer: `http://localhost:4319/?view=integration-releases`. Admin: `http://localhost:4317/?view=integration-releases`. App Store tetap port 4318, tidak berubah menjadi dashboard merchant.

## Alur penggunaan

1. Buat/simpan draft Aplikasi Integrasi pada **Aplikasi saya**. Isi versi SemVer tiga angka, deskripsi, shipping/v1, serta endpoint HTTPS yang dikelola developer. URL contoh bukan bukti server aktif.
2. Ajukan review metadata; Admin reviewer/administrator menyetujui metadata. Ini belum menyetujui konfigurasi ataupun memublikasikan listing.
3. Developer buka **Rilis integrasi**, pilih versi metadata approved. Isi callback OAuth dan health endpoint. Profil saat ini `emisell.capability-http/v1`; endpoint utama berasal dari snapshot dan tidak dapat diganti di form konfigurasi.
4. **Validasi konfigurasi**: tidak menyimpan atau memanggil URL. Semua URL harus hostname DNS HTTPS, port 443, satu origin, tanpa token/userinfo/query/fragment. Required resource scope Plan ditolak; optional Plan boleh dideklarasikan tetapi tidak diberikan akses. Koreksi draft/versi dan ulang review metadata bila endpoint atau scope wajib harus berubah.
5. **Ajukan review konfigurasi** menyimpan immutable snapshot. Satu app/version dan satu submission hanya boleh memiliki satu konfigurasi. Setelah diajukan, perbaikan memakai versi baru, bukan edit snapshot.
6. Admin/reviewer membuka detail, meninjau manifest/endpoint/scopes serta batas pemeriksaan, lalu menyetujui/menolak dengan alasan. Admin administrator menandatangani konfigurasi approved dalam tindakan terpisah. Signature tidak memublikasikan katalog.
7. Detail menampilkan checksum, signing key ID, hasil verifikasi, blocker install dan audit. Unduh snapshot/paket bertanda tangan; paket memiliki endpoint privat, jangan meletakkannya pada katalog publik. Public key integrasi dapat dibagikan melalui jalur pengelola tepercaya; private key tidak boleh dibagikan.
8. Administrator dapat menangguhkan konfigurasi approved/signed dengan alasan. Rejected/suspended tidak dapat diaktifkan ulang; versi baru diperlukan. Key hilang tidak menghalangi penolakan/suspension.

## API portal

| Operasi | Developer | Admin |
|---|---|---|
| GET `/integration-releases` | Milik organisasi | Semua, maksimal 200 terbaru |
| GET `/integration-releases/{id}` | Milik organisasi, validation + audit | Semua, validation + audit |
| POST `/integration-releases/validate` | Approved metadata sendiri, tanpa write/network | Tidak tersedia |
| POST `/integration-releases` | Ajukan konfigurasi | Tidak tersedia |
| POST `/integration-releases/{id}/status` | Tidak tersedia | Review/sign/suspend sesuai role |

Prefix `/api/v1/developer` atau `/api/v1/admin`. Cookie portal sesuai surface + Origin/Referer exact localhost 4319/4317 diperlukan, juga pada GET. Ini bukan endpoint untuk key backend Core atau token aplikasi. Submit/status wajib `Idempotency-Key`; status wajib revision dan alasan. Reuse key dengan payload berubah menghasilkan conflict. Retry identik mengembalikan state release terkini, termasuk suspended; tidak menandatangani ulang. Rincian schema tersedia pada menu Dokumentasi API Admin, bersumber dari `api/openapi/integration-releases.v1.json`.

## Apa yang belum diberikan

Profil deklaratif `emisell.capability-http/v1` merujuk payload JSON `pkg/appapi.Invocation` / `pkg/appapi.Response` untuk capability reference. Profil ini belum memasang adapter runtime ke URL dalam manifest; implementasi fixture lokal tetap terpisah. Validasi nama profil bukan conformance test terhadap server aplikasi.

- `installable` selalu false. Registry fixture / Core Prepare/Consume tidak membaca integration_releases. Tidak ada tenant, installation, app-client, token, grant, routing ataupun webhook subscription dibuat oleh menu ini.
- Signature menjamin integritas konfigurasi terhadap trusted key, **bukan** keamanan kode remote, ketersediaan server, kepemilikan domain atau grant scope.
- Pipeline konfigurasi ini tidak memanggil DNS/TLS/health/redirect/egress atau conformance end-to-end. Registrasi app-client dan bukti kendali origin sudah tersedia sebagai langkah terpisah pada **App clients**; baca [panduan app-client](app-clients.md). Bukti tersebut tidak menguji health endpoint bisnis/callback atau membuat konfigurasi installable. OAuth token exchange dan runtime umum belum tersedia.
- Artefak binary, security scan app-code, general WASM/UI extension delivery dan KMS/rotasi production belum termasuk milestone ini. Tidak menambahkan dependency/framework/infrastruktur baru untuk menyimulasikan kelengkapan tersebut.

Verifikasi Go dapat memakai `pkg/integrationmanifest.Verify(package, trustedPublicKey)`; key didapat dari pengelola, bukan field input developer. Konfigurasi v1 tidak boleh dipromosikan menjadi executable hanya karena verifikasi signature berhasil.
