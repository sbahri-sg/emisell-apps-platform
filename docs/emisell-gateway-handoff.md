# Handoff gateway resource — Tim Backend Emisell

Status: **kontrak untuk implementasi, bukan endpoint live**. Versi `emisell.resource.product/v1`. Tidak ada resource grant aktif. Dokumentasi Admin → kelompok **Gateway Emisell**. Pusat daftar/status izin berada pada **Katalog Scope**, bukan matriks duplikat dalam dokumentasi endpoint.

Katalog dan dokumentasi membaca read model `/access-scopes/verification` yang sama. `operations` memuat kesiapan per procedure, `scopes` kesiapan izin secara utuh; `contractRevision` mencocokkan mapping/kontrak dengan versi dokumentasi, `environment` membedakan konteks laporan. Saat ini hanya inventaris build `local`, bukan probe Core: semuanya Plan. Response gagal atau kontrak berbeda tampil Belum terverifikasi; dokumen generated tidak menjadi fallback status. Lihat ADR 0015.

## 1. Arah integrasi dan batas

App pihak ketiga → external API App Platform → verifikasi token/grant → internal gateway Emisell Core → data merchant.

Dokumen ini mengatur sambungan **Platform → Core**. Gateway menerima ConnectRPC/Protobuf, bukan cookie portal atau token payment/shipping Core. Host gateway harus dikonfigurasi dari allowlist deployment; jangan mengambil URL dari manifest app. Tidak ada route gateway baru pada port Platform 8088. External REST façade untuk produk belum dibuat dan tidak boleh dianggap tersedia.

Sumber yang harus diikuti:

- `api/proto/emisell/resource/product/v1/product.proto`: wire contract dan komentar field.
- `pkg/sdk/gen/emisell/resource/product/v1`: generated Go messages dan client/server Connect.
- `pkg/gatewaycontract`: validator bentuk request/response, mapping dan coverage. Validator **bukan authorizer**.
- `pkg/gatewaycontract/conformance`: suite reusable. Reference handler hanya di `_test.go`, tidak boleh dipakai production.
- `api/gateway/coverage.v1.generated.json`: matriks generated seluruh scope, bukan registry otorisasi/discovery live.

## 2. Scope → operasi

| Scope efektif | Operasi internal yang dikontrakkan | Belum tercakup |
|---|---|---|
| `read_products` | `/emisell.resource.product.v1.ProductService/List`, `/emisell.resource.product.v1.ProductService/Get` | Varian, koleksi, selling plan, inventory, seluruh operasi tulis |
| `write_products` | Dua operasi baca di atas melalui implikasi read | Seluruh operasi tulis; tidak ada Write/Create/Update/Delete yang dapat dipanggil |
| Scope lainnya | Lihat matriks; belum mempunyai mapping operasional | Tetapkan kontrak sebelum menambahkan handler |

Scope tidak sama dengan satu endpoint. Aktifkan hanya operasi yang telah diuji; jangan memberi akses seluruh domain hanya karena string scope cocok. `reference_only` dan `future_reference` adalah bahan evaluasi, bukan kewajiban meniru fitur Shopify. Scope grant profile tetap `shopify-authenticated-2026-09-05`, terpisah dari versi kontrak dan status implementasi.

## 3. Request/response baseline

Kedua RPC adalah unary POST Connect, menggunakan ProtoJSON camelCase atau Protobuf melalui generated client. Field int64 `grantRevision` berupa string desimal di JSON. Unknown enum/status ditolak.

Access context wajib: `merchantId`, `installationId`, `appId` (masing-masing ASCII `[A-Za-z0-9_-]`, 1–100), `scopeProfile` exact, `grantRevision >= 1`, `requestId` ASCII `[A-Za-z0-9_-]` 16–128. Tidak ada field requested/granted scopes yang bisa diisi caller untuk memberi akses sendiri.

`merchantId` adalah ID merchant Emisell yang sama, bukan ID toko baru dari Platform. Caller baru cukup mengirim field ini. Alias wire deprecated pada Protobuf hanya untuk kompatibilitas; bila kedua ejaan dikirim dengan nilai berbeda, tolak `invalid_argument`. Handler harus memvalidasi dengan `ValidateAccess` lalu mengambil identitas resolved melalui `gatewaycontract.MerchantID`, bukan membaca field deprecated langsung. Tidak ada mapping manual atau pemberian grant otomatis; lihat ADR 0019.

