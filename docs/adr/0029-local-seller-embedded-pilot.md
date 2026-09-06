# ADR 0029 — Pilot embedded lokal di Dashboard seller

Status: development lokal saja; bukan onboarding aplikasi publik.

## Keputusan

Demo `127.0.0.1:4321/seller` ditampilkan di Dashboard seller lama `localhost:3000`. Port 4320 tetap harness sintetis, bukan sesi merchant.

Platform mengelola consent, consume, activate, receipt dan uninstall. App `embedded-local-demo`, versi `1.0.0`, URL serta client `embedded-demo-client` tetap; scope/capability kosong. File privat `.local/embedded-pilot.json` memuat `environment=development` dan `merchantId` enrollment. Production ditolak. Ini bukan release katalog atau approval review publik; jangan gunakan policy ini untuk provider/public app.

Core membutuhkan `APP_PLATFORM_CORE_EMBEDDED_PILOT_ENV=development` dan `APP_PLATFORM_CORE_EMBEDDED_PILOT_MERCHANT_ID`, selain konfigurasi preview/install lokal. Merchant/staf berasal dari authorizer sesi Core, bukan input browser. Tidak ada tenant ID tambahan.

## Alur keamanan

1. Seller memilih Install sendiri pada halaman existing. Tidak ada auto-consent.
2. Setelah active, Open app membuka detail `?open=1` dengan iframe sandbox berbeda origin.
3. Parent meminta sesi melalui Core BFF dengan cookie, Origin tepat dan X-Emisell-Preview: 1.
4. Platform GetInstallation memberi `local_embedded_access=true` hanya setelah pemeriksaan enrollment, owner, release, active installation/grant. Field ephemeral ini default false, bukan grant resource. Receipt/uninstall tetap tersedia ketika pilot dinonaktifkan.
5. Core menerbitkan identitas JWT Ed25519 60 detik. Bridge memvalidasi origin/window/nonce; token hanya di memori, bukan URL, cookie atau localStorage.
6. Backend demo memanggil introspeksi loopback Core dengan bearer saja, tidak membawa cookie seller. Core memverifikasi signature, sesi staf, digest dan akses Platform terkini. Outage atau pencabutan menolak akses.

Cache Core dibatasi 500 sesi, TTL 60 detik. Cookie authorizer hanya ditahan sementara di memori Core dan tidak dikirim ke app. Key ephemeral; restart membatalkan token. Ini kompromi pilot, bukan persistence/SSO production. Demo renewal 45 detik dan pemeriksaan ulang 5 detik.

API-Kurir, tarif, service shipping dan order tidak berubah. Demo tidak mendapat resource access token.

## Rollback dan migration path

Matikan enrollment Platform dan konfigurasi pilot Core, lalu restart keduanya. Jangan menghapus receipt/instalasi untuk rollback. Salah satu policy nonaktif memblokir issuance/introspeksi berikutnya; uninstall tetap melalui lifecycle normal.

Production harus memakai registry/review launch dan client resmi, key rotation/verifier standar, operasional sesi/audit/rate limit production, dan verifikasi end-to-end. Pilot tidak mengklaim jalur public app sudah tersambung.

## Pengujian

Database terisolasi: consent wajib, idempotency, activate, owner isolation, enrollment disable, uninstall/reinstall, larangan resource token dan list installed. Core: expired/tampered token, revoked session/grant, digest berubah, salah merchant/app dan browser introspection ditolak. Bridge: exact origin/window, replay dan navigation race.

6 September 2026: seller menekan Install sendiri. Browser menunjukkan installed Active, Open app di Dashboard existing, identitas sesi nyata Terhubung, dan renewal berhasil. Uninstall/revocation merchant nyata tidak dijalankan; jalur negatif diuji terisolasi. Test Core 64 dan Dashboard 54 lulus; Go race/build/vet serta Buf lint/breaking lulus. Typecheck seluruh Dashboard masih gagal pada modul lama Domains/editor di luar scope; tidak ada diagnostic pada modul Apps yang diubah. Mobile belum diverifikasi.
