# App Gateway → Emisell Backend Resource API

Dokumen ini mendefinisikan **kontrak internal** untuk akses data merchant dari Emisell App Platform ke `api-service`. Kontrak mesin tersedia di [`emisell-resource-openapi.json`](./emisell-resource-openapi.json).

> Status: **implemented pilot, gated / nonaktif secara default**. Route produk di working tree api-service dan App Gateway telah diuji lintas repo dengan Prisma/PostgreSQL terisolasi. Ini bukan bukti bahwa deployment sudah terhubung. Aktivasi memerlukan konfigurasi kedua service, key khusus, dan allowlist merchant uji. Scope publik `read_products` tetap planned sampai checklist rilis selesai. Panduan dan batas bukti: [`resource-pilot.md`](./resource-pilot.md).

## Arah integrasi

Dokumen ini membahas arah berikut:

```text
Backend provider
  │ Authorization: Bearer es_at_* (installation token)
  ▼
Emisell App Gateway
  │ validasi app + installation + merchant + environment + effective scope
  │ membuat service assertion RS256 berumur ≤ 60 detik
  ▼
Emisell api-service
  │ validasi assertion + tenant + internal scope
  │ query data dengan merchantId pada query yang sama
  ▼
Database Emisell
```

Arah Emisell Backend → App Platform untuk membuka App Store, membuat merchant session, consent, dan Connected Apps dijelaskan terpisah di [`emisell-backend-integration.md`](./emisell-backend-integration.md).

## Temuan audit kontrak yang ada

Audit awal sebelum implementasi terhadap `api-service` menemukan:

- tenant utama adalah `Merchant.id` bertipe string opaque dengan default CUID;
- data product dimiliki merchant melalui `Product.merchantId`;
- route dashboard product menggunakan session/cookie dan konteks user merchant;
- entrypoint eksternal lama berada di `/ext/v1`, memakai `X-Sdk-Secret` dan menerima `x-merchant-id` sebagai konteks;
- model API key yang dipakai entrypoint tersebut sudah ditandai deprecated dan menyimpan key lama sebagai nilai langsung;
- route internal yang ada belum merupakan kontrak resource App Platform.

Karena itu App Platform **dilarang** memakai `/ext/v1`, meneruskan browser cookie, atau membuka route dashboard merchant kepada provider. Route baru harus memiliki namespace dan autentikasi khusus.

## Batas kepercayaan

1. Provider hanya mengirim installation token `es_at_*` ke App Gateway.
2. App Gateway menjadi policy enforcement point dan mengambil Merchant ID dari installation yang sudah diverifikasi.
3. Provider tidak boleh mengirim Merchant ID untuk memilih tenant.
4. App Gateway tidak pernah meneruskan installation token, client secret, developer session, atau merchant browser cookie ke `api-service`.
5. `api-service` hanya menerima service assertion yang diterbitkan App Gateway dan berumur sangat pendek.
6. Header routing tidak pernah menjadi sumber otoritas; signed claim adalah sumber kebenaran.

## Service assertion

Target production menggunakan JWT **RS256** dengan key khusus service-to-service. Masa berlaku maksimum 60 detik.

| Claim | Wajib | Nilai/aturan |
| --- | --- | --- |
| `iss` | Ya | `emisell-app-platform` |
| `aud` | Ya | `emisell-api-service` |
| `sub` | Ya | `app-gateway` |
| `iat` | Ya | Unix time saat assertion dibuat |
| `nbf` | Ya | Tidak sebelum `iat`, tidak lebih baru dari waktu verifikasi; zero clock tolerance |
| `exp` | Ya | Maksimum 60 detik setelah `iat` |
| `jti` | Ya | ID unik per assertion/request |
| `merchant_id` | Ya | Merchant ID dari installation tervalidasi |
| `installation_id` | Ya | Installation App Platform yang aktif |
| `app_id` | Ya | App yang memiliki installation |
| `environment` | Ya | `sandbox` atau `production` |
| `scope` | Ya | Gateway mempersempit effective scopes menjadi tepat `["read_products"]` untuk slice ini |

`api-service` wajib memverifikasi algorithm dan `kid`, signature, issuer, audience, subject, `iat`/`nbf`/`exp`, serta umur token maksimum. Untuk mutation mendatang, `jti` perlu dilindungi replay cache selama masa berlaku assertion.

Contoh payload dokumentasi—bukan token yang dapat digunakan:

```json
{
  "iss": "emisell-app-platform",
  "aud": "emisell-api-service",
  "sub": "app-gateway",
  "iat": 1788253200,
  "nbf": 1788253200,
  "exp": 1788253260,
  "jti": "req_01K4EXAMPLE000000000001",
  "merchant_id": "cmmerchantdemo000000000001",
  "installation_id": "01995f72-0000-7000-8000-000000000101",
  "app_id": "01995f72-0000-7000-8000-000000000201",
  "environment": "production",
  "scope": ["read_products"]
}
```

