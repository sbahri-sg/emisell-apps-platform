# Paket app dan tagihan terpusat Emisell

## Status implementasi

**Fondasi sudah diimplementasikan dan diuji; penagihan live belum diaktifkan.**

- App Platform: paket Free/Paid bulanan, quote prorata, persetujuan merchant, langganan, rincian biaya, invoice app, callback pembayaran idempotent, serta endpoint pengecekan akses fitur Paid.
- Developer Console: **Apps → pilih app → Plans & pricing**. Harga berasal dari API/database; tidak ada paket atau nominal contoh yang dimasukkan ke database pengguna.
- Merchant: **Connected apps → Plan & billing**. Harga dan persetujuan terpisah dari OAuth/install consent. Instalasi sendiri tidak membuat tagihan.
- `api-service`: konektor RS256 dan adapter penggabungan invoice/pencocokan pembayaran tersedia di `src/modules/app-platform/billing.client.js` dan `billing.invoice.js`.
- **Belum disambungkan otomatis ke cron renewal, semua jalur checkout, email/PDF invoice, atau callback provider yang lama.** Ini sengaja tidak mengubah penagihan merchant yang sedang berjalan. Pajak app harus diberikan oleh kebijakan server billing Emisell; bukan diasumsikan nol.
- Tidak ada pembayaran nyata, email invoice, refund, settlement developer, atau perubahan Payment/Shipping Gateway yang dijalankan oleh pengujian.

`APP_BILLING_ENABLED=false` dan `APP_BILLING_LIVE_ENABLED=false` adalah default. Docker development mengunci live billing ke `false`. Migration `000017_app_billing` disediakan; pada implementasi awal ini hanya diterapkan di PostgreSQL disposable milik pengujian, bukan otomatis ke database development yang sedang dipakai.

## Pembagian tanggung jawab

| Pemilik | Tanggung jawab |
| --- | --- |
| App Platform | Paket/harga app, bukti persetujuan, langganan per instalasi merchant, rincian biaya unik, snapshot bagian app dari invoice, masa akses fitur berbayar |
| Billing Emisell | Paket utama merchant, kalender billing, invoice gabungan, pajak/diskon yang berlaku, checkout/payment provider, verifikasi pembayaran penuh, penyampaian invoice, refund |
| Backend developer app | Memeriksa entitlement Paid sebelum memberi fitur berbayar, tetap mematuhi scope dan lifecycle instalasi |

Credential extension tidak masuk invoice atau JWT billing. Merchant ID tetap berasal dari identitas Emisell yang terverifikasi. ID saja bukan otorisasi untuk menyetujui biaya atau menyatakan invoice sudah dibayar.

## Batas versi pertama

- Satu langganan berjalan per instalasi app. Satu app dapat menawarkan beberapa paket.
- `interval=free` harus `amountMinor=0`; `interval=monthly` harus positif.
- IDR menggunakan rupiah utuh; USD menggunakan sen. Semua nominal integer. Tidak ada konversi kurs otomatis.
- Harga/fitur paket tidak bisa diedit. Buat paket baru untuk harga baru; archive paket lama hanya menutup pendaftaran baru, bukan menaikkan harga pelanggan lama.
- Tagihan **prepaid**: Free langsung aktif. Paid menunggu konfirmasi pembayaran; approval bukan bukti bayar. Tagihan awal harus diproses Emisell sebelum fitur Paid dapat digunakan.
- Quote menampilkan biaya awal prorata, periode layanan, harga bulanan berikutnya, pajak terpisah, dan berakhir maksimal 10 menit kemudian. Pembayaran terlambat tidak memperpanjang periode layanan yang sudah disetujui.
- Belum ada trial, usage billing, upgrade/downgrade tengah periode, prorated refund, diskon app, revenue sharing, atau payout developer. Merchant membatalkan paket lama dan menunggu masa prepaid berakhir sebelum memilih paket baru; tagihan lama yang belum lunas harus diselesaikan.
- API Resource scopes tetap terpisah dari hak fitur Paid. `paidAccess=true` tidak memberi scope baru. App eksternal harus menerapkan pengecekan fitur sendiri; module ini tidak bisa otomatis mengubah kode provider.

## Renewal dan aturan pembatalan

