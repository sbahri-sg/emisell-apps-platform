# Integrasi Core — installation lifecycle lokal

Status: lifecycle backend **executable fixture lokal**, gratis. Katalog developer tetap metadata `installable:false`, semua scope resource gateway Plan. Tidak ada halaman merchant di platform. Lihat ADR 0016 dan `docs/core-install-intents.md`.

Referensi API-Kurir opt-in ADR 0023 memakai lifecycle yang sama pada database pengujian terisolasi: `api-kurir-reference`, scope `shipping.read`, hanya GetRates dengan `originZone` dan `destinationZone` netral. Aktivasi/readiness tidak mengubah konfigurasi provider. Normal server dan katalog developer tidak otomatis mendaftarkan reference ini. Baca `docs/adr/0023-local-kurir-shipping-bridge.md` untuk batas dan verifikasi.

## Urutan integrasi

1. Core memverifikasi staf/merchant/izin mengelola apps dan CSRF. `Prepare` versi fixture yang tepat; tampilkan snapshot permission. Policy baru `installationPolicy=local-reviewed-fixture/v1` harus ada.
2. Persetujuan eksplisit → `Decide`. Penolakan/expired tidak boleh dilanjutkan. `executionAllowed:false` tetap benar: consent record sendiri bukan otorisasi.
3. Dalam TTL 10 menit, `client.Installations.Consume` dengan merchant, actor, intent, digest dan idempotency key. Installation menjadi **pending**, grant pending tanpa granted scopes. Consume ulang key yang sama mengembalikan installation yang sama/status terkini; key baru untuk intent yang sudah dipakai ditolak.
4. `Activate` setelah setup. Registry diverifikasi lagi. Simulator payment/shipping siap dari fixture lokal; remote memerlukan connection OAuth dan handshake existing. OAuth lokal tersebut belum merupakan UI/flow Core production. Kegagalan tidak boleh diubah menjadi sukses di frontend.
5. `IssueToken` hanya setelah active. Simpan secret hanya di backend; audience `emisell.app-platform.local/installation-access`, TTL 15 menit, tanpa refresh. Ini token self-check lokal, bukan key Core atau token data produk. Distribusi/exchange app production belum tersedia.
6. `Uninstall` dengan installation ID: grant/token/routing dicabut bersama. Simulator langsung uninstalled; remote disabling sampai cleanup terkonfirmasi. Baca `GetInstallation` untuk status terkini; kegagalan remote tidak mempertahankan akses lokal.

## Kontrak

Service `/emisell.installation.v1.InstallationService/` di port 8088. Hanya **key platform full-access**, tanpa Origin/Cookie. Consume membawa `merchantId` dan `coreActorId` di root; empat operasi detail/mutation lain memakai objek `target` berisi binding tersebut dan `installationId`/`idempotencyKey`. Response lifecycle memakai objek `result` berisi `installation`, `replayed` dan metadata token bila relevan. Consumer lifecycle harus sama dengan pembuat intent. Key legacy tidak memperoleh privilege execution otomatis.

Pengecualian read-only: `ListInstallations` menerima `merchantId`, `coreActorId`, `pageSize` (0/default 20, maksimum 20) dan `afterId`; response `{merchantId, installations, nextAfterId}` berisi ringkasan current intent-managed installation milik merchant lintas pemasang/key. Core memverifikasi sesi/izin apps setiap halaman. Tidak ada token, actor atau digest; tidak mengalihkan hak mutation. Uninstalled/receipt lama dan instalasi legacy tidak masuk daftar. Detail boundary, pagination dan rollout: [ADR 0020](adr/0020-core-installed-app-list.md).

| Operasi | Field khusus | Efek |
|---|---|---|
| ListInstallations | merchantId, coreActorId, pageSize, afterId | Ringkasan current installation merchant, read-only |
| Consume | intentId, consentDigest, idempotencyKey | Consume atomik, installation/grant pending |
| GetInstallation | installationId | Baca status, tanpa token secret |
| Activate | installationId, idempotencyKey | Pemeriksaan release/runtime, grant + routing active |
| IssueToken | installationId, idempotencyKey | Token sekali tampil; token sebelumnya dicabut |
| Uninstall | installationId, idempotencyKey | Revocation + cleanup; tanpa biaya |

`already_exists` berarti konflik key/body, state, TTL, snapshot, atau routing; bukan izin melanjutkan dengan API legacy. `not_found` menyembunyikan installation/intent milik merchant/service/actor lain. `permission_denied` meliputi key legacy untuk execution. Error registry/runtime fail closed.

Namespace key mutation lifecycle: merchant + service + actor, lintas operasi. Retry identik mengembalikan state terbaru, bukan active snapshot lama. Jika issuance response hilang, retry tidak mengungkap secret (`appToken` kosong); gunakan key issuance baru setelah reauthorization untuk mengganti token. `tokenInvalid` pada retry menandai token asli sudah expired/revoked. Jangan menganggap respons issuance replay berisi token usable.

Reinstall memerlukan Prepare + consent baru, menghasilkan installation ID baru. Receipt/history lama tidak mengubah replacement. Keputusan consent lama tetap immutable dan dapat ditampilkan expired setelah TTL; gunakan GetInstallation untuk lifecycle yang telah dikonsumsi.

## Self-check app

GET `http://127.0.0.1:8087/api/v1/app/installation-access` dengan bearer app token dan header `X-Emisell-Merchant-ID`, `X-Emisell-App-ID`, `X-Emisell-Installation-ID`. Tidak ada cookie/origin/query string; jangan mengikuti redirect yang dapat mengirim credential ke host lain. Respons allowlisted hanya berisi binding, version/status/grant/scopes, audience, dan `resourceGatewayAllowed:false`.

Pemeriksaan memegang lock lifecycle hingga verifikasi release selesai: uninstall dan pemeriksaan tidak melewati satu sama lain. Token expired, rotated, revoked atau binding salah ditolak. Respons self-check bukan token delegasi untuk panggilan berikutnya—setiap consumer masa depan harus menerapkan current grant check sendiri di gate yang sama.

Dokumentasi Admin: kelompok **Core · ConnectRPC** untuk lifecycle; **Aplikasi · akses lokal** untuk self-check. Tidak ada playground yang mengeksekusi request. Platform key Admin tidak boleh dibagikan kepada developer/merchant/browser.

## Bukti lokal dan batas

`payment/v1` dan `shipping/v1` diuji install→activate→invoke→token→uninstall→reinstall dengan PostgreSQL test terisolasi. Remote fixture memakai OAuth/handshake/revocation dan retry cleanup yang sudah ada. Tidak menyatakan gateway Core, executable developer self-service, app-code review/scan, OAuth app-client production atau token resource tersedia.

Rollout: backup → migration 0012 → binary kompatibel → fresh consent. Existing accounts/keys/installations tidak diubah. Sesudah lifecycle baru dipakai, forward-fix lebih aman; rollback pra-0012 dilarang karena binary lama tidak memahami ownership/grants. Semua persistence dan audit dipertahankan.
