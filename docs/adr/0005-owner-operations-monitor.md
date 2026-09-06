# ADR 0005 — Monitor operasional dan recovery milik workspace

> Historis untuk frontend: menu Operasional merchant dan harness browser-nya telah dihapus. Backend recovery/monitoring tetap ada untuk compatibility/reference; bukan API admin. ADR 0007 menjadi acuan pengganti UI.

Tanggal: 5 September 2026. Status: diterapkan untuk development lokal. Memperluas ADR 0004 tanpa mengubah capability, RPC, SDK, manifest, atau event contract v1.

## Keputusan dan batas modul

Dashboard lokal menambah menu **Operasional**, dengan tab Webhook dan Koneksi & pemulihan. Tidak ada menu ini dalam adapter demo. UI memakai primitives, theme, dan navigasi yang sudah ada; tanpa dependency baru, deployment, atau perubahan hosting Sites.

Use case monitoring berada di modul webhook dan installation. HTTP hanya menerjemahkan request/response; SQL tetap dalam repository owner schema. `platform/recovery` hanya menyimpan primitive validasi, fingerprint, dan hasil permintaan, bukan policy domain. Registry diambil melalui port dan diverifikasi seperti routing sebelumnya. Pool query webhook/registry dipisahkan dari pool yang memegang lifecycle gate.

Semua endpoint membutuhkan session dan membership **owner** pada tenant yang diminta, termasuk GET, detail, cursor pagination, dan retry. Foreign resource pada workspace yang sah tetap 404; cursor asing/tidak ditemukan 400. Workspace yang tidak diotorisasi 404, bukan informasi keberadaan resource. UI bukan security boundary.

## Pembacaan aman

- List webhook: 20 row/page, keyset `(enqueued_at, id)` descending. Count mencakup seluruh status tenant, bukan hanya filter. Snapshot transaksi konsisten per request; pergantian status saat pagination dapat mengubah isi halaman berikutnya. Gunakan muat ulang untuk snapshot terbaru.
- Detail: ID, binding installation/event, event type, correlation ID, status, attempts, revision, waktu, safe outcome code, dan maksimal 50 audit terbaru. Body hanya dibaca internal untuk validasi binding dan tidak dikirim ke browser. Token, signing secret, signature, header, dan payload tidak masuk DTO.
- OAuth: **status terakhir tersimpan**, bukan live provider health. GET tidak menjalankan refresh, introspection, atau request provider. Simulator tanpa OAuth dan instalasi yang sudah selesai uninstalled tidak masuk daftar koneksi. Audit uninstall lengkap tetap disimpan di modul installation meski kartu hilang setelah selesai.
- UI membedakan loading, empty, error, sukses penjadwalan, dan hasil akhir. Error tidak berubah menjadi angka nol palsu. Perubahan workspace/filter/detail membatalkan penerimaan respons lama. Respons setelah perubahan session tidak dapat memulihkan data tenant lama.
- Tidak ada polling monitor otomatis. Tombol Muat ulang memberikan snapshot eksplisit; menghindari perubahan fokus/dialog saat operator sedang memeriksa masalah. Timestamp selalu WIB.

## Recovery dan concurrency

POST memerlukan exact Origin, JSON, `Idempotency-Key` 16–128 karakter, `expectedRevision` non-negatif, dan alasan valid UTF-8: minimum 8 byte setelah trim, maksimum 240 byte total, tanpa control character. Jangan masukkan token, password, atau PII. Alasan adalah input pemilik, bukan hasil otomatis yang dijamin bebas PII; React menampilkannya sebagai teks, bukan HTML.

Webhook hanya boleh diulang bila **dead**, installation masih active, profile remote dan versi/scopes sesuai signed registry. Gate lifecycle yang sama dengan uninstall memegang otorisasi sampai requeue commit. Requeue mempertahankan body, event ID, delivery ID, dan enqueue timestamp; budget menjadi 12 percobaan/24 jam baru. Retry belum berarti pengiriman berhasil dan tidak membuat transaksi bisnis baru.

Cleanup hanya boleh dilanjutkan dari **disabling dengan 12 percobaan habis**. Policy diperiksa kembali dalam transaksi pemegang lock installation. Mengulang hanya mengatur attempts=0 dan jadwal sekarang; status tetap disabling, scopes/routing tetap kosong. Tidak ada tombol “paksa selesai”, reaktivasi, atau bypass revocation. Kegagalan worker dicatat dengan reason code tetap; stale worker failure tidak memakan budget revision yang baru.

Dalam transaksi owner module: cek request fingerprint → cek status/revision → perubahan antrean → audit actor/reason → simpan acceptance. Tenant/actor/key yang sama dengan body sama mengembalikan acceptance awal. Body berbeda dengan key sama, status tidak memenuhi policy, atau revision kedaluwarsa menghasilkan 409. Acceptance cache bukan current state; lakukan GET setelahnya. Webhook yang sudah melewati uninstall dapat ditolak meskipun key pernah diterima, karena lifecycle gate tetap fail-closed.