1. Emisell mengirim kalender app billing bulanan untuk merchant. Interval harus sedang berjalan, 27–32 hari, dalam detik utuh. Periode berikutnya dimulai persis di akhir periode sebelumnya; mata uang tidak dapat ditukar diam-diam.
2. Saat merchant memilih paket, App Platform menghitung `ceil(harga × sisa_detik / total_detik_periode)`, menyimpan quote, lalu meminta persetujuan.
3. Approval atomik menyimpan subscription dan satu charge awal. Request ulang dengan quote yang sama tidak membuat subscription/biaya kedua. Jangan otomatis menyetujui quote dari backend.
4. Sebelum Emisell membuat payment request, ambil snapshot rincian app dengan **ID invoice Emisell yang stabil**. Biaya app ini digabungkan ke invoice yang sesuai. Tagihan pertama dapat membutuhkan invoice app tersendiri jika paket utama sudah dibayar; renewal berikutnya dapat digabung dengan paket Emisell ketika jatuh tempo.
5. Pada renewal bulanan, invoice endpoint membuat biaya satu periode berikutnya berdasarkan **harga yang pernah disetujui**, bukan harga katalog sekarang. Subscription awal yang belum dibayar tidak terus membuat utang bulanan baru. Suspend instalasi bukan pembatalan langganan.
6. Setelah pembayaran seluruh invoice diverifikasi dan disimpan Emisell, kirim status bagian app ke App Platform. `failed` boleh menjadi `paid`; `paid` tidak boleh kembali `failed`. Retry callback memakai `eventId` yang sama; payload berbeda dengan ID sama ditolak.

Paket utama tahunan **bukan** alasan menunda tagihan app bulanan setahun. Billing Emisell harus menyediakan kalender app bulanan dan invoice ketika jatuh tempo. Periode tahunan ditolak endpoint sinkronisasi bulanan ini. Pilihan app tahunan belum tersedia.

| Aksi/status | Renewal berikutnya | Akses fitur Paid |
| --- | --- | --- |
| Approval Paid, belum bayar | Tidak menumpuk siklus utang baru | Belum tersedia |
| Invoice paid | Berjalan sesuai persetujuan | Sampai `paidThrough`, selama instalasi/organisasi diizinkan |
| Pembayaran gagal | Retry invoice yang sama; jangan buat pengganti untuk charge yang sama | Tidak menambah masa akses; masa prepaid sebelumnya tetap berlaku |
| Deactivate/suspend instalasi | Tidak otomatis berhenti | Diblokir selama instalasi nonaktif |
| Cancel subscription | Berhenti; charge belum diinvoiskan dibatalkan | Masa prepaid yang sudah dibayar tetap berlaku |
| Uninstall | Berhenti secara atomik dengan uninstall | Diblokir; callback terlambat tidak mengaktifkan ulang |

Invoice yang sudah terbit tetap tercatat dan dapat ditagih setelah cancellation/uninstall. Tidak ada refund otomatis. Jangan menghapus/membuat ulang invoice terbit untuk menangani retry.

## API berdasarkan pemanggil

Seluruh endpoint mengikuti envelope `{ "data": ... }`, error standar gateway, `Cache-Control: no-store`, dan tidak menerima query parameter. JSON field yang tidak dikenal ditolak.

### Developer organisasi

- `GET /v1/apps/{appId}/plans`: `app.read`, daftar paket termasuk archived.
- `POST /v1/apps/{appId}/plans`: `app.manage` (owner/admin), wajib `Idempotency-Key` 16–128 karakter. Key mengikat payload; key yang sama dengan harga berbeda menghasilkan 409.
- `DELETE /v1/apps/{appId}/plans/{planId}`: archive, owner/admin, idempotent.

Contoh body pembuatan paket **ilustratif**, bukan data live:

```json
{
  "name": "Reviews Pro",
  "description": "Additional review management features",
  "interval": "monthly",
  "currency": "IDR",
  "amountMinor": 100000,
  "features": ["Review moderation", "Custom branding"]
}
```

### Browser merchant

Menggunakan cookie merchant HttpOnly yang sudah ada. Semua mutasi memerlukan `X-CSRF-Token`. Jangan mengirim `merchantId`, user, harga, status bayar, atau environment dari browser.

