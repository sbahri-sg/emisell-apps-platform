# Emisell Resource API — panduan implementasi backend

Status: **kontrak desain, bukan 12 endpoint yang sudah berjalan**. Ditinjau dari source lokal `api-service/prisma/models` pada 3 September 2026, bukan audit database/deployment. Dokumen ini menetapkan target slice baca agar tim backend punya acuan implementasi; perlu review pemilik domain sebelum rollout.

Buka **Admin → Documentation → Resource API · Backend blueprint** untuk parameter, scope, aturan tenant, mapping field, contoh respons, serta unduhan OpenAPI/Postman. Dokumentasi internal tetap memerlukan session Admin. Postman blueprint memakai origin `.example.invalid` dan melewati setiap request tanpa sakelar untuk mengaktifkannya.

## 1. Kontrak yang harus diikuti

| Artifact | Makna | Boleh dianggap tersedia? |
| --- | --- | --- |
| [`emisell-resource-openapi.json`](./emisell-resource-openapi.json) | Dua GET produk yang sudah diimplementasikan dan diuji sebagai pilot | Hanya jika kedua service dikonfigurasi; default off, bukan GA |
| [`emisell-resource-blueprint.openapi.json`](./emisell-resource-blueprint.openapi.json) | Dua belas GET tambahan beserta proyeksi dan aturan backend | Tidak; design-only |
| [`provider-openapi.json`](./provider-openapi.json) | Permukaan API untuk backend app developer | Ikuti status per endpoint; blueprint tidak dimasukkan ke sini |

Source blueprint adalah [`scripts/sync-resource-blueprint.mjs`](../scripts/sync-resource-blueprint.mjs). OpenAPI, mapping di dashboard, contoh, dan Postman berasal dari source yang sama. Jangan menyunting JSON hasil generate atau menduplikasi daftar field dalam UI.

Tidak ada perubahan `api-service`, database, kunci, flag deployment, Payment Gateway atau Shipping Gateway dalam perubahan dokumentasi ini. Tidak ada paket SDK Emisell yang dipublikasikan.

## 2. Temuan SDK Shopify dan keputusan Emisell

SDK adalah **client** untuk app developer, bukan engine backend atau pengganti permission middleware. SDK Shopify tidak dapat diarahkan ke Emisell begitu saja: protokol, identitas toko, token, nama field, dan semantics berbeda.

