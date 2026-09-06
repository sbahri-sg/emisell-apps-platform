# ADR 0004 — Remote app lokal, OAuth, dan signed webhook

> Koreksi frontend: alur dashboard merchant pada ADR ini bersifat historis dan UI-nya telah dihapus. Backend/OAuth reference dipertahankan; integrasi consent final mengikuti Core dan tiga surface pada ADR 0007.

Tanggal: 5 September 2026. Status: diterapkan untuk development lokal. Memperluas ADR 0002/0003; bukan akses provider atau deployment production.

## Keputusan dan boundary

Tambahkan satu release immutable `remote-pay@1.0.0`, capability `payment/v1`, execution profile `local-remote`. Empat release simulator lama tidak diubah. Internal RPC/Protobuf/SDK Core v1 tetap sama; resolver memilih adapter berdasarkan manifest yang diverifikasi. `shipping/v1` tetap simulator. Proses `cmd/remote-reference` terpisah adalah contoh aplikasi konsumen, bukan ekstraksi engine menjadi microservices.

Public wire contract berada di `pkg/appapi`; alias internal mempertahankan JSON capability lama. Installation, registry, dan capability domain tidak mengenal endpoint provider. OAuth/remote HTTP adalah adapter protokol lokal; tidak diteruskan sebagai tipe SDK/provider ke domain atau Core. Repository OAuth dan webhook dimiliki masing-masing modul. SQL hanya mengakses schema owner: `platform_oauth`, `platform_webhook`, atau `reference_remote`. Aplikasi contoh tidak membaca tabel platform. Pemakaian fungsi simulator oleh reference app hanya untuk fixture bisnis tanpa efek finansial.

Manifest remote menyatakan subscription `emisell.capability.invoked.v1`. Endpoint runtime dipetakan dari konfigurasi operator privat, bukan URL arbitrary dari manifest/browser. Signature fixture Ed25519 tetap memakai **test trust root publik**; artifact digest adalah identitas profile contoh, bukan bukti integrity binary provider production. Tidak ada upload third-party.

## OAuth dan keamanan lokal

- Platform bertindak sebagai OAuth client; reference app sebagai provider **contoh lokal**, bukan sistem login provider production. Consent menyebutkan semua akses adalah simulasi.
- Authorization Code + S256 PKCE, confidential client Basic auth, exact redirect URI, state acak TTL lima menit, code sekali pakai. State diikat ke user, hash session browser, tenant, dan installation. Callback mensyaratkan session yang memulai alur dan membership yang masih berlaku. Wrong-session tidak mengonsumsi state yang sah; replay/expiry ditolak.
- Consent POST memerlukan exact Origin serta nonce form. `Referrer-Policy: same-origin` mempertahankan Origin form lokal tetapi tidak membawa URL authorization ke callback berbeda origin. CSP membatasi form ke origin reference app dan origin callback yang dipin; tidak memakai wildcard.
- Token access berlaku lima menit; refresh family delapan jam. Refresh berotasi; penggunaan ulang token lama mencabut family. Token hash-only di provider, credential client AES-256-GCM dengan nonce acak dan AAD tenant/installation/purpose. Webhook secret berbeda per grant dan encrypted at rest. Key client dan reference app berbeda, masing-masing file privat `0600` dalam `.local/` berizin `0700` dan gitignored.
- Callback menukar code server-to-server. Token, verifier, client secret, dan signing secret tidak dikirim ke frontend, localStorage, audit, log, atau event. Callback redirect membersihkan parameter sensitif. Query authorization hanya memuat state dan PKCE challenge, bukan verifier/token.
- OAuth token rotation, capability call, webhook send, dan uninstall memakai gate lifecycle yang sama. Pool lifecycle, capability, koneksi, dan antrean worker dipisah untuk menghindari nested connection starvation.
- Egress hanya HTTP ke IP literal `127.0.0.1` dan port yang dipin; browser consent menggunakan `localhost` agar cookie SameSite Strict tetap bekerja. Tidak menggunakan environment proxy/DNS publik, tidak mengikuti redirect, deadline tiga detik, payload maksimal 32 KiB. Batas HTTP tanpa TLS **hanya untuk loopback development**.
- Refresh gagal/ambigu ditandai perlu koneksi ulang. Jangan menganggap rotasi refresh dapat diulang dengan token lama setelah respons hilang. Reconnect mencabut family sebelumnya. Key file tidak dirotasi diam-diam; key hilang berarti koneksi perlu dipulihkan lewat proses operator, bukan key baru yang menutupi kegagalan decrypt.

