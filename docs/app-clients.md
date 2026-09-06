# App clients — identitas aplikasi dan bukti kendali origin

Milestone lokal ADR 0018. Fitur ini mendaftarkan identitas confidential server aplikasi dari **rilis konfigurasi signed**, membuktikan kendali origin HTTPS, dan menerbitkan secret untuk self-check. Ini **belum** OAuth authorization-code/token exchange, runtime integrasi umum, atau izin membaca data merchant.

ADR 0022 membatasi distribusi aplikasi umum pada shipping/v1. Release payment historis tidak memenuhi gate distribusi: registrasi client, tindakan non-revoke dan self-check tidak diterima, tanpa menghapus/merotasi secret lama. History/read/revoke tetap tersedia. Payment gateway checkout merupakan jalur internal Emisell, bukan client aplikasi umum.

## Persiapan

1. Backup database lokal ke direktori privat (0700, dump 0600), lalu jalankan `go run ./cmd/cli migrate` untuk migration 0014 additive. Jangan menjalankan reset/seed terhadap database live.
2. Pertahankan `.local/integration-signing.json` existing. Backend harus dapat memverifikasi rilis signed; jangan membuat ulang key untuk menutupi kegagalan signature.
3. Restart backend versi baru. Admin tetap `http://localhost:4317/?view=app-clients`, Developer `http://localhost:4319/?view=app-clients`, App Store port 4318 tidak berubah.

## Alur developer

1. Selesaikan metadata review → konfigurasi review → signing melalui **Rilis integrasi**. App-client tidak dibuat dari draft, metadata approved saja, atau listing katalog.
2. Buka **App clients**, pilih rilis signed milik organisasi dan daftar. Registrasi tidak memanggil jaringan dan tidak menerbitkan secret. Satu release hanya memiliki satu client; binding organisasi/app/version/checksum/endpoint/callback tidak dapat diedit.
3. Detail menampilkan **URL bukti** dan JSON yang harus dilayani server developer. Pasang JSON persis pada path `/.well-known/emisell-app-verification/<challengeId>` di origin HTTPS konfigurasi. Balas 200 `application/json`, tanpa redirect, kompresi atau field tambahan. Challenge berlaku 10 menit. Jangan menaruh client secret di file ini.
4. Isi alasan dan pilih **Verifikasi endpoint**. Platform mengambil bukti melalui backend dengan koneksi HTTPS terbatas, bukan dari browser. JSON harus memuat schema `emisell.endpoint-proof/v1`, client ID, release SHA-256 dan challenge yang sesuai. Hasil gagal ditampilkan sebagai `failed`; rincian respons remote tidak dipantulkan.
5. Setelah verified, terbitkan client secret. Simpan langsung di secret manager server developer. Secret hanya tampil sekali; UI menghapusnya setelah 5 menit, detail ditutup/diganti, atau halaman ditinggalkan. Jangan masukkan secret ke frontend, query URL, log, screenshot, atau Git.
6. Uji dari server developer melalui POST `/api/v1/app/client-check` menggunakan HTTP Basic (username client ID, password client secret), `Content-Type: application/json`, body `{}`, tanpa Origin/Cookie/query. Backend lokal berada di loopback `http://127.0.0.1:8087`; jangan mengekspos listener development ini atau mengirim credential melalui HTTP publik. Deployment production kelak wajib HTTPS.
7. Respons sukses hanya mengonfirmasi binding identitas dan callback terdaftar. `oauthEnabled:false`, `installable:false`, `resourceGatewayAllowed:false` tetap wajib. Tidak ada authorize URL/token endpoint production atau app access token yang diberikan oleh operasi ini.

Client ID memakai prefix `eac_`, secret `eacs_`. API key Core `epk_` dan token installation `eat_` adalah credential lain dengan audience lain; tidak dapat dipakai saling menggantikan.

## Status, renewal dan pencabutan

| Kondisi | Makna / tindakan |
|---|---|
| Pending | Bukti belum sah; secret tidak boleh diterbitkan. |
| Verified | Bukti origin sah sampai `verifiedUntil` (24 jam setelah berhasil); bukan install eligibility. |
| `clientReady:true` | Bukti masih berlaku, secret tersedia dan release tetap signed/valid. Bukan grant. |
| Expired | Status tersimpan dapat tetap verified; readiness dan autentikasi tetap menolak bukti expired. |
| Revoked | Terminal, secret dan bukti dicabut; tidak dapat diaktifkan ulang. |
| Release suspended / signature tidak tersedia | Readiness dan autentikasi gagal tertutup; revoke masih tersedia. |

