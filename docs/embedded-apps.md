# Embedded Apps Emisell — API dan panduan integrasi v1

## Status implementasi

Fondasi identitas, signed launch binding, adapter HTTP penerbitan sesi, shell iframe dan bridge browser tersedia serta diuji sebagai reference terisolasi. Repository PostgreSQL untuk review launch juga tersedia melalui migration 0018 (baru diterapkan ke database test). Belum dipasang pada listener live, sesi Core atau tombol Open app seller. Tidak ada endpoint token exchange/resource baru yang aktif. Tidak ada key otomatis, grant baru, migrasi instalasi, atau pembukaan scope Planned.

## Penyimpanan review launch

`internal/oauth/embedded/postgres.Repository` menyimpan metadata immutable berdasarkan app + client + release digest. Status `submitted → approved|rejected`, lalu `approved → revoked`. Approval menandatangani launch dan menulis audit secara atomik; hanya principal Admin administrator yang dapat memutuskan. Revoke tidak membutuhkan private key dan tidak membatalkan shipment. Submit identik oleh actor sama mengembalikan record existing; perubahan metadata pada binding yang sama ditolak. Retry keputusan harus cocok actor/reason/revision, dan approval lama tidak dapat menghidupkan record revoked.

Resolver membaca state terkini, memverifikasi signature serta exact app/client/release digest, tanpa cache grant. Application service `embedded.Reviews` kini memverifikasi ownership melalui AppClients dan menahan release/client lock saat Submit/Approve. Launch harus satu origin dengan endpoint yang diverifikasi. Reject/revoke tetap tersedia bagi administrator walau key/proof tidak tersedia.

API portal tersedia melalui komposisi eksplisit `bootstrap.HandlerWithEmbeddedReviews` dengan key signing khusus dan parent origin dari konfigurasi operator:

- `POST /api/v1/developer/embedded-launches`: body clientId, url, reason. App/digest/parent tidak diterima dari browser.
- `GET /api/v1/developer/embedded-launches/{id}`: ownership organisasi.
- `GET /api/v1/admin/embedded-launches/{id}`: sesi Admin.
- `POST /api/v1/admin/embedded-launches/{id}/status`: revision, status approved/rejected/revoked, reason; administrator saja.

Kontrak lengkap: `api/openapi/embedded-launches.v1.json`, juga tampil pada Dokumentasi API Admin. Listener default belum memakai komposisi opt-in ini. Response tetap `launchable:false`: approval URL belum berarti installation bisa diluncurkan. Tes portal menggunakan cookie/sesi/ownership asli dengan proof endpoint tiruan dan database terisolasi, bukan sesi merchant produksi.

**Gap sebelum sambungan Core:** app-client saat ini terikat integration release; instalasi shipping managed tidak mempunyai app-client tersebut. Harus ada binding installation–release–client yang eksplisit dan diuji, bukan menyamakan ID/digest antar-pipeline atau melewati grant. Generic integration masih tidak installable pada pipeline existing. Bridge Core/Open app seller belum diaktifkan oleh API review ini.

Migration 0018 additive, tanpa backfill atau aktivasi launch instalasi existing. Sebelum menjalankan binary baru terhadap database development: backup privat, apply migration eksplisit, lalu restart terkoordinasi. Jangan menjalankan migration test terhadap database pengguna. Setelah record dipakai, rollback dengan menonaktifkan fitur atau forward-fix; jangan drop audit/launch data.

RajaOngkir adalah calon consumer pertama, bukan nama dalam protokol. API-Kurir tetap engine pengiriman. Login website Komerce di iframe bukan mekanisme autentikasi embedded ini.

## Alur target

1. Seller login ke Core. Core memvalidasi sesi, merchant membership dan izin staf terkini.
2. Open app menyelesaikan binding installation → app-client → release → URL launch terverifikasi. Endpoint API/health/OAuth tidak boleh dianggap URL iframe.
3. Bridge meminta token identitas melalui backend Core yang terautentikasi, bukan memakai key Platform dari browser.
4. Parent mengirim token melalui pesan ke iframe yang origin dan window-nya telah diverifikasi. Token hanya disimpan di memori, tidak di URL, cookie, localStorage, log atau analytics.
5. Backend aplikasi memvalidasi signature, issuer, audience dan identity binding; pemeriksaan current grant tetap diperlukan.
6. Akses resource kelak memakai access token berbeda, diperoleh melalui pertukaran server-to-server yang memverifikasi confidential client dan current grant. Token identitas sendiri tidak dapat dipakai memanggil API resource.

