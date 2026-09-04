# Uji integrasi produk: developer app → App Platform → Emisell

Status: **implemented, gated / nonaktif secara default**. Kode di kedua working tree telah diuji lintas repo memakai Prisma/PostgreSQL terisolasi; ini bukan bukti deployment atau pengumuman general availability. Kontraknya ada di Admin → Documentation → Emisell Resource API. `read_products` tetap `planned` untuk publikasi App Store sampai checklist rilis disetujui. Produk pilot tidak mencakup variants, mutasi, atau webhook produk.

## 1. Bedakan dua URL dan dua kredensial

| Pemanggil → penerima | Endpoint | Kredensial |
| --- | --- | --- |
| Developer app → App Gateway | `GET /v1/products`, `GET /v1/products/{productId}` | Installation token dari OAuth dengan consent `read_products` |
| App Gateway → api-service | `GET /internal/app-platform/v1/products`, `GET /internal/app-platform/v1/products/{productId}` | RS256 assertion baru per request, umur 45 detik atau sisa umur installation token, mana yang lebih pendek |

Provider tidak memilih tenant dengan parameter Merchant ID. Merchant ID diambil dari instalasi aktif. Di api-service, signed claim dan routing header harus cocok, merchant harus aktif/tidak disuspend, lalu setiap query memakai `merchantId`. Akses produk merchant lain menghasilkan `404`. Kunci resource ini **berbeda** dari kredensial merchant-session-grant (arah Backend Emisell → App Platform).

## 2. Persiapan operator

Untuk uji terhadap deployment, gunakan backend/database development yang diizinkan dan Merchant ID uji yang memang ada. Jangan menghubungkan pilot ke database produksi sebagai jalan pintas. Pengujian otomatis di bagian 7 membuat merchant/produk sintetis hanya pada database sementara miliknya sendiri; tidak mengubah database atau mengaktifkan pilot pada service yang sedang berjalan.

Siapkan melalui pengelola secret lokal:

- Sepasang RSA minimal 2048 bit; private key PKCS#1 atau PKCS#8 PEM hanya untuk gateway.
- File JSON public key map untuk api-service, misalnya `{"resource-key-2026-09":"<PUBLIC KEY PEM>"}`. `kid` harus sama dengan key ID gateway. File ini hanya memuat public key, bukan private key.
- Secret acak terpisah minimal 32 byte untuk HMAC cursor; hanya untuk api-service.
- Hak baca file key dibatasi ke proses pemilik. Jangan memasukkan key, token, atau file lokal override ke source control, chat, screenshot, atau Postman shared values.
- Jam kedua service tersinkron; verifier memakai zero clock tolerance dan menolak assertion dengan lifetime di atas 60 detik.

## 3. Konfigurasi di api-service

Module: `src/modules/app-platform/resource.module.js`; dipasang terpisah sebelum logger URL, middleware browser, dan route `/ext/v1`. Tidak memakai SDK secret atau session cookie.

Pasok environment sebelum aplikasi diimpor/start, misalnya melalui environment container atau launcher yang memuat env file privat. Jangan mengandalkan konfigurasi yang baru dimuat setelah import aplikasi.

```dotenv
APP_PLATFORM_RESOURCE_ENABLED=true
APP_PLATFORM_RESOURCE_PUBLIC_KEYS_FILE=/run/secrets/resource-public-keys.json
APP_PLATFORM_RESOURCE_CURSOR_KEY_FILE=/run/secrets/resource-cursor-key
APP_PLATFORM_RESOURCE_ENVIRONMENT=<environment internal instalasi uji>
APP_PLATFORM_RESOURCE_MERCHANT_IDS=<merchant-id-uji-yang-diizinkan>
```

