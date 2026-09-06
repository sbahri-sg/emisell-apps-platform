# ADR 0024 — Aplikasi provider terpisah, API-Kurir sebagai engine

Status: keputusan produk disetujui pengguna, 6 September 2026. Implementasi awal: lifecycle per-provider teruji lokal; belum rollout produksi.

## Keputusan dan ownership

API-Kurir bukan app agregator untuk merchant. RajaOngkir, KiriminAja dan Mengantar adalah identitas app terpisah yang memakai engine API-Kurir bersama. App Platform mengelola publikasi/review/installation/grant/audit; API-Kurir mengelola credential, provider selection dan operasi shipping. Dashboard Core tetap `/store/[merchantId]/settings/apps`, menggunakan kembali konfigurasi Shipping yang sudah tersedia.

Pemetaan app → `{engine, providerCode}` dimiliki release signed dan snapshot consent immutable. Tidak ditentukan browser, nama display, atau assignment Testing. App Platform tidak membuat credential ID/tenant ID kedua, mengimpor secret atau menulis database API-Kurir.

Sumber kontrak yang diperiksa: `api-kurir/docs/merchant-shipping-providers.md`, `docs/emisell-merchant-gateway.md`, `internal/httpapi/tenant_shipping_providers.go`, dan `internal/merchantproviders/service.go`. Gateway existing memakai key Main Service server-only dengan `X-Emisell-Merchant-ID`. Detail provider sudah tersedia pada `GET /api/v1/integrations/providers/{provider_code}`. Pemeriksaan source tidak membuktikan deployment production memakai versi yang sama.

## Dua jenis state, bukan dua salinan data

- Installation/grant aplikasi adalah milik App Platform. Beberapa provider apps boleh terpasang.
- `installed`, credential readiness per environment, `active_provider_code` dan version pemilihan adalah milik API-Kurir. Maksimal satu provider efektif aktif; tidak ada provider aktif juga valid.
- Grant active bukan perintah memilih provider. Instalasi RajaOngkir yang siap tidak gagal hanya karena KiriminAja sedang dipilih untuk checkout. Sebaliknya, adanya KiriminAja aktif tidak membuktikan credential RajaOngkir tersedia.
- Pergantian provider memerlukan aksi pengguna pada alur existing dengan version check. Booking/tracking shipment lama memakai provider/environment snapshot, bukan pilihan checkout terbaru.

## Increment yang diimplementasikan

1. Manifest local reference menambahkan field opsional `shippingProvider`. Profile baru `local-kurir-provider` mengikat tiga pasangan ID/provider tertutup, digest berbeda per provider, scope tepat `shipping.read` dan deklarasi `shipping/v1`. Tidak ada URL atau secret dalam binding. Public fixture signing key hanya material test, bukan publisher identity/sertifikasi provider.
2. `rajaongkir-provider-reference`, `kiriminaja-provider-reference`, `mengantar-provider-reference` tidak di-seed, dipublikasikan, atau ditampilkan sebagai apps merchant. Dua provider pertama digunakan dalam integration test agar isolasi antaraplikasi terbukti, bukan klaim KiriminAja production sudah siap.
3. Snapshot consent/consumption menyimpan binding; perubahan provider mengubah digest. Field `omitempty` mempertahankan bentuk JSON/hash historis ketika binding tidak ada. Snapshot/release lama tidak ditulis ulang.
4. Lifecycle memakai Consume/Activate/Get/List/Uninstall existing. Readiness adapter hanya membaca detail provider terikat dan memeriksa code, installed, available, serta non-built-in. Tidak membuat credential, memilih provider, membaca nilai key, atau mengganti auth API-Kurir.
5. Profile ini lifecycle-only. Capabilities yang dideklarasikan pada manifest/intent tetap `shipping/v1`, tetapi installation routing kosong. Validasi repository Get/List memakai routed capabilities yang diturunkan dari policy profile, tidak memalsukan data routing. Grant aplikasi dapat aktif tanpa mengambil alih resolver checkout; issuance token ditolak sampai engine delegation tersedia. Profile lama tetap mempertahankan semantics lama.
6. Runtime test tetap origin IP loopback, dummy key, marker fixture, timeout, response/concurrency limit dan no proxy/redirect. Komposisi normal tidak menyuntikkan readiness provider, sehingga Activate gagal tertutup. Tidak ada environment flag yang membuka runtime produksi.
7. Uninstall mencabut akses aplikasi terpilih; tidak memanggil upstream. Tetap berhasil ketika engine outage, tidak memengaruhi grant app lain, idempotent, dan audit sekali. Reinstall wajib ID/consent baru; receipt lama tidak menghidupkan replacement.

Tidak ada frontend baru, perubahan schema SQL, auto-migration, pembukaan scope resource Plan, perubahan gateway checkout/credential, atau koneksi production. Fixture agregator ADR 0023 dipertahankan hanya untuk compatibility; bukan model produk baru.

## Gate sebelum menghubungkan UI dan merchant nyata

Urutan implementasi berikutnya:

1. Rilis app RajaOngkir terverifikasi dengan binding engine/provider menggunakan trust root produksi. Pisahkan publikasi metadata, approval instalasi, dan runtime eligibility; jangan mempromosikan fixture ke app production.
2. Kontrak otorisasi engine untuk merchant + app + installation + operasi, dengan grant freshness/revocation dan server-owned provider mapping. Integrasikan **semua jalur lama**, termasuk ongkir/fulfillment, agar permintaan yang ditolak tidak fallback ke akses langsung. Key Core full-access tidak menjadi credential developer/provider. Mekanisme auth API-Kurir existing dipertahankan; identitas service baru harus diprovision dan diaudit eksplisit.
3. Tutup race antara pengecekan grant/provider dan operasi engine. Provider/version yang diotorisasi harus dicocokkan pada saat engine memilih credential/menjalankan operasi; GET readiness diikuti POST ongkir tanpa binding atomik bukan solusi produksi.
4. Hubungkan halaman Apps existing ke instalasi per-provider, buka form konfigurasi existing bila dibutuhkan, dan gunakan credential yang sudah tersimpan tanpa meminta ulang. UI membaca ulang hasil authoritative; bukan sukses berdasarkan query navigasi. App yang terpasang dan provider yang dipakai checkout memiliki label terpisah.
5. Migrasi satu merchant pilot atas persetujuan; existing provider `installed=true` tidak otomatis menjadi consent App Platform. Jangan membuat grant/backfill diam-diam. Pertahankan kontrak lama selama compatibility window, tetapi tidak sebagai bypass untuk merchant yang sudah dimigrasi.
6. Tetapkan policy shipment berjalan saat uninstall, termasuk tracking/webhook/label/cancel yang masih diperlukan. Izin penyelesaian terbatas harus terikat shipment lama, tidak membuka booking baru atau menghidupkan grant yang dicabut.
7. Conformance API-Kurir nyata di environment yang disetujui, lalu rollout bertahap. `environment=sandbox` tidak menjamin ongkir/tracking bebas quota/live read-only; jangan menguji menggunakan credential nyata tanpa persetujuan.

## Verifikasi

Unit tests memeriksa binding/signature/digest, substitusi engine/provider, scope escalation, compatibility manifest/consent lama, tidak berbagi pointer binding mutable, wrong-provider readiness, missing credential, malformed/oversized response dan input yang ditolak sebelum request.

Integration test `TestShippingProviderLifecycle` memakai handler ConnectRPC asli + PostgreSQL disposable dan API-Kurir tiruan: consent wajib, pending, replays, aktivasi tanpa mengganti checkout, dua app terpasang independen, recomposition, merchant/actor isolation, token/invocation/legacy shortcut ditolak, concurrent uninstall saat outage, audit tunggal, reinstall dan receipt lama. Routing checkout fixture lama tetap dapat berjalan bersama provider apps dan tidak berubah setelah uninstall. Fixture HTTP hanya mengizinkan GET detail provider; seluruh mutation upstream menjadi kegagalan test.

Runner dari checkout api-service:

```sh
node scripts/test-app-platform-core-preview.mjs /absolute/path/emisell-app-platform --shipping-sandbox
```

Runner juga menjalankan regression bridge agregator ADR 0023 dan Express → Core → Platform. Database/merchant/key sintetis terisolasi dan dibersihkan sesudah test. UI merchant dan API-Kurir production tidak dipanggil. Tanpa DSN, menjalankan Go integration test langsung akan skip; skip bukan bukti lulus.

Hasil verifikasi 6 September 2026:

- `go test -race ./... -count=1` lulus untuk test tanpa dependensi database; test yang membutuhkan DSN skip pada perintah ini.
- Runner disposable `--shipping-sandbox` lulus: `TestKurirCoreE2E`, `TestKurirReferenceConsentRatesAndRevocation`, `TestShippingProviderLifecycle` benar-benar berjalan dengan PostgreSQL dan race detector.
- Runner disposable tanpa flag lulus: `TestCorePreviewNodeE2E`, `TestCurrentMerchantInstalledList`, `TestConsentInstallationPolicyExpiryAndRollback`, `TestTestingDistribution`. Container dan data sintetis kedua runner dibersihkan setelah selesai.
- `go build ./...`, `go vet ./...`, pemeriksaan format Go, dan `make contracts` lulus. API docs tetap sesuai sumber: 74 operasi, 18 kontrak.
- Ini bukan verifikasi integrasi provider production, browser merchant, atau jalur ongkir nyata; bagian tersebut belum diaktifkan.

## Rollback

Increment ini tidak diaktifkan pada startup; normal composition tetap tidak dapat mengaktifkan provider fixture. Jika ada data test persistent, revoke melalui lifecycle sebelum melepas binary yang memahami profile baru; jangan menghapus release/receipt/audit untuk menutupi state. Test runner disposable dibersihkan setelah selesai. Untuk rollout produksi kelak, rollback routing harus tetap mempertahankan grant/revocation gate, bukan membuka kembali bypass legacy.