Langkah 1–6 adalah target integrasi. Bridge dan verifier signed launch sudah tersedia; storage/review metadata launch persisten, adapter authorization Core/Platform dan token exchange belum tersedia.

## Endpoint reference (belum dipasang di server live)

`internal/oauth/embedded.LaunchHandler` adalah adapter BFF same-origin Core. Nama route final harus didaftarkan bersama adapter sesi Core; `/embedded/session` hanya path pada tes, bukan endpoint di port 8087/8088.

- Method POST, `Content-Type: application/json`, exact `Origin` parent; tanpa query.
- Body hanya `{"installationId":"installation-id"}`, maksimum 1024 byte. Merchant/staf/app tidak diterima dari body.
- Adapter `BrowserIdentity` memeriksa sesi, CSRF, membership dan permission serta memetakan installation menjadi identity. Callback test bukan authentication production.
- Resolver wajib mengambil current signed release dan mencocokkan digest installation, bukan menerima URL dari browser. Signature launch mengikat app, client, release digest, URL dan parent origin dengan domain signature terpisah.
- Response: `{launch: {appId, clientId, releaseDigest, url, parentOrigin}, identityToken, expiresIn: 60}`, selalu `Cache-Control: no-store`.
- 400 body invalid; 401 sesi tidak valid; 403 origin/CSRF/binding/grant ditolak; 405 method; 415 media type; 503 dependency/adapter unavailable. Tidak ada token di response error.
- Sebelum mounting live: persistent reviewed resolver, real auth/CSRF, audit, rate limiting dan key provisioning wajib tersedia. HTTPS production; opsi HTTP hanya loopback `127.0.0.1` untuk test eksplisit.

## Browser SDK reference

Module `pkg/embedded/bridge.mjs` menyediakan:

- `mountEmbeddedApp({container, getSession})`: membuat iframe terisolasi dari launch response backend, tanpa token di URL; mengembalikan cleanup function.
- `connectFrame(...)`: memasang handler parent yang memeriksa exact origin, window, protocol dan nonce; membuang respons yang tertunda setelah frame load/navigation/disposal.
- `getIdentity({parentOrigin})`: aplikasi meminta token baru setelah load dan mengirimkannya sebagai bearer ke backend aplikasinya. Timeout default 5 detik; token hanya di memori.

`getSession` harus memanggil BFF same-origin dengan session/CSRF existing. Tidak ada browser key Platform atau secret provider. Bridge tidak menyediakan arbitrary navigation, resource access, login Komerce, atau postMessage wildcard. Handler membatasi satu request pending dan 120 nonce per frame load; server tetap memerlukan rate limit. SDK bukan paket yang sudah dipublish.

## API Go yang tersedia

Package `emisell.app/platform/pkg/embedded`:

```go
Issue(privateKey, keyID, issuer, audience, identity, now) (string, error)
Verify(token, trustedPublicKeys, issuer, audience, expectedIdentity, now) (Claims, error)
```

Gunakan `internal/oauth/embedded.Service` dalam composition root Platform:

```go
service.Issue(ctx, identity, appClientID) (string, error)
service.Authenticate(ctx, token, appClientID, expectedIdentity) (Claims, error)
```

`Service.Authorize` wajib diimplementasikan oleh adapter trusted server. Hook dipanggil pada setiap Issue dan Authenticate; nil atau error menolak akses. Harus memeriksa staff membership/permission, merchant, installation dan grant aktif, release/assignment valid, serta app-client/launch binding yang cocok. Jangan mengisi hook dengan `return nil` pada deployment. Signature check tidak menggantikan pemeriksaan tersebut.

