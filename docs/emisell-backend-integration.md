# Integrasi Emisell Backend dengan App Platform

Dokumen ini adalah panduan implementasi untuk menampilkan **Emisell App Store** di backend/merchant workspace Emisell, meneruskan identitas toko secara aman, menjalankan instalasi OAuth, dan menampilkan **Connected apps**.

Kontrak lengkap dan contoh interaktif tersedia di Swagger UI lokal: `http://localhost:8082`. Sumber kontraknya adalah [`openapi.json`](./openapi.json).

Dokumen ini membahas arah **Emisell Backend → App Platform** untuk App Store, merchant session, dan perhitungan ongkir internal. Arah sebaliknya—**App Gateway → Emisell Backend** untuk membaca resource merchant—memiliki batas kepercayaan terpisah di [`emisell-resource-api.md`](./emisell-resource-api.md). Baca produk kini memiliki implementasi pilot yang nonaktif secara default; lihat [`resource-pilot.md`](./resource-pilot.md). Ini bukan tanda integrasi kedua backend sudah aktif di deployment.

Untuk persiapan dari kedua dashboard, gunakan **Developer → App → Integration** dan **Admin → App catalog → Review app**. [Panduan handoff](./integration-handoff.md) memisahkan hasil pemeriksaan konfigurasi dari bukti uji integrasi; laporan tersebut tidak mengaktifkan koneksi ke api-service atau production access.

## Hasil akhir yang dituju

Satu merchant memiliki dua tampilan berbeda:

| Tampilan | Route referensi | Sumber data | Fungsi |
| --- | --- | --- | --- |
| App Store | `/merchant/app-store` | `GET /v1/catalog/apps` | Menemukan app yang sudah direview dan dipublikasikan Emisell |
| Connected apps | `/merchant/apps` | `GET /v1/merchant/installations` | Melihat app yang terpasang, versi, scope, status, dan melakukan uninstall |
| Consent | `/install` | preview + authorize merchant OAuth | Memastikan merchant melihat app, callback, dan permission yang benar sebelum install |

App Store tidak mengambil semua app berstatus aktif secara otomatis. Operator Emisell harus mempublikasikannya melalui **Admin Console → App catalog**. Ini mencegah app draft atau app internal muncul tanpa review.

## Pembagian tanggung jawab

### Emisell Backend

- mengautentikasi user Emisell;
- memastikan user boleh mengelola toko yang dipilih;
- menentukan `store_id`, nama, domain, dan environment;
- membuat JWT server-to-server berumur pendek;
- meminta one-time merchant session grant;
- mengarahkan browser ke `exchangeUrl` yang dikembalikan App Platform.

### Emisell App Platform

- memverifikasi signature, issuer, audience, waktu, `jti`, permission, dan identitas toko pada JWT;
- menyimpan hanya digest kode grant dan menolak pemakaian ulang;
- membuat merchant browser session yang terpisah dari Developer/Admin Console;
- menyediakan katalog app yang sudah dikurasi;
- menampilkan consent dan membuat installation yang terikat merchant + environment;
- menerbitkan OAuth access token yang terikat installation;
- menyediakan Connected Apps dan uninstall.
- memvalidasi instalasi + immutable shipping extension sebelum meneruskan kalkulasi ongkir ke API Kurir yang sudah dikonfigurasi operator.

### Backend milik developer app

- menerima klik dari `launchUrl` katalog;
- membuat OAuth `state`, PKCE verifier, dan PKCE S256 challenge;
- menyimpan `state` dan verifier di sesi server app;
- mengarahkan browser ke halaman consent App Platform;
- memverifikasi `state` pada callback;
- menukar authorization code dari server, bukan dari browser;
- menyimpan access token secara terenkripsi.

`launchUrl` bukan URL runtime Payment Gateway atau Shipping Gateway. Keduanya tetap extension runtime terpisah.

## Perhitungan ongkir internal

App Platform menyediakan `POST /v1/integrations/emisell/shipping/rates/calculate` sebagai pilot yang nonaktif secara default. Backend Emisell mengirim hanya `origin`, `destination`, dan `weight`; merchant serta environment berasal dari assertion yang terverifikasi dan permission `shipping.rates.calculate`. App Platform menolak selector merchant, app, installation, extension, courier, provider, credential, dan URL runtime.