`APP_PLATFORM_RESOURCE_ENVIRONMENT` harus cocok dengan konteks instalasi (`sandbox` atau `production`); ini konfigurasi internal service, bukan pilihan baru pada form developer. Daftar merchant dipisahkan koma, tidak boleh kosong. Tanpa flag, route menghasilkan `503 resource_disabled`. Mengaktifkan flag dengan konfigurasi yang tidak lengkap menghentikan startup; tidak ada fallback secret. Restart service diperlukan setelah perubahan konfigurasi/key.

Query menggunakan Prisma `product.findMany/findFirst`, field projection eksplisit, transaksi repeatable-read pada **primary DB** dengan `statement_timeout` 4 detik, timeout transaksi 5 detik dan max wait 1 detik. Transaksi yang sama memeriksa `Merchant.isActive` dan `Suspension` merchant. Tidak ada schema migration atau mutasi billing/trial otomatis. Request berikutnya melihat perubahan policy yang sudah committed; request yang sedang berjalan memakai snapshot sebelumnya.

## 4. Konfigurasi di App Gateway

```dotenv
EMISELL_RESOURCE_ENABLED=true
EMISELL_RESOURCE_BASE_URL=<origin api-service yang dipercaya, tanpa path>
EMISELL_RESOURCE_KEY_ID=resource-key-2026-09
EMISELL_RESOURCE_PRIVATE_KEY_FILE=/run/secrets/resource-private.pem
```

Gunakan HTTPS di production. HTTP hanya diizinkan bila gateway berjalan dalam mode development. Di Docker Desktop, backend yang berjalan di host bisa dirujuk dengan `http://host.docker.internal:8000` **jika backend benar-benar memakai port tersebut**; sesuaikan port atau service DNS. Jangan memakai `localhost` untuk menunjuk host dari container.

Compose utama meneruskan empat variable gateway di atas. Untuk key, tambahkan mount read-only dalam file override lokal di luar repo, kemudian jalankan Compose dengan kedua file konfigurasi. Contoh bagian override:

```yaml
services:
  app-gateway:
    volumes:
      - /absolute/private/resource-private.pem:/run/secrets/resource-private.pem:ro
```

Mount api-service diatur oleh deployment backend sendiri; file public key dan cursor secret tidak pernah dibutuhkan frontend. Tidak ada tombol dashboard yang menyalakan pilot. Gateway tidak meneruskan cookie, installation token, header tenant dari provider, ataupun client secret ke backend. Redirect upstream tidak diikuti; timeout 5 detik, response maksimum 4 MiB, dan tidak ada retry otomatis.

## 5. Install app uji dan baca produk

Cara paling aman untuk mencoba checkout lokal adalah [lab Product Reader](../examples/product-reader/README.md): dua database sementara, frontend consent terisolasi, dan backend Go contoh developer. Runner tidak memuat `.env` atau memakai database existing.

Untuk test-install `read_products`, gateway development juga memerlukan `EMISELL_RESOURCE_TEST_MERCHANT_IDS=<merchant-id-uji>` dan konteks internal merchant sandbox. Daftar ini hanya membuka eligibility request/consent pilot; backend tetap mempunyai allowlist sendiri. Kedua daftar harus cocok. Default kosong, ditolak di luar `APP_ENV=development`. Bila memakai Compose manual, tambahkan variable gate tersebut melalui override privat (Compose utama hanya meneruskan empat setting resource di bagian 4). HTTP app URL memerlukan opsi lokal terpisah; lihat panduan contoh. Menghapus allowlist undangan bukan pencabutan token; matikan adapter untuk menghentikan read yang sudah terinstal.

1. Konfigurasikan app development dengan required/optional scope `read_products`.
2. Buat test-install request dengan Merchant ID uji yang terdaftar; ikuti [alur instalasi development](./development-test-installation.md), merchant session, consent, state dan PKCE yang sudah ada.
3. Tukar authorization code di backend provider melalui `/oauth/token`. Token harus memuat scope `read_products` yang disetujui.
4. Panggil `/v1/products?limit=10` menggunakan installation token. Jangan menambahkan selector merchant atau environment.
5. Untuk halaman berikutnya, kirim `cursor` dari `meta.nextCursor` dengan filter yang sama; berhenti jika nilainya `null`. `updatedAfter` menerima UTC berakhiran `Z` dengan paling banyak tiga digit pecahan detik. `published` hanya `true` atau `false`. `limit` 1–100, default 50.
6. Coba produk milik merchant lain: harus `404`. Cabut instalasi: token lama harus `401`. Token tanpa scope harus `403`.