`expectedIdentity`, issuer, audience dan trusted keys berasal dari konfigurasi/routing server yang dipercaya, bukan klaim token yang belum diverifikasi atau body browser. Signature verification tidak melakukan network fetch terhadap URL dari token.

## Kontrak token

| Field | Arti |
|---|---|
| `alg` | EdDSA (Ed25519) saja |
| `typ` | `emisell-embedded-id+jwt`, bukan token resource |
| `kid` | ID key dalam trust store terkonfigurasi |
| `iss` | Identitas issuer Platform yang dipin backend |
| `aud` | ID app-client tujuan |
| `sub` | ID staf Core |
| `merchantId` | Merchant yang diotorisasi, tanpa tenant ID kedua |
| `appId` | Aplikasi tujuan |
| `installationId` | Instalasi spesifik; reinstall memakai binding baru |
| `iat`, `exp` | Unix seconds; lifetime tepat 60 detik, tidak menerima future-issued/expired |
| `jti` | ID acak per penerbitan, bukan receipt idempotency |

Ini bearer token: dapat digunakan ulang dalam masa berlaku oleh pemegangnya. `jti` bukan proteksi replay otomatis. Operasi bisnis mutating tetap membutuhkan authorization terkini, idempotency dan audit. Token tidak memuat secret Komerce, email, password atau daftar scope yang dianggap grant.

## Error dan keamanan

- `embedded.ErrInvalid`: format/signature/key/issuer/audience/identity/waktu tidak valid. Adapter HTTP kelak memetakan ke 401 tanpa membocorkan token.
- Authorization ditolak: adapter membedakan 403 dari unavailable 503; jangan mengganti outage menjadi hasil sukses/daftar kosong.
- `service.ErrUnavailable`: authorization hook belum tersedia. Implementasi default fail closed.
- Key Ed25519 khusus embedded, terpisah dari signer manifest, key engine, dan key Core. Provision/rotation/KMS dan distribusi public key belum diimplementasikan. Jangan menulis key ke source atau menyediakan private key pada app backend.
- Validasi JWT bukan otorisasi terhadap satu order tertentu. Object-level access tetap wajib.

## Kontrak bridge yang harus dipenuhi sebelum rollout

- Origin parent dan app ditetapkan konfigurasi server, bukan `*` atau query URL.
- Verifikasi `event.origin`, `event.source`, versi pesan dan request nonce; reply hanya ke window/origin yang sesuai.
- Iframe memakai CSP `frame-ancestors` allowlist dan sandbox minimum. Jangan mengatasi kegagalan embed dengan menonaktifkan browser security atau mem-proxy cookie Komerce.
- Jangan mengirim token sebelum frame siap dan handshake cocok. Setelah refresh/navigasi/uninstall, identitas lama tidak boleh diwariskan ke aplikasi lain.
- Navigasi hanya target terdaftar; bridge tidak membuka arbitrary URL atau menjalankan JavaScript dari pesan.

## Gate berikutnya dan migration path

1. Tambahkan launch metadata versioned, reviewed, signed dan persisten serta eligibility app-client; release historis tanpa launch metadata tetap tidak embedded-launchable.
2. Implementasikan adapter current-access dan authenticated Core facade dengan CSRF, exact Origin, rate limit, no-store, audit tanpa token dan error mapping.
3. Hubungkan iframe shell + Bridge serta uji lintas merchant/staf/app, refresh, frame spoofing dan uninstall.
4. Implementasikan token exchange terpisah; seluruh resource Planned tetap ditolak sampai endpoint/permission tersedia.
5. Bangun UI operasional RajaOngkir dan koneksi API-Kurir tanpa memindahkan credential provider ke browser.

Rollout feature-gated. Revoke/disable launch tidak menghapus installation atau membatalkan shipment. Server lama dan manifest immutable tetap kompatibel; tidak mengaktifkan embedded bagi instalasi existing otomatis.

## Demo lokal yang dapat dijalankan

Lihat [Panduan demo embedded](embedded-demo.md) untuk menjalankan parent dan aplikasi uji pada 127.0.0.1:4320/4321. Demo memakai identitas sintetis dan tidak terhubung ke instalasi merchant. Keberhasilan demo tidak menutup gate integrasi Core di atas.
