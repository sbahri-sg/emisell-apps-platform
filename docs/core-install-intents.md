# Kontrak integrasi Core — grant/install intent v1

Status: kontrak consent record lokal, tidak memberikan akses sendiri. Kelanjutannya tersedia terpisah pada `InstallationService` untuk executable fixture lokal: lihat `docs/core-installation-lifecycle.md` dan ADR 0016. Tidak ada UI merchant baru, executable katalog developer, atau token resource gateway.

## Operasi dan akses

Listener internal lokal `127.0.0.1:8088`, ConnectRPC + Protobuf. Header bearer key platform full-access first-party Core atau service account merchant-bound legacy. SDK Go: `client.InstallIntents`. Key platform tidak terikat merchant, namun setiap operasi tetap terikat satu merchant yang ditegaskan Core; bukan query lintas merchant otomatis.

| RPC | Scope legacy (key platform full access memenuhi semuanya) | Request |
|---|---|---|
| `Prepare` | `apps.install_intents.write` | `merchant_id`, `core_actor_id`, `idempotency_key`, `app_id`, `version` |
| `Get` | `apps.install_intents.read` | `merchant_id`, `core_actor_id`, `intent_id` |
| `Decide` | `apps.install_intents.consent` | `merchant_id`, `core_actor_id`, `idempotency_key`, `intent_id`, `consent_digest`, `decision` |

Path procedure: `/emisell.installation.v1.InstallIntentService/Prepare`, `/Get`, `/Decide` dengan prefix service yang sama. `decision`: `CONSENT_DECISION_CONSENT` atau `CONSENT_DECISION_DENY`; unspecified/nilai lain ditolak.

Field additive `merchant_id` WAJIB untuk key platform, berasal dari otorisasi backend Core, bukan parameter browser yang dipercaya langsung. Tenant harus terdaftar. Untuk key legacy, field boleh kosong (memakai binding credential); merchant berbeda ditolak. Tidak ada scopes pilihan caller, redirect URL, atau `approved:true` pada request. Intent terikat merchant + service ID + actor; mengganti merchant/actor/key tidak memberi akses ke intent yang sama.

`core_actor_id` adalah ID opaque stabil 1–128 karakter `[A-Za-z0-9][A-Za-z0-9_.:@-]*`, bukan email/PII. Core mengambilnya dari sesi server-side. Platform tidak mempunyai sesi staf Core dan tidak dapat memverifikasi klik merchant secara independen. Karena itu scope consent **hanya boleh** diberikan kepada backend Core tepercaya yang menjalankan pemeriksaan otorisasi dan CSRF. Credential tidak boleh dibagikan kepada browser, app pihak ketiga, atau developer.

## Urutan implementasi di Emisell Core

1. Core memverifikasi sesi staf, merchant membership dan izin mengelola apps. App Store hanya boleh mengarahkan ke route Core yang di-allowlist; parameter URL bukan bukti izin.
2. Backend Core memakai key platform dan mengirim merchant terotorisasi melalui `merchant_id` (`merchantId` di ProtoJSON), lalu `Prepare` untuk app/version tepat. Key merchant legacy tetap didukung tanpa field baru. Saat ini hanya executable fixture lokal, misalnya `emisell-pay`/`parcel` versi `1.0.0`; app katalog publik ditolak.
3. Core menampilkan response snapshot: identitas app/developer, versi, scopes dan penjelasan akses yang dikenal. Scope tidak dikenal gagal tertutup sampai mapping disediakan. `consent_digest` bukan secret/token dan tidak boleh direkonstruksi frontend.
4. Pada klik setuju/tolak, Core memeriksa ulang sesi, izin, CSRF, dan intent milik actor. Panggil `Decide` dengan digest snapshot yang benar-benar ditampilkan dan key baru untuk keputusan itu.
5. Tampilkan hasil persetujuan tersimpan. **Jangan memanggil legacy install/activate sebagai kelanjutan atau fallback.** Untuk fixture lokal dan key platform, lanjutkan Consume melalui InstallationService sesuai dokumentasi lifecycle; registry production, resource gateway dan Core OAuth UI belum tersedia.

App Platform tidak otomatis mengarahkan browser ke URL arbitrer dan tidak melakukan network call ke app ketika membuat/memutuskan intent.

