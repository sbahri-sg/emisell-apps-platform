# ADR 0023 — Penghubung shipping API-Kurir, referensi lokal

Status: diterapkan untuk pengujian lokal, 6 September 2026. Bukan rollout production.

## Konteks

Pengguna menyetujui increment install → konfigurasi siap → cek ongkir → uninstall dengan API-Kurir tiruan dan database terisolasi. Pemeriksaan dokumentasi https://api-kurir.emisell.com/ dan `api-kurir/openapi/public.yaml` menemukan gateway merchant yang sudah mengelola credential, satu provider efektif aktif, ongkir, fulfillment, dan tracking. OpenAPI publik identik dengan checkout lokal saat diperiksa. Keberadaan route/dokumentasi bukan bukti transaksi production lulus.

API-Kurir juga mempunyai Partner Portal/release sendiri. App Platform belum mempunyai runtime umum untuk konfigurasi developer signed. Dilarang menyamakan publikasi metadata, grant merchant, dan readiness provider.

## Keputusan

1. Tambahkan adapter `internal/runtime/kurir` dengan constructor **NewLocalFixture**, bukan provider-specific endpoint Core. Adapter hanya bisa menghubungi IP literal `127.0.0.1`, memakai key dummy konstan dan marker fixture; tidak ada konfigurasi secret production atau DNS/redirect/proxy. Reuse `localhttp` dengan timeout 3 detik, response 32 KiB dan empat request in-flight.
2. Reference `api-kurir-reference` memiliki profile/digest tertutup, versi 1.0.0 dan scope tepat `shipping.read`. Public test signing key hanya untuk fixture ini, bukan artifact publisher. Normal seed, server, public catalog dan Testing **tidak** mendaftarkannya. Test memasang manifest pada database terisolasi lalu memakai composition root `InternalHandlerWithLocalShipping`.
3. Consume/grant/routing/receipt/audit menggunakan aggregate installation yang sudah ada. Snapshot mengikat app, versi, profile, digest dan merchant; tidak perlu tabel binding duplikat atau migration. Aplikasi membungkus akun API-Kurir, bukan memin satu provider. Pemilihan provider tetap konfigurasi operasional API-Kurir. Model per-provider app membutuhkan keputusan/mapping terpisah nanti.
4. Activate membaca `GET /api/v1/integrations/providers`; gagal jika tidak ada tepat satu provider efektif aktif, installed dan available. Status pending/grant pending tetap ketika pemeriksaan gagal. Tidak memasang credential atau memanggil endpoint activate upstream.
5. Setiap GetRates melewati current active grant/installation gate dan readiness sebelum membaca receipt. Upstream hanya dipanggil untuk cache miss idempotency. Key request dan payload sama mengembalikan rate sebelumnya, **bukan quote segar**; gunakan key baru untuk perhitungan ulang. Ini snapshot uji lokal tanpa jaminan harga booking/TTL quotation production.
6. `origin_zone` field 6 ditambahkan pada GetRates. Field opsional untuk simulator lama, wajib bridge. Mapping server-owned zona netral → district ID disalin immutable saat constructor; tidak mengirim provider ID melalui public contract. Mapping fixture kecil tidak dianggap master lokasi production.
7. `POST /api/v1/calculate/district/domestic-cost` mengirim form `origin`, `destination`, `weight`, `include_group=true`. Tidak mengirim credential ID, provider choice, service key Core atau session/actor ke upstream. Header merchant berasal dari Core principal yang sudah terverifikasi; key upstream adalah dummy fixture. Response memerlukan meta success, data valid dan `canonical_service`; service dinormalisasi sebagai `courier_code:canonical_service`, rupiah integer IDR. Tidak fallback ke kode service mentah.
8. Profile tidak mengizinkan Create/Track atau operasi lain. `shipping.read` tidak mengaktifkan Shopify `read_shipping` atau resource scope Plan. COD/komponen nominal fulfillment tetap konsep pengiriman, bukan payment gateway App Platform.
9. Uninstall mencabut grant/token/routing atomik dengan lock existing. Tidak ada provisioning remote pada profile ini, sehingga tidak perlu remote cleanup mutation: credential dan provider pilihan tidak diubah. Shipment/worker/webhook nyata tidak dipakai. Policy menyelesaikan shipment berjalan setelah uninstall wajib ditetapkan sebelum shipping write production.

