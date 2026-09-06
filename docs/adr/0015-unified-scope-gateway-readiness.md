# ADR 0015 — Satu sumber status untuk katalog scope dan dokumentasi endpoint

Status: diterima, 2026-09-05.

## Masalah dan keputusan

Matriks 108 scope pada dokumentasi gateway menduplikasi Katalog Scope, sementara status dokumentasi berasal dari build snapshot dan katalog berasal dari API verifikasi. Pengguna tidak boleh memelihara dua status atau menganggap perubahan label sebagai implementasi endpoint.

- Katalog Scope menjadi pusat izin, status, mapping procedure, kelengkapan dan kendala. Dokumen API hanya menampilkan endpoint, scope yang diterima, schema/contoh dan checklist. Matriks duplikat UI dihapus; snapshot kontrak tetap tersedia untuk unduhan/handoff developer.
- Backend `gatewaycontract.VerifyReadiness` adalah satu read model scope **dan** operasi. Endpoint Admin/Developer yang ada dipertahankan; menambah field `operations`, `environment:local`, dan `contractRevision` tanpa mengubah field lama. Fingerprint berasal dari kontrak/mapping versioned, bukan clock atau health signal.
- Kedua tampilan menggunakan loader bersama dengan sesi portal yang benar. Status dibaca ketika halaman dibuka atau Periksa ulang status ditekan. Tidak ada cache lintas pengguna, localStorage, credential baru, live push, polling, atau request resource/probe gateway dari browser.
- Detail endpoint menunjukkan status operasi dan scope secara terpisah. Scope Active memerlukan coverage lengkap, semua operasi terkait siap dan seluruh gate bukti Core/grant. Satu operasi baca Active tidak mempromosikan seluruh scope read/write. Status UI tidak menjadi authorization dan tidak membuka grant.
- Profil laporan yang berbeda, payload tidak valid, service gagal, operasi duplikat/tidak ada, atau fingerprint docs/backend tidak cocok gagal tertutup. Dokumen schema tetap dapat dibaca, tetapi status menjadi Belum terverifikasi. Tidak mengambil status dari static `implementation`/`availability` metadata.
- Tautan Katalog → endpoint memakai `api_operation`, endpoint → Katalog memakai `scope`. Parameter hanya navigasi lokal, bukan URL backend atau bukti akses. Procedure unknown menampilkan endpoint tidak ditemukan, bukan fallback ke procedure pertama. Developer tetap menggunakan katalog surface sendiri dan tidak diberi akses dokumentasi Admin.

## Batas implementasi

Ini penyatuan read model dan UI, bukan integrasi resource live. Seluruh 108 scope dan dua operasi List/Get produk tetap Plan, `coreChecked:false`, `grantable:false`. Core gateway, executable/grant/token pipeline dan production trust belum tersedia. Pendaftaran dukungan dan bukti verifikasi deployment adalah milestone berikutnya melalui ADR/test, bukan toggle status atau edit label. Host gateway tetap `CORE_GATEWAY_BASE_URL` sampai konfigurasi integrasi nyata ditentukan; Active tidak boleh membuat URL berpindah ke port Platform.

## Rollout dan rollback

Tidak ada migration/database mutation, account/key change atau dependency baru. Bangun backend/read model terlebih dahulu, regenerate kontrak/frontend, lalu jalankan UI. Frontend baru terhadap backend lama menampilkan status belum terverifikasi karena metadata operasi/fingerprint belum ada. Frontend lama mengabaikan field tambahan; perilaku scope sebelumnya tetap tersedia. Rollback binary/UI tidak mengubah grant, instalasi atau data. Jangan mengedit profil scope immutable atau menghapus baseline Protobuf.

## Verifikasi

Uji backend: fingerprint stabil dan mapping bidirectional scope/operasi; seluruh status build inventory tetap Plan; batas session/origin Admin/Developer tetap ditegakkan. Uji frontend: dua pembaca memperoleh read model sama, refresh mengikuti laporan terbaru, partial scope tidak naik karena operasi siap, gagal/malformed/mismatch/duplikat menjadi unknown, navigasi encoded lokal, tidak ada matriks duplikat, URL gateway tidak bergantung status. Jalankan generated-doc drift check, typecheck, lint, tests/build portal dan backend race/build/vet yang relevan.