Field `price` dan `compareAtPrice` adalah decimal string dari **Product dasar**, bukan harga minimum variant/display storefront; trailing zero tidak dijamin. `stock` adalah nilai sumber `Product.stock`, **bukan janji stok siap jual**, hasil pengurangan sold count, atau penjumlahan stok variant. Mata uang tidak dikembalikan: jangan berasumsi IDR atau memakai slice ini untuk checkout. Resource inventory masih planned. Respons tidak mencakup cost, metadata internal, variants, atau data merchant lain; description dapat mengandung HTML yang harus diamankan ketika dirender. Urutan pagination `updatedAt DESC, id DESC`; concurrent edits antarhalaman bukan snapshot konsisten, sehingga sinkronisasi harus menduplikasi secara aman berdasarkan ID/waktu.

Di Postman, semua secret tetap placeholder. Request Products dan seluruh koleksi Resource memiliki guard `pm.execution.skipRequest()`. Setelah konfigurasi operator selesai, set environment privat `enable_resource_pilot` ke string `true` dan ganti `baseUrl` sesuai pemanggil. Ini hanya mengizinkan request di Postman; tidak menyalakan backend. Jangan menjalankan koleksi internal dengan installation token.

## 6. Error dan operasi

| Respons | Tindakan |
| --- | --- |
| Provider `401` | Token kedaluwarsa/dicabut, app/credential/organisasi atau instalasi tidak aktif; hentikan job |
| Provider `403` | Scope tidak diberikan atau akses instalasi ditolak; jangan mengganti Merchant ID |
| `400 invalid_request` | Periksa query, UTC timestamp, cursor, dan filter; jangan retry request yang sama |
| `404 not_found` | Produk tidak ada dalam tenant token; jangan mencoba ID tenant lain |
| `429 rate_limited` | Hormati `Retry-After` (maksimum 60 detik) dan beri backoff/jitter |
| Provider `503 resource_disabled` | Flag gateway nonaktif |
| Provider `503 resource_unavailable` | Backend nonaktif, signature/key/environment/allowlist salah, merchant hilang/nonaktif/tersuspend, timeout, DB bermasalah, atau response tidak cocok; cocokkan `requestId` dengan audit internal sebelum retry |

Backend hanya mencatat identitas teknis, request ID, route, status, dan metrik; tidak mencatat JWT, cookie, query/cursor, product body, SQL error atau stack. Pembatasan awal 120 request/menit per app + instalasi + merchant + route pada **masing-masing proses** gateway/backend, dengan kapasitas bucket dibatasi. Ini belum limiter terdistribusi.

Rollback: matikan `EMISELL_RESOURCE_ENABLED`, restart gateway, lalu matikan backend bila perlu. Instalasi dan produk tidak dihapus. Rotasi RSA: tambahkan public key baru ke map backend terlebih dahulu, restart backend, ganti key ID/private key gateway, tunggu lebih dari lifetime assertion sebelum menghapus key lama. Rotasi cursor secret membuat cursor lama invalid; mulai pembacaan dari halaman pertama.

## 7. Pengujian dan batas klaim

Tes aman tanpa seed/database produksi (Node dependencies, Go, Docker dan image `postgres:17-alpine` diperlukan):