Implementasi client memakai `golang.org/x/oauth2 v0.36.0`, tanpa menambah framework HTTP. Acuan: [OAuth Security BCP / RFC 9700](https://www.rfc-editor.org/rfc/rfc9700.html) dan [dokumentasi x/oauth2](https://pkg.go.dev/golang.org/x/oauth2).

## Invocation dan uninstall

Aktivasi remote memerlukan koneksi valid yang diperiksa ke reference app. Remote request membawa tenant/installation, bearer token terikat grant, capability, dan key idempotency yang diturunkan dari actor/tenant/installation/key caller. Provider menyimpan resource dan response idempotent dalam transaksi sendiri. Jika efek remote berhasil tetapi respons/commit platform hilang, retry mengembalikan hasil remote yang sama. Platform hanya menyimpan respons yang lolos validasi kontrak, binding, dan batas nilai. Semua response tetap `simulation: true`.

Lifecycle remote: `pending → active → disabling → uninstalled`. Saat masuk `disabling`, scopes dan routing dikosongkan secara atomik bersama event `emisell.app.disabling.v1`; operasi baru ditolak. Worker menghapus credential lokal dan pending state, lalu meminta revocation semua family untuk tenant/installation tersebut. Endpoint cleanup di reference app memakai confidential client auth sehingga orphan grant akibat code-exchange ambiguity tetap dapat dicabut. Baru setelah remote mengakui revocation, worker menyelesaikan `uninstalled` dan event-nya. Public API tidak menerima `complete_uninstall`.

Cleanup gagal: exponential backoff 2–256 detik, maksimum 12 attempt. Installation tetap `disabling` jika budget habis dan perlu operator `retry-cleanup` dengan alasan audit. Reinstall menunggu cleanup selesai dan selalu memperoleh installation ID baru. Simulator lama tetap uninstall sinkron seperti sebelumnya. Audit/resource lama disimpan; tidak ada penghapusan transaksi bisnis.

## Webhook

Arah increment ini adalah **Platform → remote app**, bukan callback pembayaran inbound dari provider. NATS durable `webhook_router` membaca hanya event capability. ACK broker dikirim setelah inbox dan delivery queue PostgreSQL commit; duplicate event tidak membuat delivery kedua. Worker adalah principal platform tepercaya lintas tenant, tetapi routing dibatasi exact tenant + installation subject, status aktif, dan signed release. ACL akun Core tidak diperluas; hanya ACK durable webhook yang ditambahkan pada akun worker.

- HMAC-SHA256 dengan secret per installation. Canonical bytes: `v1\n<timestamp>\n<tenant>\n<installation>\n<delivery-id>\n<body>`. Header: `X-Emisell-Tenant`, `X-Emisell-Installation`, `X-Emisell-Delivery`, `X-Emisell-Timestamp`, `X-Emisell-Signature` (`v1=<hex>`).
- Receiver memeriksa timestamp paling lama lima menit dan paling jauh 30 detik ke depan, signature constant-time, event contract/type, tenant/subject, serta grant belum revoked. Inbox dan efek referensi berupa audit receipt ditulis atomik. Duplicate sah diakui tanpa efek ulang; ID sama dengan body berbeda ditolak.
- Retry mempertahankan body, event ID, dan delivery ID; timestamp/signature diperbarui. Backoff 2–256 detik, maksimum 12 attempt atau usia 24 jam; setelah itu `dead`. Sukses remote dengan respons hilang tetap aman karena inbox receiver.
- Reconnect yang dibutuhkan tidak membuang event; delivery menunggu retry budget. Delivery untuk installation nonaktif/dihapus menjadi `cancelled`. Replay manual tidak dapat melewati gate uninstall.
- Replay hanya delivery dead, exact tenant/ID, alasan 8–240 karakter, dengan audit. Semantik at-least-once; tidak ada jaminan ordering global atau exactly-once universal. Broker retention tujuh hari tetap memerlukan rekonsiliasi operator jika outage melampauinya.
- CLI status tersedia; `/metrics` worker mencatat outcome dan pending/dead delivery. OTel exporter dan alert delivery masih perlu konfigurasi operasional. Tidak memerlukan Temporal untuk state machine pendek yang disimpan durably ini.

## Migration dan rollout

1. Jalankan `init-remote`: migration additive 0004, credential privat, schema fixture reference app, dan release remote baru. Migration 0001–0003 tidak ditulis ulang. Seluruh installation lama dipertahankan.
2. `init-remote` juga memperbarui ACL ACK worker pada `.local/nats.conf` tanpa rotasi password. Recreate hanya container NATS compose lokal untuk memuat ulang bind mount; volume dipertahankan.
3. Restart API/worker, jalankan `cmd/remote-reference`. Dashboard tetap localhost:4317; tidak menyentuh hosting/domain Sites.
4. Coba pada workspace kosong. Jangan mengganti provider aktif milik pengguna sebagai efek samping pengujian.
5. Rollback: jangan downgrade server saat masih ada profile remote/disabling. Selesaikan/revoke remote installations dahulu, hentikan reference app, pertahankan schema/audit dan file key. Tidak ada down migration/destructive reset. Increment dapat dinonaktifkan dengan tidak menambahkan remote release pada environment baru.

## Batas dan verifikasi

Belum production-ready: tidak ada login provider nyata, HTTPS/mTLS production, secret manager/KMS/rotation orchestration, per-service DB roles, dynamic publisher trust, remote artifact verification, multiregion/HA, UI webhook inspector, inbound provider callback, WASM/UI runtime, atau integrasi ke repository Core production. Sharing database development hanya untuk kemudahan fixture; deployment aplikasi pihak ketiga wajib memakai storage/credential terpisah.

Verifikasi: regression build/test/lint dan Buf breaking; PostgreSQL integration; cross-tenant/session/expiry/replay; rotating refresh reuse; encrypted secrets; ambiguous payment response; webhook duplicate/redelivery/tenant binding/signature window; durable consumer ACK melewati restart broker; uninstall fail-closed dan cleanup retry; UI install → OAuth → activate → uninstall. Test hanya memakai database test serta broker sementara; browser QA memakai workspace contoh tanpa mengganti installation utama.

### Hasil verifikasi — 5 September 2026

- `make verify` dengan PostgreSQL test dan NATS sementara: lulus, termasuk race detector, vet, build, dan Buf breaking terhadap baseline v1.
- Dashboard: 11 tests, typecheck, lint, build demo dan build lokal lulus. Pemeriksaan browser membuktikan alur consent/callback/aktivasi/uninstall; halaman consent dan detail aplikasi diperiksa secara visual.
- Smoke test development pada `local-studio`: satu pembayaran simulasi `phase3-local-qa`, satu webhook berstatus delivered, satu receipt receiver. Installation uji sudah uninstalled, koneksi dan grant reference app revoked. Audit/resource simulasi tetap disimpan. Installation Emisell Pay milik `local-store` tetap aktif dengan ID semula.
- API, worker, dan remote reference app siap pada loopback; pending/dead webhook sama-sama nol setelah smoke test. Hanya NATS compose increment lokal yang direcreate, volumenya dipertahankan.
- `govulncheck -scan=package`: tidak menemukan kerentanan pada package yang diimpor; masih satu advisory pada tingkat module dependency yang tidak diimpor sebagaimana catatan ADR 0003. Hasil ini bukan audit keamanan production lengkap.
