# Demo embedded lokal

## Tujuan dan batas

Demo yang diminta pengguna membuktikan protokol browser dan backend identitas dengan **data sintetis saja**. Ini bukan instalasi Core, bukan login seller, bukan aplikasi RajaOngkir, dan bukan pengganti Settings → Apps. Tidak membaca database, `.env`, credential, order, atau API-Kurir. Tidak ada request keluar dari loopback.

Jalankan dari root repository: `go run ./cmd/embedded-demo`. Buka `http://127.0.0.1:4320/`. Aplikasi iframe memakai `http://127.0.0.1:4321/`. Kedua listener hanya bind IPv4 loopback dan menolak Host lain. Port portal 4317–4319 tidak berubah. Port yang sedang terpakai menyebabkan startup gagal, bukan menghentikan proses lain.

## Cara mencoba

UI iframe memakai [Emisell UI Kit lokal 0.1.0](embedded-ui-kit.md), dengan kartu, tombol, badge dan status yang reusable. Host sintetis tetap terpisah. Pada pilot seller `/seller`, identitas berasal dari sesi Core sesuai ADR 0029; batas sintetis di bawah berlaku untuk host 4320.

1. Klik **Open demo**. Iframe menampilkan `demo-store`, `demo-staff`, dan status **Terhubung**.
2. Sesi diperbarui otomatis saat tersisa 15 detik (pemeriksaan setiap 5 detik). **Perbarui sesi** tetap tersedia untuk retry manual; seller tidak perlu menekannya pada alur normal. Saat tab kembali terlihat atau koneksi online kembali, verifikasi baru dilakukan.
3. Klik **Cabut akses uji**. Backend menolak penerbitan baru dan penggunaan token lama meskipun belum expired. Iframe memeriksa akses setiap 5 detik lalu menghapus identitas yang ditampilkan.

### Pemulihan sesi

Token ditolak dapat disebabkan expiry/restart Core; demo mencoba satu sesi baru melalui authorization existing sebelum menyimpulkan kegagalan. Identitas dikosongkan saat gagal, tanpa fallback data lama. HTTP 401/403 dari introspeksi diperlakukan sebagai penolakan akses; timeout, 429, 5xx atau respons rusak menjadi unavailable. Bridge error tidak mengungkap alasan spesifik, sehingga UI tidak menyebut akses dicabut tanpa bukti tersebut.

Retry otomatis maksimal tiga kali dengan jeda minimum 2/5/15 detik, dievaluasi pada tick 5 detik. Setelah habis, retry manual atau event online/visibility dapat memulai ulang; seluruh pemeriksaan permission tetap berlaku. Request verifikasi dibatasi 5 detik dan response setelah pagehide tidak mengisi identitas kembali. Uji deterministik: `node --test cmd/embedded-demo/session.test.mjs`; boundary backend: `go test -race ./cmd/embedded-demo`. Tidak mengaktifkan runtime embedded umum atau memperluas grant.
4. Pencabutan terminal sepanjang proses demo. Jalankan ulang proses untuk mendapatkan key dan sesi sintetis baru. Reload halaman tidak menghidupkan akses lama.

## Kontrak endpoint demo — bukan API production

| Endpoint | Boundary | Perilaku |
|---|---|---|
| GET `/demo/config` | Parent 4320, same-origin tanpa CORS | Memberi anti-CSRF nonce khusus demo dan ID instalasi sintetis. Bukan credential Platform. |
| POST `/demo/session` | Exact Origin parent, `X-Demo-CSRF`, JSON `{installationId}` | Memakai `LaunchHandler`, signed launch dan identity service existing. Return launch, identityToken, expiresIn=60; identitas tidak berasal dari browser. |
| GET `/demo/identity` | App 4321, `Authorization: Bearer …` | Backend app hanya memiliki public verification key; memeriksa issuer/audience/identitas/signature/expiry dan current demo access. Mengembalikan identitas sintetis tanpa token. |
| POST `/demo/revoke` | Exact Origin parent, `X-Demo-CSRF`, body kosong | Cabut akses demo secara idempotent; tidak memanggil Uninstall merchant. |

Token/key/nonce tidak dicatat ke log. Token tidak diletakkan dalam URL, cookie atau persistent browser storage. Identitas berlaku 60 detik; current-access check wajib terpisah. Key identitas dan signing launch berbeda, acak dan hanya berada dalam memori. Restart membatalkan token proses sebelumnya. State read lock menahan issuance/verification sampai selesai; revoke menunggu operasi tersebut, lalu semua operasi berikutnya ditolak.

Parent memakai frame-ancestors none, app hanya menerima parent demo, sandbox iframe dan exact postMessage origin/window. Bridge memakai sumber `pkg/embedded/bridge.mjs` yang sama, bukan implementasi protokol salinan. CSRF demo dan identitas sintetis **DILARANG** dipindahkan ke listener production.

## Yang belum diselesaikan

- Binding persisten installation managed release → integration app-client → reviewed launch belum ada. Nama/app ID serupa tidak cukup untuk membuat binding atau memperluas consent lama.
- Session/membership/izin staf Core belum tersambung ke issuer demo. Callback sintetis hanya dalam executable demo.
- Tombol Open app Dashboard utama, review/domain proof URL aplikasi nyata, dan token exchange resource belum diaktifkan.
- Demo memeriksa state lokal dalam proses yang sama; ini bukan bukti mekanisme revocation server-to-server app production atau uji uninstall persistent.

Tahap integrasi harus menyelesaikan binding yang direview, current grant, real Core authentication, audit/rate limit, serta migration/consent policy sebelum membuka akses seller. Jalur tarif Core → API-Kurir existing tidak berubah.

## Verifikasi

`go test -race ./cmd/embedded-demo ./pkg/embedded ./internal/oauth/embedded`, `go vet` pada package yang sama, build executable, dan `node --test pkg/embedded/bridge.test.mjs` lulus. Test mencakup renewal, token invalid/expired, identity override, CSRF, foreign Origin/Host, revoke idempotent dan penolakan token lama/setelah revoke. Pengujian browser desktop dilakukan pada demo lokal, bukan akun merchant.