Contoh List (identifier fixture saja; **bukan credential/payload yang dapat dipakai pada data live**):

```json
{
  "access": {
    "merchantId": "merchant_test_a",
    "installationId": "installation_test_a",
    "appId": "app_test_a",
    "scopeProfile": "shopify-authenticated-2026-09-05",
    "grantRevision": "1",
    "requestId": "example-request-0001"
  },
  "pageSize": 25,
  "cursor": "",
  "filter": {"status": "PRODUCT_STATUS_ACTIVE", "titlePrefix": "Tas"}
}
```

List mengembalikan `products` dan `nextCursor`. Get menerima `access` yang sama dan `productId`, lalu mengembalikan `product`. Product hanya memuat `id`, `title`, `handle`, `status`, `updatedAt`:

- ID opaque, bukan Shopify GID atau ID numerik yang diasumsikan universal.
- Judul 1–255 byte UTF-8, tidak kosong/whitespace-only dan tanpa control character.
- Handle 1–100 byte, pola `[a-z0-9]+(-[a-z0-9]+)*`.
- Status konkret `DRAFT`, `ACTIVE`, atau `ARCHIVED` dengan prefix enum `PRODUCT_STATUS_`; `UNSPECIFIED` hanya berlaku sebagai filter semua status.
- `updatedAt` Timestamp valid; JSON RFC3339. Jangan menambah customer PII, biaya, inventaris, provider ID, atau payload internal bebas.
- Omitted filter setara filter kosong. `titlePrefix` maksimal 100 byte UTF-8, case-sensitive, tanpa control character; tidak ada SQL/query DSL.
- `pageSize=0` berarti 25; selain itu 1–100. Jangan clamp input yang invalid. Sort ID ascending secara bytewise, query wajib merchant-scoped, termasuk saat Get.
- Dataset boleh berubah antarpanggilan; v1 **tidak menjanjikan snapshot pagination**. Dalam dataset stabil, semua baris harus terbaca tepat sekali. Filter/sort tidak boleh memperluas akses merchant/field.

## 4. Cursor, error, retry

Cursor adalah token opaque/tamper-evident maksimal 2048 byte dengan karakter `[A-Za-z0-9_.-]`. Bind ke merchant + installation + app + profile + grant revision + canonical filter + effective pageSize + posisi terakhir + expiry maksimum 15 menit. Jangan bind ke requestId karena request berikutnya punya correlation baru. Jangan log isi cursor. Cursor kosong menandai halaman terakhir; jangan mengembalikan halaman kosong dengan cursor lanjutan atau cursor yang sama.

| Connect code | Kondisi |
|---|---|
| `unauthenticated` | Credential tidak ada, invalid, expired, atau audience/trust salah |
| `permission_denied` | Scope tidak ada; tuple assertion berbeda dari delegasi; grant stale/revoked; installation suspended; Cookie/Origin browser dikirim |
| `invalid_argument` | Bentuk field salah, profil kontrak tidak dikenal, batas page/filter/cursor invalid, cursor rusak/expired/tidak cocok dengan context/filter |
| `not_found` | Produk tidak ada **atau milik merchant lain**; response tidak mengungkap keberadaannya |
| `unavailable` | Operasi/dependency belum tersedia; tidak mengembalikan fake empty success atau fallback data merchant lain |
| `resource_exhausted` | Batas concurrency/rate/payload; retry mengikuti backoff/hint aman |
| `deadline_exceeded`, `canceled` | Deadline/cancellation dipropagasikan sampai query |
| `internal` | Kegagalan internal tanpa SQL, token, stack, atau payload sensitif |

Urutan minimal: verifikasi caller → validasi shape → cocokkan delegasi + grant/installation mutakhir + operasi supported → query merchant-scoped → validasi response → audit/telemetry. Network internal bukan bypass. Prefix scope saja tidak mengotorisasi routing.

Target deadline total 5 detik, payload response maksimum 1 MiB, page bounded. Reads tidak membutuhkan idempotency key bisnis; retry terbatas maksimal dua kali untuk `unavailable`/transient transport dalam deadline total, exponential backoff+jitter. Jangan retry 4xx, mereset deadline tanpa batas, atau menganggap requestId sebagai key mutation. Retry cursor tidak mengubah grant. Log requestId, method, duration, outcome dan identifier aman; redaksi tokens, cursor, body sensitif. Bukti audit gagal/berhasil dan kebijakan retensi disepakati sebelum aktif.

