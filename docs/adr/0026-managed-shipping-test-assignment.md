# ADR 0026 — Distribusi pengujian provider terkelola

Status: implementasi distribusi lokal, 6 September 2026. **Bukan aktivasi grant engine atau instalasi merchant.**

## Keputusan

Menu Testing existing menerima dua sumber release. Input `releaseKind` dihilangkan untuk Aplikasi Integrasi historis, atau exact `managed_shipping` untuk provider terkelola. Tidak menebak jenis dari nama, prefix ID, URL, atau metadata browser. API memverifikasi organisasi developer, snapshot, signature, checksum dan status signed pada sumber yang dipilih.

Assignment tetap `requested → approved|rejected`, `approved → revoked`, dengan admin-only approval/revoke, merchant terdaftar, revision, idempotency, receipt dan audit. Shared lock pada release di pool gate terpisah ditahan sampai commit assignment, sehingga suspend tidak berlomba dengan approval. Retry request/approval membaca status terkini; tidak menghidupkan assignment terminal atau membuka akses saat signing bermasalah.

Migration 0017 mempertahankan foreign key integration release lama, menambahkan foreign key managed release, constraint tepat satu sumber, unique index untuk assignment current managed, dan trigger yang menjaga binding managed immutable. Tidak mengubah signed bytes atau data assignment lama. Field wire `releaseId` tetap ID sumber terpilih; `releaseKind` tambahan hanya untuk portal, bukan identity merchant.

Core RPC `ListAssignments` tetap satu daftar terurut/paginated dan terisolasi merchant. Hanya approved yang terlihat. Readiness dihitung ulang dari sumber; suspend membuat konfigurasi belum siap. Core dan Dashboard menerima dua blocker managed yang eksplisit, tetap menolak `installable:true`, unknown blocker, mismatch merchant, dan page tidak valid. Settings → Apps tetap halaman existing, tanpa kartu fixture/sandbox baru.

## Batas dan ketergantungan

- Assignment tidak membuat intent, consent, installation, token, grant, selection provider atau request API-Kurir.
- Managed release tidak dimasukkan ke registry executable fixture. Endpoint OAuth fiktif tidak diperlukan.
- `managed_installation_not_available` dan `engine_grant_enforcement_not_available` tetap gate nyata. Additional shipping methods tidak boleh menampilkan app seolah sudah dipasang hanya karena assigned.
- Pengguna perlu menentukan target engine untuk tahap grant lintas service: engine lokal terisolasi atau rollout engine yang dikelola operator. Tidak ada deploy/perubahan API-Kurir production oleh increment ini.
- Grant harus dipasang pada engine/seluruh operasi pilot, termasuk jalur legacy dan replay, sebelum install eligibility dapat dibuka. Memanggil engine production lama dari UI baru bukan integrasi grant yang sah.

## Rollout dan rollback

Backup privat Platform lokal sebelum migration. Jalankan migration, restart backend dengan managed signer untuk HTTP dan Core RPC, lalu gunakan API Developer → Admin untuk assignment demo yang telah diotorisasi. Tidak auto-consent/install. Consumer Core/Dashboard diperbarui bersama agar mengenali blocker baru.

Binary lama tidak memahami managed assignment: setelah data managed dibuat, jangan rollback ke binary lama tanpa mengisolasi consumer melalui release baru/forward-fix. Revoke tersedia untuk menghentikan distribusi; jangan menghapus tabel, audit, key, atau assignment history.

## Verifikasi

PostgreSQL disposable: rilis signed → request → approval → merchant list; belum signed, wrong kind/source, foreign org/merchant, reviewer, unknown merchant, duplicate/replay, FK, immutable source, suspend dan revoke. Assert tidak ada installation yang tercipta dan Prepare tetap menolak app sebagai fixture.

Regresi pipeline integration/release dan Testing lama, Core Express → ConnectRPC + Prisma/PostgreSQL, unit parser Core/Dashboard, portal/API docs, build/typecheck/lint dan Go build/vet/race. Semua pengujian memakai database/sesi sintetis, bukan token merchant/browser pengguna.
