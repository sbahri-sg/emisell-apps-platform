# Backend identitas

## Adapter Core reviewed UI lokal

Jika rilis UI/app-client sudah tersedia di Core lokal, gunakan adapter introspeksi
existing tanpa menyalin key issuer atau cookie seller:

```sh
export EMISELL_LOCAL_CORE_ORIGIN=http://127.0.0.1:8000
export EMISELL_LOCAL_APP_ID=app_REPLACE_WITH_REAL_ID
export EMISELL_LOCAL_CLIENT_ID=eac_REPLACE_WITH_REAL_ID
emisell app dev --dir my-app --backend ./my-app/local-core-backend.mjs
```

ID contoh wajib diganti dengan ID rilis/app-client yang sebenarnya. Origin Core
harus HTTP literal 127.0.0.1; host/path lain ditolak. Adapter hanya mengakses
`/v1/app-platform/core/reviewed-ui/identity`, mengirim bearer identity dan
X-Emisell-App-Client yang dipin. Tidak ada Origin/cookie seller, API key, proxy,
redirect, URL dari token atau cache hasil grant. Timeout 4 detik, respons maksimal
8 KiB, identitas/client/app/expiry respons diperiksa sebelum diterima.

Core memverifikasi signature dan sesi in-memory, lalu sesi seller serta current
launch Platform. Core restart/expired/revoked akan menolak token. Ini kontrak
development opt-in, bukan API introspeksi production atau embedded-demo khusus.
Adapter tidak mengaktifkan flag Core, membuat rilis, memberi consent, atau
mengubah syarat HTTPS reviewed UI. Server Core harus mendukung reviewed UI.

Untuk project baru yang belum memiliki binding tersebut, default tetap 503.
Jangan mengganti ID dengan klaim dari browser agar terlihat berhasil.

## Verifier signature mandiri

Jalankan `emisell app dev --dir my-app --backend ./my-app/backend.mjs`.
Opsi backend mengeksekusi modul Node lokal: gunakan hanya kode yang Anda percaya.
Tanpa opsi ini, atau jika konfigurasi masih kosong, endpoint tetap 503.

Konfigurasi `createIdentityVerifier` pada backend.mjs:

- keys: map key ID ke public key Ed25519 SPKI PEM dari operator, bukan dari JWT/browser.
- issuer: issuer Platform tepercaya, audience: app-client ID yang direview.
- resolveExpected(hints, {audience, signal}): lookup binding backend independen, mengembalikan {merchantId, sub, appId, installationId}. Hints dari token bukan bukti instalasi aktif; jangan sekadar mengembalikan hints.
- authorizeCurrent(identity, {audience, signal}): periksa instalasi, grant, staf, app-client, rilis dan assignment terkini. Hanya boolean true berarti diizinkan. False/undefined ditolak; outage harus throw. Jangan gunakan callback selalu true selain pada fixture test sintetis.

Adapter produksi belum disediakan starter. Sambungkan layanan backend yang dipercaya;
jangan memakai JSON dari seller sebagai sumber izin. Terapkan signal pada request
downstream. Signature JWT saja bukan current authorization.

Verifier memeriksa Ed25519, tipe token, key ID, issuer, audience, binding dan lifetime
60 detik; expiry diperiksa kembali setelah authorization. Endpoint preview membatasi
5 detik, Origin same-origin dan body kosong. Respons tidak mengandung token:
401 invalid identity, 403 denied, 503 verifier/dependency unavailable.

Tidak ada sesi persisten atau polling. UI menghapus status sukses saat expired;
revocation diketahui saat request berikutnya. Operasi bisnis tetap harus memeriksa
izin sendiri. Identitas ini bukan resource token atau izin webhook.

File server dan konfigurasi tidak disajikan ke browser. Jangan simpan credential
di public/. Server preview khusus loopback, bukan deployment produksi.