Private key dan JWT mentah tidak boleh berada di repository, dokumentasi, log, trace, atau error response.

## Header internal

| Header | Wajib | Aturan |
| --- | --- | --- |
| `Authorization` | Ya | `Bearer <short-lived-service-assertion>` |
| `X-Emisell-Merchant-ID` | Ya | Harus sama persis dengan claim `merchant_id`; mismatch menghasilkan `403` |
| `X-Emisell-Installation-ID` | Ya | Harus sama persis dengan claim `installation_id` |
| `X-Request-ID` | Direkomendasikan | Gateway selalu mengirim ID aman 16–128 karakter. Backend menghasilkan ID baru bila hilang/tidak valid; duplicate header ditolak |

Header merchant dan installation adalah defense-in-depth untuk routing dan observability, bukan kredensial mandiri. Middleware sebaiknya menghasilkan konteks khusus seperti `req.appContext`; jangan memalsukan `req.user` atau memakai konteks browser merchant.

## Slice pertama: `read_products`

Target route internal:

| Method | Path | Internal scope | Status |
| --- | --- | --- | --- |
| `GET` | `/internal/app-platform/v1/products` | `read_products` | Implemented pilot, default disabled |
| `GET` | `/internal/app-platform/v1/products/{productId}` | `read_products` | Implemented pilot, default disabled |

Orders, customers, inventory, fulfillment, dan mutation produk belum termasuk kontrak runtime ini. [Backend blueprint](./resource-backend-blueprint.md) mendefinisikan 12 target baca terpisah berdasarkan model sumber, beserta scope, projection, dan checklist implementasi. Blueprint bukan route berjalan dan tidak mengaktifkan scope; mutation tetap memerlukan review domain terpisah.

### Proyeksi data minimum

Response awal hanya memuat:

- `id`, `name`, `slug`, `sku`, `description`;
- `price` dan `compareAtPrice` sebagai decimal string agar presisi tidak berubah;
- `stock`, `isPhysical`, `isPublished`, `status`, `type`;
- `trackInventory`, `continueSellingWhenOutOfStock`;
- `createdAt` dan `updatedAt` dalam RFC 3339.

Field biaya internal (`cost`), relasi internal, credential, dan metadata operasional tidak diekspos. Variant sengaja belum dimasukkan ke slice pertama agar projection-nya ditinjau dari model sumber kebenaran sebelum menjadi kontrak publik.

### Makna harga, variant, dan stok

| Field | Sumber | Bukan berarti |
| --- | --- | --- |
| `price` | `Product.price`, decimal string | Harga termurah variant, harga display storefront, hasil pajak, atau konversi mata uang |
| `compareAtPrice` | `Product.compareAtPrice`, nullable decimal string | Compare-at price variant |
| `sku` | `Product.sku` | SKU variant |
| `stock` | Nilai nullable `Product.stock` | `stock - soldCount`, akumulasi stok variant, reservasi, atau stok siap jual |

Contoh yang diuji: harga dasar `60000.50`, variant `12000.25`, stok produk `45`, sold count `7`, dan stok variant `3` menghasilkan `price: "60000.5"` dan `stock: 45`. Trailing zero pada decimal tidak dijamin. Mata uang tidak dikembalikan pada slice ini; jangan mengasumsikan IDR, melakukan kalkulasi lintas mata uang, atau menggunakannya sebagai kontrak checkout. Tidak ada klaim bahwa harga ini sama dengan `/v1/products` pada dashboard/storefront lama.

`description` dapat memuat HTML dari merchant, sehingga consumer wajib melakukan escaping/sanitasi saat menampilkan. Daftar awal memuat produk published maupun unpublished milik merchant, kecuali filter `published` diberikan; status produk tidak difilter implisit.

### Isolasi tenant

Sebelum query produk, backend memeriksa keberadaan `Merchant`, `isActive=true`, dan tidak adanya `Suspension` bertarget merchant. Ketiganya memakai primary DB dalam transaksi repeatable-read yang sama dengan query produk. Merchant hilang, nonaktif, atau disuspend menghasilkan `403 merchant_unavailable` internal tanpa membedakan detailnya. Request berikutnya melihat perubahan policy yang sudah committed; request yang sedang berjalan memakai snapshot transaksinya. Tidak ada mutasi trial/billing atau peniruan browser session pada jalur ini.

Setiap query wajib membatasi merchant dalam query database yang sama:

```text
WHERE id = :productId AND merchantId = :signedMerchantId
```

Dilarang mengambil resource hanya berdasarkan ID lalu memeriksa merchant setelah data terambil. Resource milik tenant lain harus terlihat sebagai `404`, bukan `403`, untuk menghindari kebocoran keberadaan data.

### Pagination dan filter

