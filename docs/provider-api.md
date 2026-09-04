# Emisell Partner API

Dokumen ini adalah panduan **Partner API** untuk developer yang telah diundang dan disetujui Emisell. Kontrak mesin yang menjadi sumber kebenaran ada di [`provider-openapi.json`](./provider-openapi.json). Route Developer Dashboard tidak termasuk; Payment dan Shipping memakai kontrak runtime capability yang ditinjau terpisah.

## Prinsip utama

- `Merchant.id` adalah identitas tenant yang stabil dan opaque. Nilainya dapat berbentuk CUID; provider tidak boleh mengurai, menebak, atau membuat Merchant ID sendiri.
- Merchant ID bukan kredensial. Provider selalu mengautentikasi dengan installation access token `es_at_*`.
- Provider tidak mengirim `merchantId` melalui header, query, atau body untuk memilih tenant. App Gateway mengambil tenant dari installation token yang sudah diverifikasi.
- Token, client secret, dan authorization code hanya boleh diproses server-to-server. Jangan letakkan nilai tersebut di browser, URL, log, analytics, atau source code.
- Payment Gateway dan Shipping Gateway adalah extension runtime terpisah. Scope data umum tidak memberi izin untuk melakukan capture, refund, membeli label, atau tindakan runtime lainnya.

## Alur instalasi

```text
Merchant Emisell
  → membuka App Store
  → meninjau scope required/optional
  → menyetujui instalasi
  → provider menerima authorization code pada redirect URI terdaftar
  → backend provider menukar code + PKCE verifier di POST /oauth/token
  → backend provider menyimpan installation token
  → backend provider memanggil Provider API dengan Bearer token
```

Authorization code bersifat single-use, berlaku singkat, memakai PKCE S256, dan terikat pada redirect URI yang persis sama. Token hasil pertukaran terikat pada satu app, satu instalasi, satu merchant, satu environment, dan satu versi aktif.

Untuk pengujian sebelum App Store publication, developer dapat membuat merchant-bound launch link dari Developer Console dengan memasukkan Merchant ID yang sudah terdaftar. Provider wajib mempertahankan `emisell_test_install_request` dari launch URL dalam session backend dan mengirim nilainya sebagai `test_install_request` pada URL consent. Detail lengkap tersedia di [`development-test-installation.md`](./development-test-installation.md). Request ID tidak menggantikan merchant session, consent, state, PKCE, client authentication, atau installation token.

## Scope efektif

Scope yang benar-benar berlaku adalah irisan dari:

1. scope pada versi app yang diinstal;
2. scope yang disetujui merchant;
3. scope pada access token yang masih aktif; dan
4. kebijakan akses resource yang aktif. Untuk Products pilot, route juga memerlukan aktivasi operator dan allowlist merchant di backend; katalog publik tetap planned untuk mencegah publikasi sebelum review rilis.

Perubahan versi, pencabutan consent, uninstall, suspend, credential revoke, atau token expiry dapat mengurangi atau menghapus akses. Karena itu provider sebaiknya memanggil `GET /v1/installation-context` saat memulai job penting dan menangani `401`/`403` secara aman.

## Katalog scope resmi

Katalog runtime tersedia di `GET /v1/scope-catalog`. Dashboard Developer juga memakai endpoint ini; nama scope tidak bisa ditulis bebas.

| Scope | Status | Data | Persetujuan |
| --- | --- | --- | --- |
| `read_merchant` | Available | Identitas dan metadata toko | Merchant consent |
| `read_products` | Planned untuk publik; implemented pilot default disabled | Field dasar produk, tanpa variant | Merchant consent + aktivasi pilot oleh operator |
| `write_products` | Planned | Mutasi produk dan variant | Merchant consent + review Emisell |
| `read_inventory` | Planned | Stok dan lokasi | Merchant consent |
| `write_inventory` | Planned | Penyesuaian stok | Merchant consent + review Emisell |
| `read_orders` | Planned | Order dan limited customer data | Merchant consent + review Emisell |
| `write_orders` | Planned | Mutasi order yang didukung | Merchant consent + review Emisell |
| `read_customers` | Planned | Personal data pelanggan | Merchant consent + review Emisell |
| `write_customers` | Planned | Mutasi profil pelanggan | Merchant consent + review Emisell |
| `read_fulfillments` | Planned | Shipment, tracking, delivery state | Merchant consent + review Emisell |
| `write_fulfillments` | Planned | Mutasi fulfillment yang didukung | Merchant consent + review Emisell |

`Planned` berarti belum tersedia untuk integrasi umum. Khusus `read_products`, implementasi pilot telah ada tetapi harus diaktifkan operator sesuai [`resource-pilot.md`](./resource-pilot.md). Resource planned lainnya belum memiliki endpoint. App dengan planned scope tidak dapat dipublikasikan ke App Store. Jangan menganggap pilot sebagai integrasi produksi yang siap.

## Endpoint yang tersedia sekarang

| Method | Path | Scope | Fungsi |
| --- | --- | --- | --- |
| `GET` | `/v1/scope-catalog` | Public | Katalog scope resmi dan status availability |
| `GET` | `/v1/webhook-event-catalog` | Public | Katalog event resmi dan status producer |
| `POST` | `/oauth/token` | HTTP Basic + PKCE | Menukar authorization code dengan installation token |
| `GET` | `/v1/installation-context` | Installation token | Membaca konteks dan scope efektif |
| `GET` | `/v1/merchant/profile` | `read_merchant` | Membaca profil merchant terhubung |

Tambahan **pilot nonaktif secara default**: `GET /v1/products` dan `GET /v1/products/{productId}`, keduanya memerlukan installation token dengan `read_products`. Panduan setup, pagination, contoh testing dan error ada di [`resource-pilot.md`](./resource-pilot.md); parameter dan schema berasal dari OpenAPI yang sama dengan Admin Documentation.

