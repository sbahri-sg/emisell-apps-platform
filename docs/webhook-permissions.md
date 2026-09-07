# Webhook dan izin API

## Status penerimaan perubahan

### Demo HTTPS lokal gabungan

Jalankan `make test-resource-https-demo` dengan `EMISELL_TEST_DATABASE_URL` menuju
database loopback `emisell_local_test`. Demo membuat receiver TLS ephemeral dengan
sertifikat tes yang diverifikasi, memakai rilis demo bertanda tangan Ed25519,
consent dan grant lifecycle asli yang tersimpan di PostgreSQL, lalu producer dan
dispatcher sintetis. Tidak ada halaman browser atau server demo yang ditinggalkan
berjalan setelah tes selesai.

Skenario: signature rusak/client demo nonaktif ditolak, consume sebelum consent
ditolak, transaksi producer rollback tidak meninggalkan event, ID event duplikat
tidak menggandakan job dan payload berbeda ditolak, respons pertama 503 kemudian
retry berhasil 204, job delivered tidak dikirim lagi, serta job pending menjadi
cancelled tanpa panggilan receiver setelah uninstall.

Tabel product/job demo bersifat temporary dan dihapus ketika koneksi ditutup.
Fixture akun/instalasi sintetis mengikuti lifecycle database test existing.
Pemetaan URL konfigurasi demo ke receiver loopback hanya ada di harness tes.
Transport produksi `SendHTTPS` tetap menolak loopback; TLS demo tidak memakai
`InsecureSkipVerify`. Retry dipicu deterministik oleh tes, bukan scheduler produksi.

Ini membuktikan komposisi lokal, **bukan** koneksi event mutation api-service atau
worker universal produksi. Source status client demo masih enrollment sintetis;
source release/app-client produksi dan outbox Core masih harus dihubungkan.

| Bagian | Bukti yang tersedia | Batas |
| --- | --- | --- |
| Konfigurasi versi dan snapshot | Tes validasi, immutability, source hash | Bukan executable release |
| Consent/resource grant | Tes lifecycle dengan PostgreSQL terisolasi | Source/verifier khusus tes |
| Pemeriksaan akses | Tes owner, required/optional scope, release drift, revoke | Belum route token resource publik |
| HTTPS sender | Receiver TLS tes, signature, pin DNS, blok IP internal/redirect | Belum dipasang ke worker universal |
| Worker lama | Tes lifecycle remote dan pembatalan saat scope dicabut | Tetap reference/legacy; tidak mengirim topic resource umum |
| Producer Core dan fan-out universal | Belum ada bukti end-to-end | Jangan aktifkan di produksi |

Kriteria selesai produksi: source rilis resource bertanda tangan dan app-client
terverifikasi dipasang ke lifecycle, producer Core terautentikasi dengan kontrak
payload minimal serta outbox transaksional, binding per merchant, queue durable
dan retry terbatas, lalu tes staging install/consent/event/delivery/revoke.
Jangan mengganti verifier dengan always-true atau mengubah scope Plan menjadi
Active untuk melewati batas ini. Pengecekan scope saja bukan validasi producer.

Ulangi pengujian terarah melalui `make test-remote-lifecycle` dengan
`EMISELL_TEST_DATABASE_URL` menunjuk database loopback `emisell_local_test` dan
`EMISELL_NATS_SERVER` menunjuk executable NATS lokal. Jangan memakai database
merchant/produksi untuk suite ini. Pengujian TLS tidak membutuhkan koneksi ke
endpoint developer sebenarnya.

Suite penuh `go test -race ./...` juga mencakup tes managed shipping lama yang
mewajibkan `MANAGED_ENGINE_TEST_BINARY` dan `MANAGED_ENGINE_TEST_DATABASE_URL`.
Gunakan runner engine terisolasi sesuai ADR 0027; jangan mengarahkannya ke engine
atau database merchant aktif. Tanpa keduanya, `TestManagedShippingAssignments`
gagal dengan pesan `managed assignment integration requires isolated engine runner`.
Kegagalan prasyarat ini bukan bukti suite penuh lolos, walaupun tes webhook dan
resource lifecycle terarah telah berhasil.

## Aturan bersama

Webhook tidak memberikan izin baru. Kebutuhan scope harus ditentukan oleh
kontrak event milik server, bukan oleh browser, payload event, atau developer.
`internal/webhook.AllowsSubscription` memakai pemeriksa yang sama dengan
`internal/apppermission.Allows`: identitas merchant/app/installation harus sama,
grant aktif dan tidak dicabut, serta setiap scope dibutuhkan harus ada pada
consent dan grant terkini. Tidak ada wildcard atau konversi scope implisit.