- urutan stabil: `updatedAt DESC, id DESC`;
- cursor bersifat opaque dan tidak boleh dibuat/diurai provider;
- `limit` default 50, maksimum 100;
- filter awal: `updatedAfter` (UTC RFC 3339, suffix `Z`, maksimal tiga digit pecahan detik) dan `published` (`true`/`false`);
- response list membawa `meta.nextCursor`; nilai `null` berarti tidak ada halaman berikutnya.

## Response dan error

Success memakai envelope `data` dan, untuk list, `meta`. Error selalu memakai bentuk:

```json
{
  "error": {
    "code": "insufficient_scope",
    "message": "Required internal scope is missing.",
    "requestId": "req_01K4EXAMPLE000000000001"
  }
}
```

| Status | Makna | Retry |
| --- | --- | --- |
| `400` | Parameter/filter tidak valid | Perbaiki request, jangan retry otomatis |
| `401` | Service assertion hilang/tidak valid/kedaluwarsa | Buat assertion baru satu kali; jangan loop |
| `403` | Claim/header mismatch, scope kurang, allowlist/environment salah, atau merchant tidak aktif/tersuspend/tidak ditemukan | Jangan retry; hentikan dan audit |
| `404` | Resource tidak ada dalam tenant tersebut | Jangan retry otomatis |
| `429` | Rate limit | Hormati `Retry-After`, exponential backoff + jitter |
| `503` | Dependency backend sementara tidak tersedia | Bounded retry + jitter |

Error tidak boleh membocorkan apakah ID ditemukan pada tenant lain, isi claim, token, SQL, atau stack trace.

Tabel di atas adalah respons **internal**. App Gateway menyembunyikan 401/403 backend sebagai `503 resource_unavailable` bagi provider. Operator menggunakan `requestId` untuk diagnosis; jangan menebak Merchant ID atau retry tanpa batas. Uninstall divalidasi di gateway setiap request baru. Assertion yang telanjur diterbitkan tetap memiliki jendela berlaku singkat; endpoint baca ini tidak memiliki replay cache `jti`.

## Audit, log, dan rate limit

Catat minimal `requestId`, `jti`, app ID, installation ID, Merchant ID, environment, operation ID, status, latency, dan jumlah item. Jangan mencatat assertion, installation token, client secret, full request body, atau field produk yang tidak diperlukan.

Rate limit diterapkan sekurangnya per app + installation + merchant + route. App Gateway tetap menerapkan limit provider; limit di `api-service` menjadi lapisan pertahanan tambahan.

## Cakupan implementasi dan persiapan deployment

Jalur baca sudah ada di kode. Daftar berikut juga mencakup kebutuhan deployment/operasi yang tidak otomatis terpenuhi hanya dengan adanya implementasi. Lihat hasil dan batas pengujian di [`resource-pilot.md`](./resource-pilot.md).

### Di Emisell App Platform

- simpan konfigurasi issuer, audience, key ID, private key, dan key rotation melalui secret manager;
- mint assertion hanya setelah installation dan effective scope lolos validasi;
- gunakan Merchant ID dari installation context, bukan input provider;
- map response internal ke response Provider API yang data-minimized;
- propagasikan correlation ID, timeout, cancellation, dan safe error mapping;
- tambahkan contract test dan test bahwa token/secret tidak bocor ke log.

### Di `api-service`

- buat route group `/internal/app-platform/v1` yang terisolasi dari route browser dan `/ext/v1`;
- verifikasi RS256 assertion secara fail-closed dan dukung key rotation melalui `kid`;
- bangun `appContext` terverifikasi dan cocokkan header dengan signed claims;
- enforce `read_products` di middleware dan service layer;
- periksa merchant aktif/tidak disuspend di primary database pada setiap read tanpa menjalankan mutasi billing/trial;
- gunakan query tenant-safe, projection minimum, cursor stabil, rate limit, dan audit;
- tambahkan test assertion invalid/expired, scope kurang, header mismatch, pagination, dan akses lintas merchant.

## Definisi siap rilis

`read_products` hanya boleh diubah dari `planned` menjadi `available` jika seluruh kondisi ini terpenuhi:

1. kedua route internal sudah diimplementasikan sesuai OpenAPI;
2. key provisioning/rotation dan service discovery production sudah aktif;
3. test lintas merchant membuktikan tidak ada data leakage;
4. projection dan data classification disetujui;
5. rate limit, timeout, audit, alert, dan redaction log teruji;
6. App Gateway proxy/provider route memiliki contract dan integration test;
7. runbook rollback dan incident response tersedia;
8. review security Emisell selesai.

Sampai checklist tersebut selesai, `/v1/products` dan `/v1/products/{productId}` hanya untuk pilot yang diaktifkan operator. `read_products` tetap planned pada katalog publik dan app yang memintanya tidak dapat dipublikasikan ke App Store. Tidak ada aktivasi production otomatis.