## State, expiry, dan retry

- `PENDING`: menunggu keputusan.
- `CONSENTED`: persetujuan tercatat; **bukan** active grant.
- `DENIED`: ditolak, terminal.
- `EXPIRED`: pending/consented sudah mencapai TTL, tidak bisa digunakan kembali. Read memproyeksikan expiry memakai clock DB, tanpa mengubah row atau menulis audit. Denied tetap denied.

TTL 10 menit dari waktu prepare server. Snapshot JSON immutable; manifest digest SHA-256 dihitung atas representasi manifest terverifikasi terurut, bukan digest file asli. Consent digest mengikat owner, ID, release snapshot, created/expiry. Saat consent, registry diverifikasi ulang; perubahan manifest/versi/scopes membuat keputusan gagal. Tolak tidak memerlukan registry tersedia.

Mutation memakai key 16–128 karakter `[A-Za-z0-9_-]`. Namespace key adalah merchant + service ID + actor, lintas operasi. Timeout dapat menyembunyikan hasil commit: retry **key dan body yang sama**. Retry mengembalikan intent sama dengan state terkini, bukan response historis; expiry tidak diperpanjang dan audit tidak diduplikasi. Key sama/body berbeda atau keputusan kedua dengan key baru ditolak. Setelah expired, mulai intent baru hanya setelah Core memeriksa ulang izin dan menampilkan snapshot baru.

`execution_allowed` selalu false. Jangan menganggap `CONSENTED`, ID intent, atau digest sebagai credential untuk API capability. Tidak ada RPC consume pada kontrak ini.

## Error

| Connect code | Arti |
|---|---|
| `unauthenticated` | Bearer tidak sah/expired/revoked; sesi browser bukan credential Core |
| `permission_denied` | Scope service account kurang |
| `not_found` | Intent bukan milik merchant/service/actor, atau app tidak ada di registry executable |
| `invalid_argument` | ID/key/digest/decision tidak valid |
| `already_exists` | Key bentrok, versi/digest berubah, terminal atau expired |
| `unavailable` | Verifikasi/kelayakan release gagal tertutup |

Detail DB/secret tidak dikirim. Origin/Cookie pada listener internal ditolak HTTP 403 sebelum RPC. Instrumentasi memakai log request ID/procedure/code, trace dan metrik RPC; tidak merekam request body, bearer, atau actor sebagai label metrik. Audit di `platform_installation.intent_audit` menyimpan merchant, service, actor, keputusan, digest, timestamp secara atomik bersama mutation.

## Rollout, batas, dan pengujian

Tambahan schema intent melalui `0009_install_intents.sql`; key platform terpisah melalui `0011_platform_full_access_keys.sql` (ADR 0014). Jalankan explicit migration sesudah backup dan sebelum binary server baru; server tidak auto-migrate. Tidak ada backfill installation atau permission. Pertahankan tabel/audit saat rollback; binary lama tidak mendukung key platform sehingga consumer harus dipindah secara eksplisit ke credential yang didukung. Credential Core lama tetap sama dan tidak memperoleh scope baru; CLI `init-core`/`rotate-core` tidak mengaktifkannya otomatis. Rotasi platform key mengubah service ID: intent lama tidak dipindahkan, mulai consent baru setelah Core memeriksa ulang otoritas.

`go test -race ./internal/bootstrap -run 'TestInstallIntent|TestCatalogPublicationBoundary'` dengan `EMISELL_TEST_DATABASE_URL` DB test terisolasi menguji merchant/service/actor isolation, scope, expiry, snapshot mismatch, idempotency/concurrency, audit rollback dan katalog yang tidak installable. `make verify` tetap wajib untuk kontrak/baseline, seluruh race tests, vet dan build. Test tanpa DB yang sesuai akan skipped, bukan terverifikasi.

ADR 0016/migration 0012 menambahkan consume atomik, grant/aktivasi dan token self-check hanya untuk fresh consent fixture lokal; field additive `installationPolicy` mengikat policy tersebut pada snapshot. Record historis tanpa policy tidak dapat dikonsumsi. Executable trust production, OAuth client/exchange, resource gateway, rate/quota, retention, TLS identity dan UI Core terotorisasi tetap tahap lanjutan. Legacy install bukan fallback production.