## 5. Gate keamanan lintas tim — belum diimplementasikan

Tim Platform dan Core harus menyepakati transport identity/trust, credential audience/issuer/expiry/key rotation, cara mengikat assertion pada delegasi, dan mekanisme memperoleh state grant terbaru. Jangan memakai string bearer statis, body context, header merchant dari internet, token capability lama, atau katalog signing key sebagai solusi production.

Platform bertanggung jawab atas token app, consent, grant lifecycle, installation state, adapter, allowlist tujuan, timeout dan audit. Core bertanggung jawab atas verifikasi caller/delegation, enforcement tuple+scope+resource+field, freshness/revocation, operasi yang tersedia per merchant, query dan response. Core tidak membaca tabel database Platform langsung. Jika state grant/support tidak dapat dipastikan, fail closed. Keberhasilan suite proyeksi tidak membuka gate ini.

## 6. Menjalankan contract tests pada backend Core

Pakai versi module/revision yang disepakati melalui module internal atau `go.work` lokal, **jangan mengasumsikan module sudah dipublish publik**. Generated handler/client memakai ConnectRPC yang sudah dipilih; tidak perlu framework baru.

Di integration test repository Core, buat fixture khusus dua merchant dan panggil:

```go
conformance.Run(t, conformance.Suite{
    Client: clientForTestIdentity,
    AccessA: accessA, AccessB: accessB,
    ProductsA: sortedProductsA,
    ProductBID: productB.ID,
    MissingID: "product_missing_fixture",
})
```

Import `emisell.app/platform/pkg/gatewaycontract/conformance`. `Client` harus mengembalikan generated `ProductServiceClient` untuk setiap `conformance.Identity`: ReadA, ReadB, WriteA, NoScopeA, Invalid, Absent, RevokedA, StaleA, SuspendedA. Siapkan credential dan **state backend sungguhan** per case; jangan mengembalikan error buatan dari client. `ProductsA` adalah seluruh dataset merchant A yang stabil, >=3 produk, >=2 status, sorted, cocok dengan response. Tenant B mempunyai produk terpisah. Jalankan hanya pada environment test; jangan mencetak token.

Suite menguji read/implied write, penolakan credential/scope/state, browser header, context binding, foreign-resource masking, shape limits, filter, pagination, cursor tampering dan context binding. Suite ini belum menguji implementasi crypto, rotasi key, transisi revoke secara concurrent, expiry clock, deadline/query cancellation, audit durability, rate/payload limits atau snapshot drift. Tambahkan pengujian khusus tersebut di Core dan uji end-to-end consent/consume/token di Platform sebelum aktivasi.

Di repository App Platform:

```sh
make generate
make contracts
go test -race ./pkg/gatewaycontract/...
go run ./cmd/cli gateway-contract
# Regenerasi matriks dan dokumentasi portal:
npm run docs:generate --prefix web/dashboard
npm test --prefix web/dashboard
```

## 7. Melengkapi scope planned

1. Pilih scope/relevansi bisnis pada matriks. Jangan mengubah arti profil immutable.
2. Tambahkan kontrak per resource/operasi, request/response minimal, pagination/filter, errors, authorization/field policy dan test. Tidak ada generic arbitrary method/passthrough query.
3. Update mapping `pkg/gatewaycontract` dengan procedure dari generated Connect constants. Contract coverage bisa partial; **implementation tetap planned sampai ada bukti**.
4. Implementasikan Core; jalankan suite di environment terisolasi; catat owner, revision, tanggal, target environment, hasil security/contract tests dan review bersama.
5. Status pelacakan berikutnya: planned → in_progress → conformance_passed → available. Perubahan status harus disertai bukti dan code review; dokumentasi ini belum menyediakan toggle runtime. Tidak semua scope harus menjadi available.
6. Platform menambah adapter + support registry terverifikasi, lalu consent/consume/grant/token pipeline. Baru uji rollout per merchant. Required unsupported menggagalkan install; optional unsupported tidak diberikan. Jangan otomatis menaikkan consent record lama menjadi active grant.

Tidak ada perubahan database, credential, akun, instalasi, atau biaya pada milestone handoff. Lihat ADR 0011 dan 0012 untuk compatibility dan migration path.
