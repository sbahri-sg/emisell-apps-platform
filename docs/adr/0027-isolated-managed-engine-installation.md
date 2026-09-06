# ADR 0027 — Instalasi managed shipping dengan engine lokal terisolasi

Tanggal: 6 September 2026. Memperluas ADR 0026; bukan migrasi API-Kurir produksi.

## Keputusan dan batas authority

Pengguna memilih API-Kurir lokal terisolasi untuk melanjutkan Emisell Kurir. Binding tetap `api-kurir/emisell`; tidak memakai Biteship sebagai identitas app. Binary `api-kurir/apps/local-managed` memakai handler, provider selection, merchantshipping policy, rates service dan PostgreSQL repository engine existing. Tidak memuat `.env` engine produksi, provider/fallback adapters, worker, credential endpoint, fulfillment, pickup atau tracking. Data tarif sample ditandai `local_sample`; tidak melakukan panggilan/saldo provider nyata.

Managed source di module app diverifikasi lewat port pada composition root. Tidak ada injeksi release managed ke fixture registry. Consent snapshot versioned `managed-shipping-local/v1` berisi `managedSource` immutable (release ID, assignment ID, merchant, environment), digest dan binding. Fields baru optional sehingga serialized snapshot fixture lama tetap sama. Tidak diperlukan migration Platform tambahan; migration 0016/0017 tetap immutable.

Prepare/consent/consume/activate membaca approved assignment dan signed release saat ini, memverifikasi signature dan hash, dengan urutan lock release → assignment → lifecycle merchant. Gate memakai pool terpisah. Replay yang sudah tersimpan membaca state terkini; replay bukan aktivasi baru dan tidak memperpanjang TTL. Uninstall selalu bisa dilakukan saat source/engine unavailable. Satu binding engine/provider managed aktif per merchant; provider app tidak mengisi legacy capability resolver. IssueToken aplikasi tidak berlaku untuk engine.

## Engine grant

`emisell.engine.v1.EngineGrantService/Check` adalah ConnectRPC internal loopback dengan key engine terpisah. Full Core key, session browser dan developer/app token tidak diterima. Request mengikat merchant, provider, operation dan environment `local-isolated`; current assignment/signature serta installation/grant `shipping.read` diverifikasi kembali. `rates.read/settings.read` bukan perluasan scope ke booking/tracking/payment.

Engine tidak cache keputusan grant. HTTP settings diproteksi autentikasi service → merchant terverifikasi → current grant. Rates service memeriksa current provider dan grant **sebelum** singleflight/cache untuk tiap caller. Tidak ada fail-open atau fallback produksi. Otorisasi dilinearisasikan terhadap uninstall lokal; operasi engine berbatas waktu yang sudah diperbolehkan bisa selesai ketika revocation sedang berlangsung. Pengecekan baru setelah revocation/suspend ditolak. Snapshot tarif tidak menjadi bukti grant.

Pengaturan provider adalah aksi staf Core yang sudah diverifikasi permission shipping, bukan authority aplikasi developer. Install hanya memberi izin: pemilihan provider dan service masih memerlukan aksi pengguna. Uninstall memutus akses; tidak menghapus konfigurasi pilihan provider yang sebelumnya dibuat pengguna. Reinstall dapat memakai konfigurasi tersimpan, tetapi tetap perlu consent dan grant baru.

## Core dan UI

Flag `APP_PLATFORM_CORE_MANAGED_SHIPPING_ENABLED=true` hanya boleh dalam konfigurasi Core preview lokal dengan instalasi enabled. Core/Dashboard menerima app ID asli dan profil managed dengan scope exact; tetap menolak mapping ke fixture, provider URL dari browser, scope tambahan atau status tidak konsisten. Merchant berasal dari verified session, bukan request.

`EMISELL_LOCAL_MANAGED_ENGINE_FILE` menunjuk file operator privat berisi engine loopback dan enrollment MerchantIDs. Merchant lain mempertahankan rute yang ada. Merchant enrolled tidak boleh fallback produksi saat local grant/engine gagal. Endpoint order/checkout server-side lama belum dipindahkan; jangan melaporkan pilot ini sebagai rollout checkout produksi.

Settings → Apps menampilkan tombol tinjau izin pada assignment yang siap. Consent menggunakan tombol Install existing. Instalasi aktif muncul di Installed Apps dan Additional shipping methods existing. Open app membuka pengaturan provider shipping, bukan URL runtime arbitrer. Tidak ada `/sandbox` baru. Nama app/data milik merchant lain tidak boleh dibawa saat context switch.

## Operasional dan rollback

File `.local/managed-engine.json` mode 0600 adalah konfigurasi operator lokal; tidak boleh dikomit/disalin ke developer/browser. Fields: EngineURL, PlatformURL, DatabaseURL, ServiceKey, EngineKey, MerchantIDs. Engine hanya menerima PostgreSQL loopback bernama `api_kurir_managed_local`. Storage Docker lokal terpisah dari Platform/Core/produksi.

Start engine melalui `go run ./apps/local-managed -config <absolute-private-file>` dari checkout API-Kurir. Platform membaca file privat pada startup; restart setelah perubahan. Key engine berbeda dari key service engine dan full platform Core key. Stop engine atau revoke/suspend source untuk menghentikan pilot; jangan menghapus enrollment agar error tidak berubah menjadi fallback produksi. Pertahankan intent/grant/audit dan konfigurasi untuk forward-fix. Rollback binary lama setelah data managed tercipta bukan jalur aman tanpa maintenance gate dan migrasi eksplisit.

## Verifikasi

Runner `api-service/scripts/test-app-platform-core-preview.mjs <platform-path> --managed-shipping` membuat satu container disposable dengan tiga DB terpisah. Memakai binary API-Kurir nyata, signed release/approved assignment sintetis, consent, consumption, readiness, active grant, current installed list, manual provider/service selection, sample rate calculation, suspend/uninstall deny dan replay. Core Express/session/Prisma regresi tetap berjalan. Unit checks mencakup managed DTO, merchant/policy/scope mismatch, independent engine authentication dan authorization sebelum cache. Tidak menggunakan cookie merchant nyata atau consent pada toko pengguna.

Tarif produksi, checkout/order migration, RajaOngkir credential dan browser visual QA belum termasuk verifikasi ini.
