# ADR 0002 — Lifecycle persisten di backend lokal

> Koreksi frontend: UI owner/workspace merchant yang diceritakan di bawah telah dihapus melalui ADR 0007. Backend tenant, permission, installation, data, dan tests tetap dipertahankan sebagai integrasi/reference Core, bukan portal admin/developer.

Tanggal: 5 September 2026. Status: diterapkan untuk increment lokal, **bukan production-ready**.

Catatan lanjutan: bagian event/RPC pada ADR ini merekam batas tahap pertama. Tahap kedua menambahkan ConnectRPC, service identity, dan relay/inbox NATS; lihat ADR `0003-core-rpc-and-event-delivery.md` tanpa mengubah keputusan historis di bawah.

## Keputusan dan scope

- Engine baru mengikuti struktur root `cmd`, `internal`, `pkg`, `migrations`, dan `api`. Source gateway lama yang telah dihapus pengguna tidak dipulihkan. Database/container lama tidak dipakai atau dimigrasikan.
- Backend Go 1.26+ adalah modular monolith. Chi hanya adapter HTTP. Registry, identity, installation, capability, dan runtime simulator mempunyai boundary sendiri. Frontend tetap memakai komponen dan desain yang sudah ada.
- Jalur lengkap: login lokal → workspace dari membership server → katalog release → consent → pending → active → uninstall. Data, sesi, grant, hasil idempotency, dan audit disimpan di PostgreSQL melalui pgx.
- Browser memakai proxy same-origin `/api/v1` dari `localhost:4317` ke API loopback `127.0.0.1:8087`. Mode dipilih saat menjalankan `dev:local`, bukan melalui query parameter. Tidak ada fallback otomatis ke data demo ketika backend gagal.
- Mode demo lama tetap tersedia melalui `npm run dev`/build default. Situs hosted yang sudah ada tidak dipublish ulang. Tidak ada perubahan akses Sites atau DNS.

## Authentication dan authorization

Login lokal merupakan bootstrap development, bukan OAuth app provider atau SSO Emisell. CLI membuat password acak dan menyimpan credential operator di `.local/login.json` dengan izin `0600`; direktori diabaikan Git. Database hanya menyimpan hash PBKDF2-SHA256 dengan salt acak dan 600.000 iterasi. Tidak ada password default untuk akun operator.

Session menggunakan token acak 256 bit; hanya hash token disimpan di DB. Cookie HttpOnly, SameSite=Strict, path `/api/v1`, expiry absolut 8 jam. Logout mencabut session server-side. Cookie tanpa `Secure` **hanya** karena HTTP loopback; server menolak binding non-loopback dan origin eksternal. Unsafe request harus ber-Origin sesuai konfigurasi dan ber-Content-Type JSON. Host dibatasi untuk mengurangi risiko DNS rebinding. Login dibatasi 30 percobaan per menit per proses. Ini bukan kontrol login terdistribusi/production.

Setiap use case memeriksa membership server-side melalui port identity. Query `workspace` di UI tidak memberikan hak akses. Resource di luar membership menghasilkan 404 tanpa mengungkap keberadaannya. Role awal hanya owner, dengan native grants per installation; OpenFGA belum diperlukan.

## Release, lifecycle, dan consistency

Empat fixture registry: dua implementation `payment/v1` dan dua `shipping/v1`. Release immutable melalui trigger PostgreSQL. Byte manifest diverifikasi dengan Ed25519, compatibility, checksum artifact simulator, scope, dan capability sebelum installation/resolve.

**Signing key fixture adalah material tes publik**, tidak memberi kepercayaan kepada publisher/artefak eksternal. Profil `local-simulator` wajib; tidak ada upload, remote endpoint, third-party WASM, atau arbitrary process execution. Schema `api/manifest/local-v1.schema.json` mendokumentasikan subset fixture lokal, bukan onboarding publisher lengkap.

Transaksi installation memakai lock per tenant: transition, grants, response idempotency, dan event/audit ditulis atomik. `(tenant, actor, key)` dengan body berbeda ditolak 409. Replay mengembalikan response historis, sehingga dashboard selalu memuat ulang state terkini. Key disimpan tanpa penghapusan otomatis pada increment ini; retention/purge membutuhkan kebijakan berikutnya.

Hanya satu installation aktif untuk tiap capability/tenant. Pergantian dilakukan dengan uninstall app aktif lalu activate app pengganti; terdapat celah availability yang eksplisit. Atomic switch/upgrade tanpa downtime belum diimplementasikan. Reinstall memperoleh ID baru dan tidak mewarisi resource/grant lama.