```sh
# Dari api-service; jangan memakai npm test karena pretest melakukan seed.
node --test tests/unit/app-platform-resource.test.mjs

# Tetap dari api-service. Runner membuat DB sendiri; tidak menerima DATABASE_URL existing.
node scripts/test-app-platform-resources.mjs /absolute/path/to/emisell-app-platform

# Verifikasi Go biasa, dari emisell-app-platform/services/app-gateway
go test -race ./...

# Dari emisell-app-platform
npm run validate:contracts
npm run test:docs
```

Runner membuat container PostgreSQL 17 dengan nama/label unik, port loopback acak, tmpfs, tanpa host volume. Schema Prisma saat ini diterapkan hanya ke DB kosong tersebut, lalu fixture merchant A/B, merchant nonaktif/disuspend, produk, dan variant dibuat. Environment koneksi yang mungkin ada di shell tidak digunakan. Runner tidak memanggil `npm test`, seed repo, atau entrypoint lama. Pada sukses maupun kegagalan biasa, container miliknya dihentikan/dihapus; terminasi paksa dapat memerlukan cleanup manual setelah label pemilik diverifikasi.

Contract test lintas repo menggunakan OAuth/App Gateway Go, HTTP, verifikasi RS256, dan query Prisma/PostgreSQL nyata. **Penyimpanan App Gateway tetap repository in-memory terisolasi**, bukan PostgreSQL gateway. Authorization dibuat melalui tooling development yang sudah ada; ini bukan pengujian UI consent/browser atau SSO produksi. Tes standar Go melewati contract test bila path backend tidak disediakan. Jika path diberikan tanpa database terisolasi yang dipasok runner, test gagal dengan petunjuk; tidak fallback ke data palsu atau DB existing.

### Bukti pengujian lokal

Selain contract test di atas, `npm run example:check -- /absolute/path/to/api-service` dari platform menjalankan provider Go terpisah, **repository gateway PostgreSQL nyata**, dan database resource PostgreSQL kedua. Tesnya mencakup HTTP form + merchant consent API, callback, PKCE/state/replay, penyimpanan token provider terenkripsi, restart gateway/provider, dan uninstall terisolasi. Ini berbeda dari runner lama yang memakai gateway in-memory; test otomatis ini sendiri tidak mengotomasi UI browser atau SSO produksi. Walkthrough browser dan seluruh batas contoh ada di [Product Reader](../examples/product-reader/README.md).

- List/detail merchant A hanya berisi produk A, merchant B hanya berisi B; akses silang kedua arah `404`.
- Token tanpa scope `403`; request tanpa installation token `401`; selector Merchant ID/duplicate query `400`.
- Cursor valid mengikuti urutan timestamp + ID; cursor diubah, dipakai merchant/instalasi lain, atau dengan filter berbeda `400`.
- Harga dasar `60000.50` tetap menjadi `"60000.5"` meskipun variant berharga `12000.25`; stock produk `45` tidak diganti stock variant `3` atau dikurangi sold count `7`.
- Merchant hilang/nonaktif/disuspend ditolak internal; perubahan status merchant di DB langsung berlaku pada read berikutnya.
- Uninstall A membuat tokennya `401` pada list dan detail, sementara token B tetap bekerja.
- Flag default tetap mati; tidak ada private key, activation flag, atau data merchant yang dimasukkan ke deployment lokal existing.

Hasil ini membuktikan boundary kode pada checkout yang diuji, bukan koneksi deployment, review data produksi, performa pada data besar, atau kesiapan semua scope.

Sebelum status publik `available`: uji deployment development yang disetujui dengan data representatif, review query plan/index tenant + updatedAt + id, deployment/trust/key provisioning, agregasi rate limit, audit retention/alerting, projection/data classification, batas response produk besar, dan review keamanan. Schema Product saat ini belum mempunyai index komposit khusus merchant + updatedAt + id; rencana migrasi/index memerlukan evaluasi terpisah, bukan diterapkan ke database existing oleh test. Checklist lengkap ada di [Resource API](./emisell-resource-api.md#definisi-siap-rilis). Order, inventory, customer, fulfillment, dan webhook resource tetap planned.