## Error dan keamanan

- Input/zona tidak dikenal: invalid_request; tidak ada request ongkir upstream.
- Tidak ada konfigurasi/credential siap atau gateway 409/422: conflict; tidak menganggap instalasi siap.
- Gateway 401/403/429/5xx, timeout, redirect, marker hilang, malformed/oversized response: unavailable; body/secret upstream tidak diteruskan.
- Full-access key Core tidak melewati grant, scope maupun readiness. Legacy install endpoint menolak profile ini.
- Uninstall menunggu operasi yang sudah memegang lifecycle lock; setelah revocation tidak ada invocation baru ke gateway. Tidak menjanjikan membatalkan request yang sudah dikirim.
- Hash request memuat installation ID dan OriginZone. OriginZone kosong `omitempty` mempertahankan hash/payload lama. Receipt uninstall/consume lama tidak menyentuh replacement setelah reinstall.
- Normal server tanpa adapter gagal tertutup pada aktivasi/invocation, tidak menggunakan simulator sebagai fallback. Profile lain yang tidak dikenal juga gagal tertutup.

## Verifikasi dan cara menjalankan ulang

Gunakan PostgreSQL disposable khusus bernama database `emisell_local_test`, host loopback; jangan gunakan database aktif pengguna. Set `EMISELL_TEST_DATABASE_URL` melalui environment privat, lalu:

```sh
go test -race ./internal/runtime/kurir ./pkg/appmanifest -count=1
go test -race ./internal/bootstrap -run '^TestKurirReferenceConsentRatesAndRevocation$' -count=1 -v
make contracts
go vet ./...
go build ./...
```

Tanpa test DSN, integration test di-skip; skip bukan bukti lifecycle lulus. Test membuat merchant/key/manifest sintetis, memanggil Core ConnectRPC asli pada httptest server, dan memeriksa daftar installation persistent, consent, grant, retry, readiness, isolation, concurrency uninstall, reinstall, audit dan fail-closed composition. API-Kurir tiruan hanya melayani dua operasi di atas; endpoint setup/mutasi provider tidak dipanggil. Unit test memeriksa payload, canonicalization, egress, response batas, timeout/cancellation, concurrency dan error.

Hasil verifikasi 6 September 2026: targeted lifecycle test dan `go test -race ./... -count=1` lulus dengan PostgreSQL disposable + NATS test binary lokal; `make contracts`, Go vet/build, generator consistency, 33 test dashboard, lint, typecheck dan build dashboard lulus. Coverage statement adapter baru 95,4%. Tidak dilakukan browser UI test, pemanggilan API-Kurir production atau transaksi provider; hasil ini tidak membuktikan kesiapan production.

## Rollout dan batas

Tidak ada data pengguna atau schema yang dimigrasi, tidak ada restart/deploy. Kontrak Protobuf additive dan generated docs/SDK diperbarui. Reference tidak muncul otomatis di Dashboard Core pengguna; bukti Installed Apps pada increment ini berasal dari RPC integration test dengan database disposable, bukan perubahan UI/store live.

Untuk production: lengkapi mapping lokasi dan profile executable tersertifikasi, service-to-service credentials/delegation, fresh grant enforcement yang tidak bisa dilewati jalur lama, operation/scopes coverage, credential readiness per environment, callback ownership/replay dan policy shipment aktif. Uji conformance dengan API-Kurir sebenarnya dalam environment yang disetujui sebelum mengalihkan satu merchant. Jangan mengubah label `simulation` atau membuka installability hanya karena test ini lulus. Booking/pickup/label/cancel/tracking adalah increment berikutnya dengan kontrak quote fulfillment terpisah.

Rollback lokal: gunakan normal composition tanpa adapter; grant yang masih ada tidak dapat invoke. Jangan menghapus reference release sebelum installation terkait dicabut secara sah; hilangnya release dapat membuat registry fail closed. Production rollout belum diizinkan oleh ADR ini.