Products pilot mengembalikan harga/compare-at/SKU **produk dasar**, bukan harga/SKU variant atau harga display storefront. `stock` adalah nilai mentah produk, bukan stok siap jual; currency dan variant belum dikembalikan. Jangan memakai slice ini sebagai kontrak checkout/inventory. Backend juga memeriksa merchant aktif dan tidak disuspend setiap read. Kegagalan policy backend disembunyikan sebagai `503 resource_unavailable`; minta operator menelusuri `requestId` sebelum mencoba ulang tanpa batas.

Tidak ada endpoint inventory, orders, customers, atau fulfillments pada Provider API saat ini. Endpoint dashboard internal Emisell bukan kontrak provider dan tidak boleh dipanggil oleh app pihak ketiga.

## Contoh aman

Tukar authorization code dari backend provider:

```sh
curl --request POST 'http://localhost:8081/oauth/token' \
  --user 'CLIENT_ID:CLIENT_SECRET' \
  --header 'Content-Type: application/x-www-form-urlencoded' \
  --data-urlencode 'grant_type=authorization_code' \
  --data-urlencode 'code=CODE_FROM_CALLBACK' \
  --data-urlencode 'redirect_uri=https://provider.example.com/emisell/callback' \
  --data-urlencode 'code_verifier=PKCE_VERIFIER_FROM_SERVER_SESSION'
```

Validasi konteks instalasi:

```sh
curl 'http://localhost:8081/v1/installation-context' \
  --header 'Authorization: Bearer es_at_REDACTED'
```

Baca merchant profile:

```sh
curl 'http://localhost:8081/v1/merchant/profile' \
  --header 'Authorization: Bearer es_at_REDACTED'
```

Simpan `access_token` dalam secret store backend. Contoh di atas memakai placeholder dan tidak memuat secret yang dapat digunakan.

## Error yang wajib ditangani

- `400 invalid_grant`: code salah, kedaluwarsa, sudah dipakai, redirect URI berbeda, atau PKCE gagal. Mulai ulang instalasi; jangan mengulang code yang sama.
- `401 invalid_client`: client ID/secret salah atau credential dicabut. Hentikan retry dan minta credential baru.
- `401 invalid_token`: token tidak valid, kedaluwarsa, dicabut, instalasi suspended/uninstalled, atau app/organization tidak aktif. Hentikan job untuk instalasi tersebut.
- `403 insufficient_scope`: installation token tidak memiliki scope yang diperlukan. Jangan mencoba mengakali dengan Merchant ID; arahkan merchant ke consent/upgrade flow bila tersedia.
- `404`: resource tidak terlihat dalam tenant yang terautentikasi atau endpoint memang belum tersedia.
- `429`: lakukan exponential backoff dengan jitter dan hormati `Retry-After` ketika dikirim.

Jangan retry otomatis pada `400`, `401`, atau `403`. Untuk request mutasi di Provider API mendatang, dokumentasi endpoint akan menentukan idempotency key dan retry policy yang spesifik.

## Batas kepercayaan ke backend Emisell

App Gateway adalah policy enforcement point. Setelah token tervalidasi, gateway akan memanggil backend Emisell melalui jalur internal yang diautentikasi, membawa Merchant ID tepercaya dari installation context. Provider tidak menerima API key internal Emisell dan tidak boleh memanggil endpoint dashboard merchant secara langsung.

Kontrak resource berikutnya harus dibangun di backend Emisell sebagai route internal khusus App Gateway, bukan dengan membuka route browser/dashboard yang sudah ada. Setiap route baru harus memiliki filter tenant, data minimization, audit, rate limit, dan test lintas-merchant sebelum scope diubah dari `planned` menjadi `available`.

Kontrak internal tersedia di [`emisell-resource-api.md`](./emisell-resource-api.md) dan [`emisell-resource-openapi.json`](./emisell-resource-openapi.json), berstatus **implemented_gated**. Provider memanggil `/v1/products` melalui gateway, tidak langsung memanggil `/internal/app-platform/v1/products`. Kedua service default nonaktif untuk pilot; public scope availability masih planned.

## Webhook

Gunakan `GET /v1/webhook-event-catalog` sebagai sumber resmi daftar event. Developer Console membaca endpoint yang sama, sehingga selector event tidak memiliki daftar hardcoded terpisah. Hanya event `available` yang bisa dibuatkan subscription; event `planned` hanya mendokumentasikan nama, producer, dan scope yang akan diperlukan.

Saat ini event nyata yang tersedia adalah `app/uninstalled`, diproduksi oleh App Platform ketika instalasi merchant diputus dan tokennya dicabut. Payload ter-sign menyertakan `merchantId` dan `installationId` pada envelope agar provider dapat membersihkan data tenant secara tepat. Event products, inventory, orders, customers, dan fulfillments tetap `planned` sampai kontrak producer dari backend Emisell tersedia dan lulus pengujian lintas merchant.

Field `requiredScope` pada event catalog dan `requiredForWebhooks` pada scope catalog harus konsisten. Payload webhook harus diperlakukan sebagai data tenant; verifikasi signature dan timestamp sebelum memprosesnya, deduplikasi event ID, lalu respons cepat sebelum pekerjaan berat dijalankan secara asynchronous.

## Paid app features

The gated `GET /v1/installation-billing` endpoint returns the current subscription and `paidAccess` for the installation identified by its bearer token. It accepts no merchant/installation identity override. Check entitlement before serving paid features; an active app installation or resource scope is not proof of payment. No subscription is automatically created by OAuth consent. See [app-billing.md](./app-billing.md) for the merchant pricing/consent flow, expiry, cancellation and rollout boundaries.