| Pola Shopify | Keputusan Emisell untuk tahap ini |
| --- | --- |
| Admin API client menerima konfigurasi shop, API version, dan access token; menyediakan typed requests serta bounded retries | SDK Emisell kelak membungkus **App Gateway** dengan installation token, explicit `/v1`, timeout dan error aman. Tidak mengambil Merchant ID dari argumen request. [SDK resmi](https://github.com/Shopify/shopify-app-js/blob/main/packages/api-clients/admin-api-client/README.md) |
| REST Admin API berstatus legacy dan app publik baru menggunakan GraphQL | Pertahankan REST + OpenAPI yang sudah dipakai pilot. GraphQL memerlukan keputusan arsitektur terpisah, bukan perubahan terselubung. [REST status](https://shopify.dev/docs/api/admin-rest/latest) |
| Scope diberikan saat otorisasi; di Shopify write scope mencakup read | Emisell memakai scope eksplisit pada setiap operasi. **Jangan menganggap write otomatis read** karena implementasi Emisell belum mempunyai aturan tersebut. [Access scopes](https://shopify.dev/docs/api/usage/access-scopes) |
| GraphQL connections memakai cursor/pageInfo | Emisell mempertahankan `{data, meta: {nextCursor}}`, limit 50/max 100 dan cursor opaque. [Pagination](https://shopify.dev/docs/api/usage/pagination-graphql) |
| API versioned Shopify mengikuti jadwal kuartalan | Emisell memakai major API `/v1`, terpisah dari versi app dan versi dokumen. Tidak meniru jadwal/support window yang belum bisa dijamin. [Versioning](https://shopify.dev/docs/api/usage/versioning) |
| GraphQL Admin API memakai calculated query cost | Emisell memakai HTTP 429/Retry-After; bukan quota/cost model Shopify. [Limits](https://shopify.dev/docs/api/usage/limits) |

Ini adaptasi pola, **bukan klaim kompatibel atau setara fitur dengan Shopify**.

## 3. Batas tanggung jawab

1. **Backend app developer** menjalankan OAuth melalui App Platform, menyimpan installation token secara terenkripsi, lalu memanggil Provider API. Secret tidak boleh masuk frontend/browser.
2. **App Gateway** memverifikasi token, status instalasi, akses app, dan effective scope pada setiap request. Merchant ID, app ID, installation ID, dan environment berasal dari konteks tervalidasi. Gateway menerbitkan assertion RS256 singkat untuk operasi tersebut.
3. **Backend Emisell (`api-service`)** memverifikasi assertion, memeriksa merchant aktif/tidak disuspend, menerapkan query tenant-safe, dan memproyeksikan field yang diizinkan. Backend tetap source of truth data commerce; tidak menyimpan credential provider sebagai bagian Resource API.

Merchant ID merupakan **identitas tenant, bukan password**. Developer tidak boleh memilih merchant hanya dengan mengganti query/header. Jalur `/ext/v1`, `X-Sdk-Secret` lama, browser session, dan credential Admin tidak digunakan untuk Resource API ini.

Assertion target mengikuti [`emisell-resource-api.md`](./emisell-resource-api.md): dedicated RS256 key + `kid`, issuer/audience/subject tetap, umur maksimum 60 detik, `iat/nbf/exp`, `jti`, dan identitas konteks wajib. Untuk setiap endpoint, `scope` harus tepat sama dengan `x-emisell-required-scopes`; jangan kirim seluruh scope instalasi. HTTP bearer OpenAPI memakai `security: [{appGatewayAssertion: []}]`; scope internal bukan OAuth security-array.

Header `X-Emisell-Merchant-ID` dan `X-Emisell-Installation-ID` wajib cocok dengan signed claims. Jangan mengandalkan header sendirian. Internal environment tetap batas kepercayaan service, bukan pilihan baru pada form developer. Assertion read yang sudah diterbitkan memiliki jendela hidup singkat; jangan menjanjikan pencabutan instan pada request yang sudah berjalan.

## 4. Scope dan urutan slice

Seluruh path tabel berikut relatif terhadap `/internal/app-platform/v1`. Provider target kelak memakai suffix yang sama di `/v1`, tetapi **belum menjadi route Provider API**. Metadata `x-emisell-backend.providerTargetPath` bukan URL yang bisa dipanggil sekarang.

| Slice | Target GET | Scope tepat | Model sumber utama |
| --- | --- | --- | --- |
| Variant | `/products/{productId}/variants`, `/variants/{variantId}` | `read_products` | `Product`, `Variant` |
| Lokasi & inventory | `/locations`, `/locations/{locationId}/inventory` | `read_inventory` | `StoreLocation`, `StoreLocationItem`, `Product`, `Variant` |
| Order | `/orders`, `/orders/{orderId}`, `/orders/{orderId}/items` | `read_orders` | `Order`, `OrderItem` |
| Fulfillment | `/orders/{orderId}/fulfillments`, `/fulfillments/{fulfillmentId}`, `/fulfillments/{fulfillmentId}/items` | `read_fulfillments` | `Order`, `OrderFulfillment`, `OrderFulfillmentItem`, `OrderItem` |
| Customer | `/customers`, `/customers/{customerId}` | `read_customers` | `Customer` |

Urutan implementasi yang disarankan: variant setelah pilot produk, lokasi/inventory setelah rekonsiliasi ledger, order/fulfillment setelah review semantics, lalu customer setelah review data pribadi. Scope tersebut tetap `planned` dalam katalog publik. `read_merchant` sudah memakai konteks profil gateway, sehingga tidak dibuat endpoint backend duplikat.

Scope `read_orders`, `read_fulfillments`, dan `read_customers` memerlukan consent dan review Emisell sesuai katalog yang ada. Scope yang sah tidak berarti semua data model boleh dibaca: proyeksi endpoint tetap membatasi field. Route fulfillment tidak otomatis memberikan akses customer/order lengkap.

## 5. Mapping dan keputusan domain yang disengaja

OpenAPI menandai setiap field proyeksi dengan `x-emisell-source`, misalnya `Variant.price` atau `OrderItem.productSnapshot.name`. Hanya field dalam schema yang boleh keluar; jangan mengembalikan hasil Prisma mentah.

- **Variant:** `Variant` tidak memiliki `createdAt` atau `updatedAt` pada schema saat diperiksa. Tidak ada timestamp buatan, filter `updatedAfter`, atau janji delta-sync. Harga decimal string bukan float. Mata uang tidak ditambahkan tanpa sumber yang ditinjau.
- **Inventory:** `StoreLocationItem.available` dan `.onHand` adalah nilai ledger tersimpan; jangan hitung dari `Product.stock`, `soldCount`, atau `Variant.stock`. Tidak ada jaminan stok reservasi checkout. Ledger perlu direkonsiliasi dengan domain inventory sebelum diaktifkan.
- **Tenant inventory:** `StoreLocationItem` tidak mempunyai `merchantId`. Query harus lewat lokasi milik signed merchant **dan** validasi item polymorphic `PRODUCT`/`VARIANT` milik merchant yang sama. Row orphan/relasi silang dikeluarkan; jangan bocorkan item tenant lain hanya karena lokasi cocok.
- **Order:** hanya `currentApp=ONLINE_STORE`; draft dan abandoned order tidak termasuk. `Order.status` dipertahankan, bukan diberi label baru `paymentStatus`. `subtotal`, `totalTax`, dan `shippingFee` adalah komponen, bukan janji grand total atau jumlah yang perlu dibayar. `currencyCode` hanya dari snapshot valid, selain itu null—tidak default IDR.
- **Line item:** ambil allowlist string dari snapshot order, bukan join nama/harga Product saat ini. `productId`/`variantId` yang sudah null tetap null; relasi ID tidak boleh mengekspos tenant lain. Nama custom/legacy yang tidak tersedia bernilai null. `quantity` bukan sisa untuk dikirim. Detail item dipaginasi terpisah agar payload terbatas.
- **Customer:** `merchantId` harus cocok dan `deletedAt=null`. Jangan memperluas lewat `customerMerchants` atau relasi `User`. Contact nullable dihormati; tidak ada lookup tambahan ke User. Belum mencakup alamat, marketing consent, pengayaan data, notes atau amount spent.
- **Fulfillment:** pemilik fulfillment dan parent order harus sama. `OrderFulfillmentItem.orderItemId` harus menunjuk order yang sama, bukan sekadar fulfillment ID yang benar. Cost, address Shipment, raw `trackingList`, hold reasons, dan booking shipment tidak termasuk. Tracking terstruktur memerlukan normalisasi dan review terpisah.

Status adalah string sumber yang dapat berkembang (`x-extensible-enum` menjelaskan nilai saat ini). UI/SDK harus punya fallback unknown. Semua teks/HTML merchant tidak dipercaya; escape sebelum rendering. Field wajib yang rusak menghasilkan error aman, bukan konversi diam-diam; hanya fallback null yang dinyatakan dalam schema yang diizinkan.

## 6. Pagination, respons dan error

- Default limit 50, maksimum 100, minimum 1. Tolak query tidak dikenal dan parameter duplikat.
- Order/customer/fulfillment list: `updatedAt DESC, id DESC`. `updatedAfter` harus UTC dengan `Z` dan maksimum tiga digit pecahan; lebih besar secara strict.
- Variant/lokasi/inventory/order item/fulfillment item list: `id ASC`, tanpa filter incremental. ID opaque; cursor keyset menggunakan perbandingan string yang konsisten dengan database, bukan timestamp yang ditebak dari CUID.
- Cursor ditandatangani dan terikat pada app, merchant, installation, environment, API version, operation, parent, filters, dan ordering. Mengubah salah satu menghasilkan `400 invalid_cursor`. Cursor bukan credential dan tidak boleh memuat secret/PII.
- Page kosong milik tenant sendiri: `{"data":[],"meta":{"nextCursor":null}}`. Parent hilang, foreign tenant, soft-deleted atau di luar cakupan menghasilkan `404`, bukan page kosong.
- Pagination bukan snapshot seluruh dataset selama penulisan berlangsung. `updatedAfter` bukan delete feed; penghapusan/customer privacy belum terjangkau webhook resource. Jangan menjanjikan sinkronisasi penuh hanya dengan polling ini.
- Target implementasi mempertahankan timeout gateway 5 detik dan batas respons 4 MiB. Proyeksi gagal/oversize tidak boleh menjadi success parsial diam-diam; gunakan error aman dan request ID.
- Seluruh success/error memakai `Cache-Control: no-store` dan `X-Request-ID` aman.

| Internal status | Perilaku backend | Perilaku gateway/consumer |
| --- | --- | --- |
| 400 | Validasi query/cursor gagal | Kembalikan error aman; jangan retry tanpa memperbaiki input |
| 401 | Assertion invalid/expired | Gateway menyembunyikan sebagai 503; jangan meneruskan diagnosis kunci ke provider |
| 403 | Scope/context/merchant policy gagal | Gateway menyembunyikan sebagai 503; scope token provider yang kurang ditolak gateway sebagai 403 sebelum upstream |
| 404 | Resource tidak terlihat di tenant | Jangan membedakan foreign-tenant dengan tidak ada |
| 429 | Rate limit | `Retry-After` detik, bounded backoff + jitter |
| 503 | Disabled, timeout, invalid projection/dependency | Jangan memberikan success kosong; retry transient secara terbatas, disabled/policy perlu operator |

Tidak ada retry otomatis baru pada gateway dalam perubahan ini. Nilai aggregate rate limit perlu diputuskan lewat uji kapasitas; angka pilot per-process bukan jaminan untuk seluruh resource. Jangan log token, assertion, customer data, URL-query/cursor, raw snapshots atau SQL error.

## 7. Contoh pola client untuk developer

Contoh di bawah hanya menunjukkan request server-side ke **endpoint product pilot yang sudah ada**, bukan implementasi SDK baru. Jalankan setelah mengikuti [`resource-pilot.md`](./resource-pilot.md), consent merchant, dan memperoleh token instalasi. Jangan memakai blueprint yang belum diimplementasikan sebagai endpoint live.

```ts
// Backend app developer saja. Config origin dikelola server, bukan dari input user.
async function readFirstProductPage(installationToken: string) {
  const origin = new URL(process.env.EMISELL_APP_GATEWAY_URL!);
  if (origin.protocol !== 'https:' || origin.username || origin.password ||
      origin.pathname !== '/' || origin.search || origin.hash) {
    throw new Error('Configure a trusted HTTPS App Gateway origin');
  }
  const url = new URL('/v1/products', origin);
  url.searchParams.set('limit', '50');
  const response = await fetch(url, {
    headers: { Authorization: `Bearer ${installationToken}`, Accept: 'application/json' },
    redirect: 'manual', // Never follow redirects with credentials.
    cache: 'no-store',
    signal: AbortSignal.timeout(5000),
  });
  if (!response.ok) {
    // Do not log the request headers, token or raw response body.
    throw new Error(`Resource request failed (${response.status})`);
  }
  return response.json(); // A published SDK must additionally validate the response schema/size.
}
```

Tidak ada parameter Merchant ID, internal JWT, secret Shopify atau import paket `@emisell/*` fiktif. Kontrak SDK berikutnya boleh menambahkan type generation dari Provider OpenAPI, pagination helper, AbortSignal, dan safe structured errors setelah permukaan provider stabil. OAuth, penyimpanan token, refresh/reinstall, dan client resource adalah tanggung jawab berbeda; contoh di atas tidak menyelesaikan seluruh lifecycle.

## 8. Checklist implementasi untuk tim api-service

1. Pilih satu slice; review proyeksi, data classification dan kebutuhan app nyata. Jangan aktifkan seluruh scope sekaligus.
2. Tambahkan route di namespace resource internal yang terpisah dari browser dan `/ext/v1`. Gunakan app context tervalidasi, bukan `req.user` palsu.
3. Reuse verifier dan merchant policy pilot, perluas exact-scope allowlist **per operasi**, tetap default off. Implementasi verifier pilot saat ini hanya `read_products`; blueprint tidak otomatis memperluasnya.
4. Tambahkan query/model adapter dengan tenant predicate dan explicit select. Semua parent/child harus sesuai tenant; verifikasi orphan/inconsistent relations dan field JSON legacy.
5. Tambahkan request/response contract tests dan fixtures disposable; uji assertions salah/kedaluwarsa, merchant nonaktif, cursor context mismatch, relasi silang, PII minimization, precision, empty/404, timeout dan payload limit.
6. Setelah backend lulus, buat adapter/proyeksi dan route gateway, validasi installation lifecycle/scope, safe upstream error mapping. Tambahkan Provider OpenAPI beserta unit/integration test.
7. Promosikan hanya operasi yang selesai dari blueprint ke kontrak runtime; hapus target duplikat. Perbarui status/proyeksi/docs dari source yang sama. **Jangan memindahkan semuanya hanya karena kontrak sudah ditulis.**
8. Sebelum GA: keys/rotation, deployment test lintas merchant, capacity/aggregate limit, privacy/retention review, rollback dan persetujuan Emisell. Public scope availability tetap planned sampai bukti lengkap.

### Pemeriksaan lokal dokumen

```sh
npm run validate:contracts
npm run test:docs
node scripts/check-resource-blueprint-source.mjs /absolute/path/to/api-service
```

Pemeriksa source bersifat read-only: memastikan model/field akar benar-benar ada, nilai enum sumber sesuai, serta endpoint incremental tidak memakai model tanpa `updatedAt`. Ia **tidak** memeriksa data produksi, membuktikan semantics ledger/snapshot, atau menguji route target yang belum ada.

Jangan menjalankan `npm test` di api-service untuk pemeriksaan ini: alur `pretest` yang ada menjalankan seed database. Uji pilot terisolasi tetap mengikuti [`resource-pilot.md`](./resource-pilot.md).

## 9. Sengaja belum dibuat

`write_products`, `write_inventory`, `write_orders`, `write_customers`, `write_fulfillments` memerlukan aturan domain, validasi, concurrency, idempotency dan replay protection tersendiri. Tidak ada CRUD generik yang menulis langsung ke Prisma hanya berdasarkan Merchant ID. Payment capture/refund, shipment booking, tracking normalisasi, customer privacy events, resource webhooks, bulk exports, GraphQL, dan SDK terpublikasi tetap di luar slice ini.