Sebelum panggilan upstream, platform membutuhkan tepat satu instalasi aktif dengan immutable shipping extension yang mendeklarasikan capability tersebut. API Kurir kemudian memiliki penuh pilihan layanan/provider, credential, rate card, snapshot/cache, coalescing, dan quota. Detail kontrak, response, error, konfigurasi, dan checklist tersedia di [`shipping-rate-bridge.md`](./shipping-rate-bridge.md). Endpoint ini bukan Partner API untuk developer dan tidak berarti setiap request menggunakan kuota RajaOngkir.

## Alur masuk dari Emisell

```text
Browser merchant
  │  buka menu Apps pada Emisell
  ▼
Emisell Backend
  │  verifikasi user + hak akses toko
  │  sign JWT RS256 berumur ≤ 5 menit
  │  POST /v1/integrations/emisell/merchant-session-grants
  ▼
App Platform API
  │  simpan digest one-time code, TTL default 2 menit
  │  balas exchangeUrl
  ▼
Browser merchant
  │  GET exchangeUrl
  ▼
App Platform API
  │  consume code satu kali
  │  set HttpOnly merchant session + CSRF cookie
  │  303 ke /merchant/app-store
  ▼
Merchant App Store
```

Identitas merchant tidak boleh dikirim oleh browser dalam body grant. Body endpoint hanya menerima `returnTo`; identitas dan permission berasal dari JWT yang sudah ditandatangani Emisell Backend.

## Adapter yang tersedia di Emisell `api-service`

Emisell Backend sekarang menyediakan satu endpoint kecil untuk frontend merchant:

```http
POST /v1/app-platform/merchant-session
Content-Type: application/json

{"returnTo":"/merchant/app-store"}
```

Endpoint ini memakai cookie login Emisell yang sudah ada. `api-service` mengambil
`User.id` dan `Merchant.id` dari sesi, memastikan user adalah owner/staff aktif,
lalu membaca email, nama user, nama toko, dan main domain dari database Emisell.
Browser tidak dapat mengirim atau mengganti merchant ID, environment, permission,
email, maupun nama toko.

Jika berhasil, response berisi `data.exchangeUrl` dan `data.expiresAt`. Frontend
Emisell harus segera menjalankan `window.location.assign(data.exchangeUrl)`.
Jangan menyimpan URL tersebut di local storage, analytics, error reporting, atau
log karena query-nya memuat kode yang hanya berlaku sekali.

Implementasi dan konfigurasi server ada di
`src/modules/app-platform/session-bridge.*` serta
`src/modules/app-platform/SESSION_BRIDGE.md` pada repository Emisell
`api-service`. Adapter nonaktif secara default. Local development memakai bearer
khusus server; production wajib memakai assertion RS256 dengan private key yang
hanya tersimpan di Emisell Backend.

## JWT Emisell Backend

Production menggunakan token RS256 dengan key dan audience khusus integrasi ini. Jangan memakai ulang token Developer Console atau access token milik app.

| Claim | Wajib | Contoh | Keterangan |
| --- | --- | --- | --- |
| `iss` | Ya | `https://api.emisell.com` | Harus sama dengan konfigurasi App Platform |
| `aud` | Ya | `emisell-app-platform-store-bridge` | Audience khusus session bridge |
| `sub` | Ya | ID user Emisell opaque | Identitas user Emisell yang sedang bertindak; jangan diasumsikan UUID |
| `iat` | Ya | Unix time | Waktu token dibuat |
| `nbf` | Opsional | Unix time | Token belum boleh dipakai sebelum waktu ini |
| `exp` | Ya | Unix time | Maksimum 5 menit setelah `iat` |
| `jti` | Ya | ID unik 16–128 karakter | Mencegah replay request grant |
| `email` | Ya | email user | Identitas audit/session, tidak boleh diambil dari browser |
| `name` | Ya | nama user | Nama tampilan user |
| `store_id` | Ya | `Merchant.id` opaque | Store yang sudah diotorisasi Emisell Backend; gunakan nilai stabil yang sama dengan backend Emisell |
| `store_name` | Ya | nama toko | Nama merchant |
| `store_domain` | Opsional | hostname | Tanpa `https://` atau path |
| `environment` | Ya | `sandbox` / `production` | Mengikat session dan installation ke environment |
| `permissions` | Ya | `['apps.install']` | Harus memuat `apps.install` |

Contoh payload sebelum ditandatangani:

```json
{
  "iss": "https://api.emisell.com",
  "aud": "emisell-app-platform-store-bridge",
  "sub": "cmuserowner0000000000001",
  "iat": 1788253200,
  "nbf": 1788253200,
  "exp": 1788253380,
  "jti": "store-apps-20260901-000001",
  "email": "owner@example.com",
  "name": "Store Owner",
  "store_id": "cmmerchantdemo000000000001",
  "store_name": "Example Store",
  "store_domain": "store.example.com",
  "environment": "production",
  "permissions": ["apps.install"]
}
```

JWT mentah, private key, one-time code, cookie, dan CSRF token tidak boleh dicatat di log.

## Membuat merchant session grant

Request dilakukan dari Emisell Backend ke App Platform API:

```http
POST /v1/integrations/emisell/merchant-session-grants
Authorization: Bearer <short-lived-emisell-backend-jwt>
Content-Type: application/json

{
  "returnTo": "/merchant/app-store"
}
```

Response:

```json
{
  "data": {
    "exchangeUrl": "https://gateway.apps.emisell.com/auth/emisell-merchant/exchange?code=<one-time-code>",
    "expiresAt": "2026-09-01T08:02:00Z"
  }
}
```

Emisell Backend mengarahkan browser ke `exchangeUrl`. Jangan menukar kode ini dari backend karena cookie harus diterima oleh browser. Kode hanya dapat dipakai sekali dan kedaluwarsa dalam waktu singkat.

`returnTo` hanya menerima path relatif `/merchant`, `/merchant/...`, `/install`, atau `/install/...`. URL absolut dan protocol-relative URL ditolak untuk mencegah open redirect.

## Contoh local development

Development menyediakan bearer khusus server-to-server dan header simulasi. Nilai ini hanya untuk lokal dan tidak boleh dimasukkan ke frontend atau dipakai di production.

```sh
curl -X POST http://localhost:8081/v1/integrations/emisell/merchant-session-grants \
  -H 'Authorization: Bearer emisell-backend-local-token' \
  -H 'X-Emisell-Subject: 11111111-1111-7111-8111-111111111111' \
  -H 'X-Emisell-Token-Id: local-session-grant-000001' \
  -H 'X-Emisell-Email: owner@example.com' \
  -H 'X-Emisell-Display-Name: Store Owner' \
  -H 'X-Emisell-Store-Id: 22222222-2222-7222-8222-222222222222' \
  -H 'X-Emisell-Store-Name: Example Store' \
  -H 'X-Emisell-Store-Domain: store.example.com' \
  -H 'X-Emisell-Environment: sandbox' \
  -H 'X-Emisell-Permissions: apps.install' \
  -H 'Content-Type: application/json' \
  --data '{"returnTo":"/merchant/app-store"}'
```

Buka nilai `data.exchangeUrl` di browser. App Platform akan membuat session merchant dan mengarahkan browser ke App Store.

## Katalog dan publikasi app

Endpoint merchant-safe:

| Method | Path | Keterangan |
| --- | --- | --- |
| `GET` | `/v1/catalog/apps` | List app yang sudah dipublikasikan; mendukung `search`, `category`, `featured`, `cursor`, dan `limit` |
| `GET` | `/v1/catalog/apps/{appId}` | Detail app, versi aktif, extension type, dan required/optional scopes |

Endpoint operator:

| Method | Path | Keterangan |
| --- | --- | --- |
| `GET` | `/v1/internal/catalog/apps` | Kandidat publikasi beserta eligibility dan alasan blocking |
| `GET` | `/v1/internal/organizations/{organizationId}/apps/{appId}/catalog-listing` | Keputusan publikasi saat ini |
| `PUT` | `/v1/internal/organizations/{organizationId}/apps/{appId}/catalog-listing` | Simpan draft, publish, atau hide dengan optimistic revision |

Syarat publish:

1. organisasi developer aktif;
2. app aktif;
3. app memiliki immutable active version;
4. active version berstatus aktif;
5. app memiliki HTTPS `appUrl`/`launchUrl`;
6. operator Emisell memilih kategori dan melakukan publish.

Menonaktifkan app, organisasi, atau active version otomatis membuat listing tidak tampil walaupun keputusan operator sebelumnya `published`.

## Alur instalasi app

1. Merchant menekan **View & install** di App Store.
2. Browser membuka `launchUrl` milik backend developer app.
3. Backend developer membuat `state` dan PKCE verifier/challenge, lalu menyimpan verifier pada sesi server.
4. Backend developer mengarahkan browser ke:

```text
https://apps.emisell.com/install
  ?client_id=<client-id>
  &redirect_uri=<exact-registered-callback>
  &state=<random-state>
  &code_challenge=<pkce-s256-challenge>
  &code_challenge_method=S256
  &scope=read_merchant
```

5. App Platform mengambil merchant dari server-side session, bukan dari query/body.
6. Merchant meninjau required dan optional scopes lalu menyetujui.
7. App Platform mengarahkan browser ke callback developer dengan `code` dan `state`.
8. Backend developer memverifikasi `state`, lalu menukar code melalui `POST /oauth/token` menggunakan client secret dan PKCE verifier.
9. Installation muncul pada `/merchant/apps`; access token terikat pada app, merchant, environment, versi, dan scope yang disetujui.

## Error penting

| HTTP | Code | Arti umum |
| --- | --- | --- |
| `401` | `unauthorized` | Signature/issuer/audience/waktu token salah atau token tidak ada |
| `403` | `forbidden` | JWT tidak memiliki `apps.install` atau actor bukan operator |
| `409` | `conflict` | `jti` dipakai ulang atau revision listing sudah berubah |
| `422` | `validation_error` | Claim, kategori, return path, atau app eligibility tidak valid |
| `400` | `invalid_grant` | One-time code sudah dipakai, tidak dikenal, atau kedaluwarsa |

Setiap error API memiliki `requestId`. Simpan `requestId` untuk korelasi log tanpa menyimpan token atau kode rahasia.

## Konfigurasi production

Semua nilai berikut hanya berada pada server App Platform:

```text
PUBLIC_GATEWAY_URL=https://gateway.apps.emisell.com
EMISELL_BACKEND_SESSION_GRANT_TTL=2m
EMISELL_BACKEND_JWT_ISSUER=https://api.emisell.com
EMISELL_BACKEND_JWT_AUDIENCE=emisell-app-platform-store-bridge
EMISELL_BACKEND_JWT_KEY_ID=<active-signing-key-id>
EMISELL_BACKEND_JWT_PUBLIC_KEY_BASE64=<base64-encoded-RSA-public-key-PEM>
EMISELL_BACKEND_JWT_CLOCK_SKEW=30s
```

Jangan menggunakan prefix `NEXT_PUBLIC_`. Private key tetap hanya di Emisell Backend; App Platform hanya menerima public key.

Untuk cookie `SameSite=Lax` dan request browser yang konsisten, tempatkan frontend dan gateway pada domain yang same-site atau gunakan reverse proxy satu origin. Tetapkan HTTPS, allowlist CORS yang presisi, `Secure` cookies, rate limit, dan observability sebelum production.

## Checklist go-live

- RSA key minimum 2048 bit, disimpan dan dirotasi melalui secret/key manager.
- Issuer, audience, dan key ID khusus integrasi App Store.
- Maksimum umur JWT 5 menit dan `jti` unik per request.
- Hak akses user ke toko diverifikasi Emisell Backend sebelum JWT dibuat.
- Hanya operator yang dapat publish/hide katalog.
- App callback harus exact-match terhadap active version snapshot.
- Tidak ada token, code, cookie, CSRF, client secret, atau private key di log.
- Frontend dan gateway berjalan melalui HTTPS dan topologi cookie sudah diuji.
- Rate limit diterapkan pada endpoint grant, exchange, consent, dan token.
- Uji replay, open redirect, CSRF, session fixation, cross-tenant access, stale revision, dan uninstall.

## Managed extension credential boundary

For Emisell-managed extensions, App Platform owns encrypted provider credentials and installation–extension authorization. Emisell Backend supplies merchant identity over its authenticated integration and does not store these provider secrets. Merchant ID alone is not authentication: keep service assertions and tenant filtering on Backend resource endpoints. External developers retain their own OAuth client secrets and installation tokens. See [managed-extensions.md](./managed-extensions.md) for the default-off credential API and its execution/production limitations.

## App subscription billing

For Free/Paid app plans and consolidated merchant invoices, see [app-billing.md](./app-billing.md). The server-to-server billing endpoints require the separate `apps.billing.write` permission; `apps.install` is insufficient. The new adapter in `api-service` is tested in isolation but is not wired to the existing renewal cron or payment callbacks. Do not enable live charges merely because the API contract exists.