Uninstall atomik membersihkan scopes dan routing di DB serta mempertahankan tombstone dan audit. Invocation memegang lock lifecycle sampai operasi selesai; uninstall menunggu in-flight operation. Tidak ada resource eksternal yang perlu dibersihkan pada profil simulator. Untuk provider nyata, tambahkan disabling/reconciliation dan cleanup bertahap sebelum mengklaim lifecycle produksi.

Pool capability dipisahkan dari pool lifecycle untuk mencegah deadlock akibat menunggu lease kedua ketika lock masih ditahan. Modul hanya mengakses schema miliknya. Runtime simulator menjalankan aturan request/state melalui port capability; tidak mengetahui Chi atau pgx.

## Reference capability

Kontrak REST versioned berada di `api/openapi/platform.v1.json`.

- `payment/v1`: create, capture, refund penuh, status; state authorized → captured → refunded. Nilai integer `amountMinor`, mata uang IDR; seluruh nominal hanyalah fixture simulasi.
- `shipping/v1`: get_rates, create, track; zona serta tarif adalah fixture, tidak memesan kurir atau menghasilkan label asli.
- Invocation diotorisasi terhadap tenant, installation aktif, release, dan scope. Resource terikat pada installation pembuatnya; mengganti aplikasi tidak otomatis memigrasikan resource tersebut.
- Semua invocation mendukung idempotency key dan audit, termasuk operasi baca. Tidak ada endpoint atau tipe provider-specific di contract.

## Event/RPC/observability

Event envelope versioned mempunyai tenant, actor, subject, correlation, causation, producer, dan waktu. Audit sekaligus outbox berada dalam schema modul pembuatnya. **Belum ada relay NATS**; `published_at` tetap null, bukan tanda event berhasil dikirim. Increment berikutnya dapat menambahkan relay JetStream dengan acknowledgement, deduplication, bounded retry, dan inbox consumer tanpa mengganti transaksi domain.

Antarmodul saat ini berkomunikasi melalui interface in-process. Tidak ada internal network RPC yang memerlukan server ConnectRPC pada increment ini. ConnectRPC + Protobuf + Buf tetap pilihan wajib saat mengekspos internal boundary ke Emisell Core/service lain; jangan menambah RPC hop untuk pemanggilan internal monolith.

`slog` mencatat request ID, route template, status, durasi, dan trace ID tanpa password/token/body. OpenTelemetry membuat span dan menerima W3C trace context; exporter/collector belum dikonfigurasi. `/metrics` menyajikan metrik Prometheus request dan latency dengan label ber-cardinality terbatas. `/readyz` memeriksa koneksi DB; server memverifikasi schema saat startup. Tidak ada klaim collector, alerting, atau dashboard monitoring sudah tersedia.

## Migration dan exit path

1. Gunakan DB baru `emisell_local`, bukan database gateway lama. Jalankan migration embedded melalui CLI secara eksplisit. Checksum diverifikasi dan migration diproteksi advisory lock; server tidak auto-migrate.
2. Validasi increment dengan database terpisah `emisell_local_test`. Pengujian tidak menghapus database atau data development.
3. Untuk rollout produksi: tentukan SSO/identity yang disetujui, HTTPS dan Secure cookie, secret management, deployment/domain sendiri, signed publisher trust root, serta runtime provider nyata. Jangan mengekspos simulator loopback dengan reverse proxy publik.
4. Pindahkan data gateway lama hanya melalui rencana pemetaan dan migrasi yang direview terpisah. Tidak ada import otomatis atau rollback destruktif.
5. Rollback frontend lokal ke mode demo tidak menghapus DB. Untuk menghentikan increment, stop proses/container; pertahankan volume. Forward migration baru diperlukan untuk perubahan schema bersama, bukan mengedit migration yang telah diterapkan.

## Verifikasi

Unit tests untuk password, manifest/signature, state machine, serta konfigurasi loopback. Integration tests dengan PostgreSQL nyata mencakup seluruh reference operations, consent, persistence lintas instance server, login/logout/expiry, tenant isolation, concurrency/idempotency, audit, immutable release, switching, dan revocation. Frontend tests mencakup retry key yang tetap sama dan pembersihan data setelah session expiry. Alur UI diperiksa pada browser desktop/mobile sesuai `codex.md`.

Audit `govulncheck` menemukan empat advisory terjangkau pada toolchain awal Go 1.26.5 (GO-2026-6090, GO-2026-6089, GO-2026-6088, GO-2026-5972). Minimum proyek dinaikkan ke Go 1.26.6 sebelum verifikasi akhir; instalasi Go global tidak diganti. Dependency backend dikunci di `go.mod`/`go.sum`. PostgreSQL lokal memakai patch terbaru dari jalur major 17; schema/data tetap berada pada volume khusus increment ini.