- `GET /v1/merchant/installations/{installationId}/billing`: paket tersedia, langganan terakhir, `paidAccess`, `paidBillingAvailable`, dan penanda `test`.
- `POST .../billing/quotes`: `{ "planId": "<plan-uuid>" }`.
- `POST .../billing/approve`: `{ "quoteId": "<reviewed-quote-uuid>", "acceptRecurringCharge": true }` **setelah merchant menyetujui harga yang tampil**.
- `DELETE .../billing/subscriptions/{subscriptionId}`: membatalkan renewal, bukan uninstall.

### Backend Emisell

Gunakan RS256 assertion berumur pendek dengan konfigurasi issuer/audience/key Emisell Backend yang sudah tersedia. Claims minimal: `iss`, `aud`, `sub`, `iat`, `exp` (maksimal 5 menit), `jti`, `store_id`, `environment`, `permissions: ["apps.billing.write"]`. Konektor Node menggunakan 60 detik. Token `apps.install`, token developer, dan token instalasi tidak boleh menulis billing. Request yang membawa `Cookie` atau `Origin` ditolak.

- `PUT /v1/integrations/emisell/billing/account`
- `POST /v1/integrations/emisell/billing/invoices`
- `POST /v1/integrations/emisell/billing/invoices/{invoiceId}/payment`

Sinkronisasi kalender (tanggal di bawah hanya ilustrasi; gunakan periode merchant yang sedang berlaku):

```json
{
  "currency": "IDR",
  "cycleStart": "2026-09-01T00:00:00Z",
  "cycleEnd": "2026-10-01T00:00:00Z",
  "enabled": true,
  "revision": 0
}
```

Ambil rincian invoice sebelum payment request:

```json
{
  "invoiceId": "<stable-emisell-bill-id>",
  "currency": "IDR",
  "cycleStart": "2026-09-01T00:00:00Z",
  "cycleEnd": "2026-10-01T00:00:00Z"
}
```

Respons berisi `lines`, ID charge unik, app/plan/subscription/installation ID, deskripsi app + paket, periode, currency dan `amountMinor` total **app saja**. Tidak mengandung harga paket utama, pajak, atau credential. Lines membeku saat issued, walaupun status invoice kemudian paid.

Konfirmasi setelah pembayaran penuh Emisell berhasil diverifikasi:

```json
{
  "eventId": "<stable-payment-event-id>",
  "paymentId": "<verified-persisted-payment-reference>",
  "status": "paid",
  "currency": "IDR",
  "amountMinor": 100000
}
```

`amountMinor` di callback harus sama dengan bagian app sebelum pajak, **bukan** seluruh nominal invoice gabungan. Jangan mempercayai `paid` dari redirect checkout atau browser. Backend harus memverifikasi signature/reference/currency/nominal pembayaran provider sesuai jalur yang dipakai.

### Provider developer app

`GET /v1/installation-billing` memakai **installation access token**, tanpa merchant ID/installation ID tambahan. Token hanya dapat membaca instalasinya sendiri. Cookie/Origin ditolak. Periksa `paidAccess`, paket yang berlaku, dan fitur paket sebelum mengizinkan fitur berbayar. Saat response gagal, jangan menganggap merchant sudah bayar.

Token OAuth sendiri tetap memakai aturan expiry/revocation dan scope yang sudah ada. Endpoint billing tidak memperpanjang token dan tidak mengubah resource permissions.

## Database App Platform

Migration `000017_app_billing` menambah tujuh tabel:

- `app_plans`: paket immutable dan key/hash permintaan pembuatan.
- `merchant_app_billing`: kalender, currency, status enabled dan revision per merchant/konteks internal.
- `app_billing_quotes`: snapshot harga dan periode yang ditampilkan sebelum persetujuan.
- `app_subscriptions`: bukti persetujuan, paket yang disetujui, status, paid-through, dan pembatalan.
- `app_subscription_charges`: biaya unik per subscription/periode dan keterikatan ke invoice.
- `app_billing_invoices`: snapshot bagian app dari invoice Emisell.
- `app_billing_events`: bukti mutasi billing/pembayaran serta deduplikasi event.