- **Perbarui challenge** menghasilkan nonce/path baru, langsung membatalkan proof dan secret lama. Pasang ulang JSON, verifikasi, lalu terbitkan secret baru. Challenge baru tidak melewati cooldown 1 menit antar upaya verifikasi.
- **Rotasi secret** memerlukan proof belum expired. Secret lama tidak valid lagi setelah transaksi rotasi berhasil. Bila respons hilang, ulang key yang sama hanya mengembalikan metadata; setelah memastikan state terbaru, lakukan rotasi baru dengan key baru untuk memperoleh pengganti. Tidak ada endpoint pembacaan secret.
- Developer dapat mencabut client sendiri; hanya administrator yang dapat revoke lintas organisasi. Reviewer/operator tidak dapat memutasi. Admin tidak dapat menerbitkan secret atau melakukan probe atas nama developer.
- Satu client terikat release secara immutable pada versi awal ini. Client revoked memerlukan release/versi baru. Identitas OAuth stabil lintas versi kelak memerlukan desain/migration tersendiri; jangan memindahkan grant/token atau mengganti redirect URI existing secara diam-diam.
- Bukti yang diperoleh sebelum challenge diperbarui, client direvoke, atau release disuspend tidak boleh mengaktifkan kembali client. Penyelesaian probe memeriksa revision dan release lagi.

## API dan retry

| Operasi | Developer | Admin |
|---|---|---|
| GET `/app-clients` | Organisasi sendiri | Semua |
| GET `/app-clients/{id}` | Detail + audit sendiri | Detail + audit semua |
| POST `/app-clients` | Register dari `releaseId` signed | Tidak tersedia |
| POST `/app-clients/{id}/actions` | `challenge`, `verify`, `rotate_secret`, `revoke` | Administrator: `revoke` saja |

Prefix `/api/v1/developer` atau `/api/v1/admin`; wajib session surface dan Origin/Referer exact sesuai portal. List/history dibatasi 200 entri. Mutasi wajib `Idempotency-Key` 8–128 karakter alfanumerik/underscore/hyphen. Action wajib `revision` terkini dan alasan nonkosong tanpa control character, maksimum 2000 byte. Organization/actor diturunkan dari session, bukan input.

Retry identik membaca metadata terkini, tidak mengulang probe, audit, atau mengungkap secret. Key dengan payload berbeda conflict. Gate release tetap diperiksa: retry non-revoke dapat ditolak bila release sudah tidak sah. Network proof gagal menghasilkan respons 200 dengan `lastResult:failed`, bukan status verified; conflict/cooldown/expired mengembalikan 409. Jika process terputus setelah reservasi probe, hasil dapat tertinggal `checking` (UI API menjadi `interrupted` setelah 1 menit). Refresh lalu ulang dengan key baru sesudah cooldown; key lama tidak menjalankan probe lagi.

Kontrak lengkap delapan operasi ada di `api/openapi/app-clients.v1.json` dan Dokumentasi API Admin. Self-check client berada pada grup **Akses aplikasi**, berbeda dari self-check installation token.

## Batas verifikasi jaringan

- URL diturunkan dari origin release signed, tidak menerima input URL bebas. TLS valid, hostname DNS ASCII, port 443, IPv4 publik saja. DNS A diperiksa seluruhnya lalu koneksi dipin ke satu IP; tidak melakukan lookup ulang saat dial.
- Menolak IP literal/private/link-local/metadata/reserved, IPv6-only dan mixed DNS answer yang mengandung alamat terlarang. Tidak mengikuti redirect, proxy environment, cookie, atau kompresi. Tidak ada bypass localhost untuk server live.
- Maksimal 4 probe bersamaan per process, cooldown 1 menit per client, timeout keseluruhan 5 detik, TLS/dial/header timeout 2 detik, header maksimum 16 KiB, body maksimum 4096 byte. JSON duplicate/unknown fields atau data trailing ditolak.
- Expected nonce, secret, dan tenant tidak dikirim pada request outbound. Server harus sudah menyediakan bukti secara independen; URL hanya mengandung ID challenge.
- Ini membuktikan kendali origin pada saat pemeriksaan, **bukan** ketersediaan endpoint bisnis/callback, kesesuaian kontrak payment/shipping, keamanan kode developer atau consent merchant. Jangan menampilkan status ini sebagai sertifikasi aplikasi aman.

## Verifikasi dan rollback

Test mutasi hanya pada database `emisell_local_test`. Jalankan `make verify` dengan `EMISELL_TEST_DATABASE_URL` dan `EMISELL_NATS_SERVER` lokal yang benar; ini meliputi race tests, format/lint/breaking Protobuf, build dan vet. Ketiga frontend wajib typecheck/test/lint/build; Admin juga memeriksa generated API docs drift. Test SSRF/TLS memakai server dan trust root terkontrol yang tidak dapat dikonfigurasi dari input aplikasi.

Rollback aplikasi dapat menyembunyikan menu/rute app-client, tanpa drop tabel/request/audit atau mengubah installation/grant existing. Pertahankan backend yang memahami lifecycle ADR 0016. Migration 0014 tidak melakukan backfill, auto-client, akun, key Core, installation atau grant mutation. Production masih memerlukan OAuth flow/consent yang lengkap, runtime conformance, secret manager, egress controls dan kebijakan operasi yang ditinjau terpisah.
