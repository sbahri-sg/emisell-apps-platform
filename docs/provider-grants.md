# Kontrak grant provider API-Kurir v1

Implementasi ini terpisah dari `EngineGrantService` pilot Emisell. Tidak mengubah release/install policy pilot, tidak membuat instalasi RajaOngkir, dan tidak mengaktifkan scope yang masih Plan.

## Aktivasi internal (default off)

Terapkan migration `0022_provider_grant_revision.sql` melalui prosedur migration normal. Set `EMISELL_PROVIDER_GRANT_FILE` ke file JSON absolut privat (0600), berisi:

```json
{"EngineKey":"<64 hex acak, khusus engine>","Apps":{"<app-id-yang-direview>":"rajaongkir"}}
```

Enrollment app/provider adalah konfigurasi operator; tidak menggantikan consent dan grant instalasi. Gunakan kunci terpisah dari API key seller/Core. Endpoint hanya terpasang pada listener RPC internal loopback. Tidak ditambahkan ke proxy dashboard atau domain publik. Deployment server tidak diubah otomatis.

## Request

`POST /internal/provider-grants/v1/check`, JSON, `Authorization: Bearer <EngineKey>`. Cookie/Origin browser ditolak. Body:

```json
{"merchantId":"merchant-id","providerCode":"rajaongkir","appId":"app-id","installationId":"installation-id","operation":"rates.read"}
```

Operasi `binding.read` mengembalikan snapshot aktif/pending/revoked untuk rekonsiliasi oleh engine. `rates.read`, `settings.read`, `tracking.read` memerlukan `shipping.read`; `settings.write`, `shipments.create` memerlukan `shipping.write`. Scope harus ada pada consent release DAN grant saat ini; operasi bisnis memerlukan instalasi aktif. Tidak ada token akses baru diterbitkan.

Respons 200: `merchantId`, `providerCode`, `appId`, `installationId`, `revision` (string desimal agar lossless), `active`, `revoked`, `scopes`. Tidak ada credential, harga, atau URL browser. Identitas berasal dari consumed release dengan engine `api-kurir`, bukan dari klaim browser. 403 = tidak diizinkan/tidak ditemukan; 503 = sumber grant tidak tersedia. Semua respons no-store.

Revision PostgreSQL meningkat ketika grant berubah, termasuk trigger revoke existing saat uninstall. Pengecekan operasi dilakukan ulang, bukan memakai snapshot tersimpan sebagai authorization. Request yang sudah lolos dapat selesai bersamaan dengan pencabutan; endpoint ini bukan transaksi lintas layanan dengan shipment.

## Status integrasi

- Handler terhubung ke server internal secara opt-in; sumber menggunakan tabel instalasi/consent/grant existing.
- Implementasi client fresh-check, repository binding, dan worker API-Kurir disimpan pada branch lokal `archive/app-platform-integration-20260907`; folder utama API-Kurir sudah kembali ke baseline. Worker tidak aktif.
- Endpoint internal `GET /internal/provider-grants/v1/targets?after=INSTALLATION_ID` memerlukan engine key yang sama, mengembalikan maksimum 100 identitas target dan cursor `next`. Target revoked tetap disertakan. Setiap siklus worker memulai scan dari awal; tidak memakai high-watermark sequence yang bisa melewatkan transaksi terlambat commit. Setiap target di-pull ulang melalui binding.read sebelum ditulis.
- Release v2 memakai `managed-kurir-provider/v2`; v1 tetap khusus built-in Emisell. Source instalasi v2 (`ProviderInstallSource`) memerlukan release signed, assignment approved, enrollment operator dan environment yang eksplisit. Policy instalasi `provider-app/v1` tidak memilih route checkout dan tidak menerbitkan token legacy.
- Komposisi server default masih memblokir instalasi v2. Pemeriksaan kesiapan gateway dan grant enforcement harus dipasang sebelum source v2 diaktifkan. Penghubungan credential dan gate shipment belum aktif. App tanpa consumed shipping binding yang sah tetap ditolak.
- Pengujian menggunakan mock HTTP dan PostgreSQL sementara; belum uji install RajaOngkir end-to-end atau deploy production.

Aturan izin operasi memakai [fondasi permission aplikasi](app-permissions.md). Emisell Kurir built-in tidak masuk integrasi ini; kontrak pilot sebelumnya bukan jalur production yang harus diaktifkan.
