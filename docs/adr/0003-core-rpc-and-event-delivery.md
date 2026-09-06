# ADR 0003 — Kontrak Core dan pengiriman event lokal

Tanggal: 5 September 2026. Status: diterapkan untuk development lokal. Melanjutkan ADR 0002; bukan deployment produksi atau integrasi ke repository Emisell Core sesungguhnya.

## Keputusan

Core mengakses capability melalui ConnectRPC di listener loopback `127.0.0.1:8088`. Server REST/dashboard tetap pada `8087`. Keduanya berada pada executable modular monolith yang sama, memanggil use case dan gate installation yang sama. Pemanggilan antarmodul tetap in-process; tidak ada ekstraksi microservice.

Kontrak `api/proto/emisell/payment/v1` dan `shipping/v1` menghasilkan package publik `pkg/sdk/gen`. SDK tidak memuat nama atau tipe provider. Client referensi `cmd/core-reference` mengonsumsi kontrak ini; mengganti installation tidak memerlukan perubahan client. Resource lama tetap terikat pada installation pembuatnya, bukan dipindahkan otomatis.

Buf memakai lint STANDARD, breaking policy FILE, dan snapshot initial v1 `api/proto-baseline.binpb`. Snapshot ini adalah baseline kontrak, bukan file yang diperbarui rutin untuk menghilangkan kegagalan breaking check. Field number tidak boleh dipakai ulang. Breaking change membutuhkan v2 dan compatibility window. Generated Go berasal dari plugin yang dipin di `go.mod`, bukan diedit manual. [Dokumentasi Connect](https://connectrpc.com/docs/go/getting-started/) dan [konfigurasi Buf](https://buf.build/docs/configuration/v2/buf-gen-yaml/) menjadi acuan implementasi generator.

## Identitas service dan keamanan

- Service account terpisah dari browser user/session. Satu account mempunyai tepat satu tenant, explicit scopes, expiry, dan revocation state. Tidak ada kepercayaan pada tenant/actor header buatan caller.
- Akun referensi hanya boleh mengakses `local-store`. Scope service DAN grant installation aktif harus lolos sebelum invocation. Browser cookie/token tidak berlaku pada RPC.
- Token acak 256 bit hanya disimpan sebagai SHA-256 di PostgreSQL. Credential development di `.local/core.json` berizin `0600`, direktori `0700`, diabaikan Git. Token lokal berlaku 24 jam; issue/rotate/revoke tercatat pada audit identity. Tidak ada token dalam log/event.
- SDK tidak menggunakan environment HTTP proxy, tidak mengikuti redirect, membatasi timeout delapan detik, dan tidak melakukan retry tersembunyi. Caller mempertahankan business idempotency key pada retry setelah hasil ambigu.
- Listener RPC menolak Origin browser dan Host eksternal. Body decoded maksimum 32 KiB; use case maksimal 10 detik. Request ID dan trace ID dicatat tanpa body/token.
- HTTP/NATS tanpa TLS hanya boleh loopback development. Production membutuhkan HTTPS/mTLS atau workload identity yang disetujui, secret manager, rotation operasional, dan pemisahan credential broker/DB. Konfigurasi lokal ini tidak boleh dipublikasikan melalui reverse proxy.

Error RPC stabil: invalid argument untuk input salah; unauthenticated untuk token tidak sah/kedaluwarsa/dicabut; permission denied untuk scope kurang; not found untuk tenant/resource/routing yang tidak dapat diakses; already exists untuk idempotency conflict atau state conflict; unavailable untuk runtime tidak tersedia. Detail database/provider tidak dikirim ke caller.

## Outbox dan JetStream

`cmd/worker` menjalankan adapter delivery dari repository setiap modul. Modul event tidak query tabel installation/capability langsung. Helper SQL outbox adalah primitive infrastruktur dengan identifier owner yang dibatasi, bukan domain/shared business layer.

- Migration baru menambahkan metadata delivery; envelope, event ID, dan transaksi domain lama tetap kompatibel.
- Relay mengunci satu row dengan `FOR UPDATE SKIP LOCKED`, publish dengan deadline dua detik, lalu mengisi `published_at` **hanya setelah PubAck**. Worker lain dapat memproses row berbeda. Tidak ada klaim ordering global atau ordering per aggregate pada increment ini.
- Broker memakai file storage dan volume lokal khusus, acknowledgement, `Nats-Msg-Id=event.id`, deduplication window 24 jam, stream retention tujuh hari, maksimum 256 MiB, `DiscardNew`. Jika kapasitas habis, publish gagal dan outbox tidak ditandai terkirim.
- Retry publisher: exponential backoff 2–256 detik, maksimum 12 attempt; kegagalan pada event berusia lebih dari 24 jam atau envelope tidak valid masuk antrean gagal. Event tidak dihapus. Replay operator hanya untuk row dead yang belum pernah diakui, dengan alasan dan audit, mempertahankan ID asli.
- Jendela crash antara PubAck dan commit database dapat menghasilkan pengiriman ulang. Karena deduplication broker berbatas waktu, consumer tetap wajib menyimpan inbox. Semantik end-to-end **at-least-once**, bukan exactly-once universal. [Semantik JetStream](https://docs.nats.io/nats-concepts/jetstream/streams) dan [consumer acknowledgement](https://docs.nats.io/learn/jetstream/pull-consumers) mendasari keputusan ini.
- `/metrics` worker di loopback `8089` mencatat outcome delivery, pending, dan dead; `/readyz` memeriksa DB/broker. RPC memiliki metrik terpisah pada `8088/metrics`. Exporter OpenTelemetry tetap belum dikonfigurasi.

## Consumer Core referensi

Envelope JSON v1 dipublikasikan pada `emisell.events.<tenant>.<event-type>`. Kontrak dibagikan melalui `pkg/sdk/events` tanpa tipe broker. Akun broker consumer memiliki inbox prefix privat dan hanya boleh membaca durable `core_local-store`; tidak boleh membuat/mengubah filter, membaca stream langsung, mengambil durable tenant lain, atau memublikasikan event. Provisioning dilakukan worker bercredential operator terpisah. Password broker acak dan hanya berada pada file lokal privat.

Consumer memvalidasi envelope, version/type, tenant, dan kecocokan subject. `reference_core.inbox` dan side effect referensi berupa penghitung receipt ditulis pada transaksi PostgreSQL yang sama. ACK dikirim setelah commit. Duplicate ID dengan isi berbeda dikarantina; payload asing/rusak tidak disimpan dalam inbox tenant.

Consumer menggunakan backoff 5/30/120 detik dan 16 pesan in-flight maksimum. Kegagalan DB tidak di-ACK dan tidak dibuang setelah sejumlah reconnect; transport mengulang sampai pulih atau retention stream tujuh hari berakhir. Pesan invalid yang dapat dicatat pada DB diterminate setelah dead-letter metadata tersimpan. Ini **bukan** bounded business workflow: tidak ada efek eksternal atau business handler Core pada contoh ini. Jika DB mati melampaui retention, operator perlu rekonsiliasi dari audit/outbox; retention/backup/alerting produksi wajib dirancang sebelum rollout.

Dead-letter consumer berisi sequence/reason tanpa payload asing. Tidak ada automatic poison-message replay. Setelah perbaikan kontrak, recovery consumer memerlukan tindakan operator terpisah; CLI `replay-event` hanya untuk kegagalan outbox publisher. Jangan menyamakan keduanya.

## Migration, rollout, dan batas berikutnya

1. Jalankan `init-events` untuk file broker privat; start hanya service NATS baru dan pertahankan volume/database sebelumnya.
2. Jalankan `migrate` secara eksplisit. Migration 0001 tidak berubah. Restart server agar listener RPC tersedia.
3. Jalankan `init-core`, lalu worker. Schema `reference_core` adalah storage milik client contoh, tidak dibaca engine. Pada integrasi Core nyata, pindahkan inbox/side effect ke database dan repository Core sendiri melalui migrasi terpisah.
4. Jalankan consumer dan client reference; mulai dengan simulator tanpa transaksi asli. UI tidak memerlukan perubahan.
5. Rollback dengan menghentikan worker/consumer dan kembali ke server sebelumnya. Schema additive serta outbox dipertahankan, tidak ada `down -v` atau rewrite migration. Event pending dapat dilanjutkan setelah versi baru kembali.

Belum termasuk OAuth remote app, signed webhook delivery, WASM, provider production, cluster/HA NATS, Core production repository, atau dashboard pemantauan event. Satu broker development tidak memberikan toleransi kegagalan disk/host.

## Verifikasi yang diwajibkan

RPC: seluruh tujuh operasi, validasi scope/tenant, session rejection, expiry/rotation/revocation, stable idempotency, provider switching dan akses tertutup setelah uninstall. Events: PostgreSQL nyata, PubAck ambiguity, inbox commit-before-ACK crash, duplicate redelivery, restart broker dengan store persisten, bounded publisher retries/dead/replay audit, invalid envelope dan broker ACL. Test NATS memakai proses/broker sementara tersendiri; tidak menghentikan broker development pengguna.

Audit dependency menemukan advisory pada package SSH/OpenPGP dalam dependency transitif `golang.org/x/crypto` yang tidak diimpor aplikasi. Versi dinaikkan dari 0.49.0 ke 0.56.0 tanpa menaikkan minimum Go 1.26.6. Package OpenPGP yang ditandai tidak terpelihara tidak boleh diadopsi. Hasil audit harus dibaca pada level package/symbol yang terjangkau; keberadaan module tidak sama dengan bukti eksploitabilitas aplikasi.