Adapter mempertahankan key untuk hasil network/5xx yang belum pasti. Dialog mengunci alasan saat hasil ambigu dan memberi “Coba konfirmasi lagi”; tidak membuat key baru pada retry yang sama. Klik ganda dicegah di UI dan transaksi backend. Idempotency request dan audit belum diberi expiry otomatis; retention policy production harus menjaga window deduplication dan kepentingan audit.

Semantik tetap at-least-once, bukan exactly-once universal. Receiver wajib dedup berdasarkan identitas event/delivery. Manual recovery tidak menyelesaikan outage, koneksi OAuth yang perlu dibuat ulang, atau worker yang berhenti; pemilik harus menangani sebabnya terlebih dahulu.

## Migration dan rollout

1. Migration additive **0005** menambah revision, actor audit, recovery request tables, timestamp metadata, dan index history tenant. Migration 0001–0004 tidak diubah.
2. `created_at` lama tetap menjadi awal retry cycle agar CLI/worker kompatibel. `enqueued_at` baru memakai nilai created_at saat migrasi sebagai sejarah terbaik yang tersedia dan tidak berubah pada replay berikutnya. Waktu attempt/completion historis tidak direka: NULL ditampilkan “Belum tercatat”.
3. Jalankan `go run ./cmd/cli migrate`, restart hanya server dan worker proyek lokal. Tidak perlu recreate database/NATS, rotasi credential, atau menyentuh instance gateway lama. Jangan menjalankan worker versi lama bersamaan dengan baru: worker lama belum menaikkan revision/metadata.
4. Rollback aplikasi membutuhkan penghentian worker baru dahulu. Pertahankan schema/audit; tidak ada down migration/destructive reset. Hindari downgrade write paths saat ada recovery yang belum selesai, karena aplikasi lama tidak menjaga revision baru. Baca metadata tetap aman.

## Verifikasi dan batas berikutnya

Integration test menggunakan `emisell_local_test`: pagination/filter/tenant isolation, metadata redaction, session/Origin, body/key/revision validation, concurrent duplicate retry, audit atomic, disabled/uninstalled gate, cleanup exhausted/retry/stale worker, serta penyelesaian revocation. Frontend menguji session invalidation, stable idempotency key, error handling, dan byte validation. Buf breaking menjaga contract Core v1 tetap sama.

Harness browser **opt-in `_test.go` saja** memakai port `127.0.0.1:4319`, database test dan provider fixture. Frontend diproxy dari localhost:4317. Cookie host berbeda dari dashboard pengguna. Endpoint `__qa/*` hanya hidup dalam proses test eksplisit, tidak dikompilasi ke server aplikasi. Skenario gagal dibuat sebagai fixture, bukan mengubah delivery pengguna. Tutup harness setelah QA.

Belum mencakup realtime alerts, export audit, broker/outbox inspector, inbound payment callback, provider production, atau production observability deployment. Koneksi yang selesai uninstalled tidak muncul sebagai kartu; webhook historis tetap dapat dibaca. Status operasional ini bukan laporan transaksi finansial.

### Hasil verifikasi — 5 September 2026

- `make verify` dengan PostgreSQL test dan broker sementara lulus: race detector, vet, build, Buf format/lint/breaking. Harness browser bersifat opt-in dan dikecualikan dari run otomatis biasa.
- Dashboard: typecheck, lint, 15 tests, build demo dan build lokal lulus. JSON dan seluruh referensi lokal kedua dokumen OpenAPI diperiksa.
- Browser desktop/mobile: filter, pagination 20+3, detail, empty/error/loading, perpindahan workspace, status OAuth, retry webhook, dan recovery cleanup diperiksa. Retry mempertahankan identitas delivery/event dan menambahkan audit pemilik; cleanup tetap disabling dengan scopes/routing tidak dibuka. Fokus keyboard dikembalikan ke konteks yang masih tersedia. Layout mobile memakai kartu, tanpa overflow horizontal halaman.
- Sempat ditemukan benturan candidate antrean ketika harness browser dan regresi memakai database test bersamaan. Test worker sekarang memilih tenant fixture-nya sendiri; enqueue/locking/audit/replay tetap memakai repository nyata. Fixture yang selesai mempertahankan sejarah, membatalkan pending test deliveries dan menghentikan penjadwalan cleanup test. Regresi ulang lulus; tidak ada perubahan policy worker aplikasi lintas tenant.
- Migration 0005 diterapkan pada database development tanpa reset. API/RPC dan worker direstart; API, worker, dan reference app siap pada loopback. Proses harness port 4319 sudah dihentikan setelah QA.
- Instalasi `local-store/emisell-pay` tetap active dengan ID `ins_EFKEZBQFRNR2RWRZRGJUTITNZY`. Remote Pay historis di local-studio tetap uninstalled. Tidak ada mutasi aplikasi pengguna, deployment, rotasi credential, atau perubahan hosting. Seluruh 349 penghapusan tracked yang sudah ada tetap dipertahankan.