Snapshot menggunakan JSONB; foreign key dan unique indexes mengikat merchant, instalasi, quote, subscription, periode serta invoice. Transaksi memegang lifecycle locks hingga commit. Trigger uninstall membatalkan langganan di transaksi uninstall yang sama. Riwayat tagihan tidak dihapus. Audit failure membatalkan perubahan status dan entitlement pembayaran.

Data dimuat per merchant pada implementasi awal. Sebelum rollout volume besar, tambahkan paging/retensi quote kadaluarsa dan query ledger terbatas; jangan mengaktifkan untuk seluruh merchant sekaligus.

## Integrasi `api-service` dan gerbang aktivasi live

Panduan kode spesifik ada di `api-service/src/modules/app-platform/BILLING.md`.

Hal penting hasil inspeksi repo:

- `MerchantBilling.grossTotal` adalah nominal invoice sebelum fee provider; `netTotal`/`cashReceived` adalah hasil settlement dan `feeTotal` adalah fee pihak ketiga. Biaya app tidak ditambahkan ke kolom settlement tersebut.
- Adapter menyimpan snapshot app di `MerchantBilling.metadata.appPlatformBilling` dan memperbarui `grossTotal` dengan optimistic concurrency, tanpa menghapus metadata lain.
- `src/cron/index.js` memanggil `billing-v2.js`. Ada beberapa jalur checkout/callback, serta kode yang menghapus atau membuat ulang pending bill. Jalur-jalur ini **belum diubah** oleh fondasi ini.
- Sebelum live: tetapkan kebijakan pajak app dan format invoice, sambungkan prepare sebelum payment request, pertahankan ID/snapshot saat retry, tangani invoice awal app, jadwalkan rekonsiliasi pembayaran dari DB, dan uji seluruh jalur yang akan diaktifkan.
- Caller harus menserialisasi persiapan invoice dan pembuatan payment request. CAS adapter mencegah overwrite DB, tetapi tidak dapat menghentikan kode lama yang sudah mengirim payment request secara paralel. Jangan mengaktifkan live sebelum semua jalur terkait menghormati batas ini.
- Backend wajib menyimpan dan mencoba ulang rekonsiliasi sampai acknowledgement. Konektor menyediakan stable event ID dan penanda `statusSync`, **bukan** daemon/outbox worker yang sudah berjalan.
- Billing Emisell harus menyediakan kalender app bulanan, termasuk merchant dengan paket utama tahunan; jangan menebak dari `endCycle` karena field tersebut dapat mencakup grace pembayaran gagal.

Aktifkan di lingkungan terisolasi terlebih dahulu. Konektor meminta allowlist merchant eksplisit dan menolak mencampur invoice test/live. Mode test hanya dapat ditempelkan ke draft dengan `metadata.appBillingTest=true`; production tidak menerima penanda itu. Assertion/signing key harus berada di secret manager/server, tidak di frontend atau Postman shared values.

Tidak menjalankan down migration pada ledger berisi tagihan. Rollback aplikasi dilakukan dengan mematikan pembuatan langganan baru/adapter sesuai prosedur operasi dan mempertahankan database; rekonsiliasi invoice yang sudah terbit tetap harus diselesaikan melalui deployment yang mendukungnya.

## Verifikasi aman

```sh
# Dalam repo App Platform; membuat PostgreSQL baru di tmpfs lalu membersihkannya.
npm run test:billing
npm run backend:check
npm run validate:contracts
npm run test:docs
npm run typecheck
npm run lint
npm run build

# Dalam repo api-service; jangan gunakan npm test karena pretest menjalankan seed.
node --test tests/unit/app-platform-billing.test.mjs
```

Pengujian mencakup prorata integer, consent, pemisahan role/session/service assertion, CSRF, tenant isolation, concurrent replay, snapshot durability, penolakan amount/currency salah, callback urutan terbalik, gagal audit, Free tanpa charge, cancellation/uninstall, serta penolakan live di development.

Referensi OpenAPI ada dalam kontrak **Developer Management**, **Emisell Backend Integration**, dan **Provider Runtime** di Admin Documentation. Generator `sync-billing-contracts.mjs` menjaga schema dan endpoint sinkron. Koleksi Postman melewati endpoint billing kecuali environment privat menetapkan `enable_app_billing=true`; jalankan mutasi satu per satu setelah review, bukan seluruh koleksi.
