# ADR 0021 — Distribusi aplikasi uji, terpisah dari instalasi

Status: diterapkan untuk assignment, persetujuan dan daftar dinamis. Runtime aplikasi developer **belum tersedia**.

Pembaruan 2026-09-08: runtime UI dengan `read_products` tersedia untuk pengujian
lokal opt-in; lihat [alur terkini](../resource-test-distribution.md). Seller
dengan izin kelola Apps kini dapat menghentikan assignment approved milik
tokonya melalui `StopAssignment`, tanpa hak approval. Riwayat tidak dihapus
dan uninstall tetap terpisah. Untuk runtime yang mensyaratkan assignment aktif,
pencabutan juga menolak akses berikutnya. Bagian batas runtime di bawah mencatat
keputusan awal sebelum sambungan ini tersedia.

## Keputusan

Portal Developer mengajukan release integrasi **signed** milik organisasinya ke `merchantId` yang diketahui. Tidak ada pencarian atau daftar seluruh merchant dari portal. Dashboard Admin memeriksa tujuan pengujian; hanya `administrator` boleh menyetujui, menolak, atau mencabut. Role reviewer/operator hanya membaca. Persetujuan memverifikasi merchant terdaftar melalui public port modul identity.

Assignment immutable mengikat organisasi, release ID, checksum konfigurasi dan merchant. State: `requested → approved|rejected`, `approved → revoked`. Versi/merchant baru memerlukan request baru. Satu assignment requested/approved per release+merchant. Perubahan memakai revision, idempotency key dan audit alasan; retry keputusan mengembalikan status terkini, tidak menghidupkan assignment kembali. Request ulang tetap memerlukan release signed yang valid.

Modul app memiliki service/repository assignment. Persetujuan menahan shared lock release dari pool terpisah selama commit singkat assignment; suspension tidak bisa melewati validasi source secara bersamaan. Tidak ada network probe dalam transaksi. Core tidak mengakses SQL Platform.

## Kontrak dan batas akses

- Portal: `/api/v1/{admin|developer}/test-assignments`, `/{id}`, dan Admin `/{id}/status`. Cookie, exact origin, developer organization scope. Pagination maksimal 20, `afterId` hanya posisi baca.
- Internal: `emisell.testing.v1.TestDistributionService/ListAssignments`, `merchantId`, `coreActorId`, `pageSize`, `afterId`. Wajib full-access key Core yang masih valid dan merchant terdaftar; legacy key/app token ditolak.
- Core browser facade: `GET /v1/app-platform/core/test-apps`. Core memperoleh merchant/actor dari cookie sesi terverifikasi primary DB, memeriksa izin kelola aplikasi setiap halaman. Tidak menerima browser identity overrides. Response no-store, pagination 20, tanpa credential, digest consent, actor, private endpoint atau organization membership.
- Merchant Dashboard Settings → Apps menampilkan hanya assignment **approved** untuk merchant sesi. Tidak ada fallback ke Emisell Pay/Parcel hardcoded. Installed apps dan lifecycle fixture lama tetap kompatibel; removal kartu contoh bukan uninstall.

## Readiness bukan grant

`configurationReady` memeriksa status signed, signature dan checksum terkini. `requiredScopesReady` mengikuti pemeriksaan scope wajib dari integration manifest, **bukan** bukti grant aktif, app-client ready atau gateway production siap. Scope optional Plan tetap tidak diberikan akses. Detail daftar bukan sumber deklarasi scope kedua; manifest dan katalog scope yang ada tetap authoritative.

`installable=false` selalu: runtime executable aplikasi developer, instalasi/OAuth token exchange, verifikasi kontrak endpoint dan egress belum selesai. Blocker berupa kode stabil. Release suspended membuat configurationReady=false pada pembacaan berikutnya, tanpa mengubah status persetujuan. App-client readiness tetap diperiksa di modul App clients terpisah; tidak disamakan dengan kesiapan instalasi.

Assignment/approval tidak memanggil Prepare/Consent/Consume/Activate/IssueToken, tidak membuat registry executable, dan tidak mengaktifkan scope Plan. Pencabutan assignment menutup distribusi uji baru; tidak mencabut grant atau uninstall aplikasi yang sudah terpasang. Tindakan lifecycle tetap eksplisit dan owner-bound.

## Migrasi dan verifikasi

Migration additive `0015_test_assignments.sql`; tidak mengubah tabel instalasi, keys, merchants atau scope. Backup database lokal sebelum apply, tidak seed/reset. Rollback operasional: nonaktifkan menu/facade jika perlu, pertahankan tabel dan audit; gunakan forward migration untuk perubahan schema.

Uji terisolasi mencakup organisasi asing, merchant asing, role, idempotency/concurrency, stale revision, signature/status suspension, duplicate active assignment, pagination, revocation, immutable target, serta tidak ada instalasi dari assignment. Uji Core → Connect → PostgreSQL membuktikan daftar dinamis dan fresh session checks. UI tidak menawarkan instalasi developer sampai milestone runtime tersendiri selesai.