Subscription juga harus aktif, berasal dari konfigurasi/rilis yang sah, serta
cocok topic dan versi rilis. Flag internal `Approved` adalah otorisasi sumber
konfigurasi, bukan review admin terpisah untuk setiap subscription toko.
Panggil pemeriksaan di dalam lifecycle gate sebelum enqueue DAN sebelum setiap
delivery/retry. Persetujuan subscription tidak menggantikan validasi producer,
kontrak payload, endpoint, signature release, atau autentikasi caller API.

## Yang aktif dalam perubahan ini

Worker reference lokal memeriksa subscription signed manifest, binding
tenant/installation/event, versi rilis, dan scope instalasi saat enqueue maupun
delivery. Bila scope/subscription tidak lagi valid, delivery menjadi cancelled
dengan alasan audit `subscription_or_scope_revoked`. Tidak ada permintaan ke
receiver dan tidak memakai budget retry untuk pengiriman yang dilarang.

Scope legacy fixture tetap terpisah dari scope resource `read_*`/`write_*`.
Pemeriksa universal tidak secara otomatis mengaktifkan scope di katalog API.

## Model utama: konfigurasi webhook pada versi aplikasi

Mengikuti model app-specific yang direkomendasikan
[Shopify](https://shopify.dev/docs/apps/build/webhooks/subscribe), deklarasi webhook
menjadi bagian versi aplikasi, bukan antrean review per subscription. Shop-specific
melalui token API merchant adalah model terpisah dan belum tersedia di Emisell.

Tambahkan field berikut ke `document` pada API buat/perbarui draft aplikasi:

```json
{
  "webhooks": {
    "apiVersion": "2026-09",
    "subscriptions": [
      {
        "topics": ["products.created"],
        "uri": "https://hooks.your-domain.com/events"
      }
    ]
  }
}
```

Ini potongan `document`, bukan keseluruhan body draft. Deklarasikan
`read_products` melalui `accessScopes` juga. `apiVersion` adalah versi kontrak
payload Emisell (masih usulan/non-deliverable), bukan semver aplikasi dan bukan
versi Shopify. Topic Emisell tetap memakai nama sendiri; tidak kompatibel API 1:1.

Konfigurasi ikut snapshot review aplikasi yang immutable. Hash sumber dalam
metadata katalog bertanda tangan mengikat konfigurasi ini tanpa mengekspos URL
receiver ke katalog publik. Mengedit draft tidak mengubah snapshot atau rilis
sebelumnya. Format katalog v1/v2 tetap kompatibel dan `installable: false`,
bukan manifest executable atau grant data.

Gunakan `subscriptions: []` untuk menyatakan konfigurasi kosong di versi baru.
Masing-masing pasangan topic/URI harus unik; endpoint berbeda untuk topic sama
diizinkan. Batas: 20 entri, 50 pasangan topic/URI. Tidak ada HTTP call saat menyimpan.

Target runtime: konfigurasi versi yang dirilis berlaku pada instalasi terkait,
tetapi hanya dikirim jika scope topic tersedia pada grant dan consent merchant
terkini. Scope optional yang baru dideklarasikan tidak cukup. Uninstall/revoke
harus menghentikan event bisnis yang belum terkirim. Event lifecycle/compliance
memerlukan kebijakan tersendiri, bukan bypass grant untuk semua topic.

**Yang sudah berjalan di kode:** validasi draft, snapshot review, signature metadata.
**Yang belum:** aktivasi konfigurasi menjadi routing merchant dan pengiriman event
universal. Approval/release metadata saja tidak mengaktifkan pengiriman.

## API pengajuan subscription awal (kompatibilitas)

API di bawah dipertahankan sebagai authoring pending eksperimental. Bukan jalur
utama app-specific, bukan API shop-specific Shopify, dan tidak otomatis dimigrasikan
atau diaktifkan. Jangan tambahkan tahap approval per subscription pada alur utama.

Endpoint berikut memakai sesi developer portal dan pemeriksaan Origin yang sama
dengan API draft aplikasi. Ini API authoring, bukan API token instalasi merchant.

| Metode | Path di `/api/v1/developer` | Fungsi |
| --- | --- | --- |
| GET | `/webhook-topics` | Topic usulan dan scope wajib; `deliveryReady: false` |
| POST | `/apps/{appId}/webhook-subscriptions` | Mengajukan endpoint/topic untuk revisi draft |
| GET | `/apps/{appId}/webhook-subscriptions?afterId=...` | Maksimum 50 item dan `nextAfterId` |
| POST | `/apps/{appId}/webhook-subscriptions/{id}/revoke` | Mencabut pengajuan dengan body `{}` |

Body pendaftaran contoh (header `Idempotency-Key` wajib, 8–128 karakter huruf,
angka, underscore atau tanda minus):

```json
{
  "draftRevision": 1,
  "version": "1.0.0",
  "topic": "products.created",
  "endpoint": "https://hooks.your-domain.com/events"
}
```

Draft harus milik organisasi pemanggil dan mendeklarasikan `read_products`
di `accessScopes.required` atau `accessScopes.optional` pada contoh tersebut.
Deklarasi optional bukan consent merchant. Scope dan readiness katalog tidak
diaktifkan oleh operasi ini. Topic usulan tersedia untuk created/updated pada
products, orders, dan fulfillments; ini belum menjanjikan producer event aktif.

Respons `subscription` berisi `id`, `appId`, `draftRevision`, `version`, `topic`,
`requiredScope`, `endpoint`, `status`, `createdAt`, dan `deliveryEnabled: false`.
Status awal `pending`, setelah dicabut `revoked`. Replay key dengan body sama
mengembalikan pengajuan yang sama, termasuk bila sudah dicabut; body berbeda
menghasilkan 409. Satu topic per versi aplikasi hanya memiliki satu pengajuan
pending. Pencabutan berulang tidak menduplikasi audit.

400 berarti input/topic/endpoint tidak valid; 403 scope tidak dideklarasikan;
404 aplikasi/subscription tidak ditemukan atau milik organisasi lain;
409 revisi/versi tidak cocok, topic pending duplikat, atau konflik idempotency.

Migrasi `0023` hanya menambahkan tabel pengajuan dan audit. Endpoint harus HTTPS,
tanpa credential, query, fragment, IP literal, atau hostname lokal. Pendaftaran
tidak menghubungi endpoint. Ini **bukan** verifikasi DNS maupun perlindungan
SSRF pengiriman: transport mendatang tetap wajib memvalidasi/resolusi dan pin IP
publik, melarang redirect, serta memverifikasi kepemilikan endpoint.

## Belum diimplementasikan

Transport `internal/webhook.SendHTTPS` sudah tersedia dan diuji dengan receiver
TLS lokal: validasi sertifikat/hostname, timeout total 5 detik, request maksimum
1 MiB, signature protokol Emisell yang sudah ada, serta hasil delivered/pending/dead.
Transport melakukan resolusi DNS per percobaan, menolak IP private/special-purpose
termasuk jawaban DNS campuran, kemudian mengunci koneksi ke IP tervalidasi. Proxy,
redirect, dan reuse koneksi dimatikan. Status 2xx delivered; 3xx dead; error jaringan,
429 dan kegagalan receiver lain pending untuk kebijakan retry caller.

**Transport belum dipasang ke worker universal.** Memanggilnya tidak memberi izin
merchant. Caller wajib memegang lifecycle gate, memverifikasi rilis/instalasi/grant
terkini dan mengambil endpoint serta secret dari sumber terpercaya. Jangan
mengambil target/secret dari payload producer. Header/signature mengikuti Emisell,
bukan protokol wire Shopify. Tidak ada pengecualian localhost pada fungsi publik;
pengujian memakai resolver/dialer internal dengan TLS yang tetap diverifikasi.

Lifecycle kini memiliki profile opt-in `resource-app/v1`, dengan binding immutable
release ID, app-client, deklarasi scope dan konfigurasi webhook. Jalur existing
prepare → consent → consume → activate menyimpan grant menggunakan tabel instalasi
yang sama. Consume menghasilkan status pending tanpa grant; activate hanya memberi
required scopes. Optional scopes belum memiliki alur permintaan consent tambahan.
UI umum tetap tanpa scope bisnis dan profile shipping tidak menerima binding resource.

`WithResourceAccess` memeriksa Core principal/owner, source release terkini dan grant
di dalam lifecycle lock sebelum callback. Token fixture tidak diterbitkan untuk
profile resource. Uninstall tetap dapat dilakukan saat source/verifier tidak tersedia.

**Batas implementasi:** lifecycle baru diuji menggunakan verifier dan release source
khusus tes dengan database nyata. Bootstrap produksi belum memasang `ResourceReadiness`
atau source executable resource; tanpa keduanya grant tidak dapat diaktifkan. Verifier
produksi harus memeriksa signature, app-client, dukungan scope dan runtime; bukan flag
`true` dari browser. Katalog/review webhook sendiri bukan sumber grant executable.
Belum ada route publik resource-token, producer event atau worker universal terpasang.

- Binding konfigurasi versi rilis ke instalasi/consent merchant.
- Kontrak serta producer event order/shipment dari api-service/API-Kurir.
- Fan-out per installation, signature delivery berbasis secret aplikasi,
  deduplikasi/retry, dan transport HTTPS aman.
- Manifest rilis bisnis universal; rilis UI v1 tetap tidak meminta scope bisnis.
- Deployment worker universal dan uji staging receiver aplikasi sebenarnya.

Jangan mengganti status scope menjadi Active sebelum adapter API, grant pipeline,
dan operasi terkait diverifikasi. Jangan memperluas consumer NATS ke semua topic
sebelum validasi producer, schema payload dan routing merchant selesai.

Emisell Kurir built-in tidak diubah dan tidak diwajibkan memasang aplikasi.
