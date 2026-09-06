# ADR 0020 — Daftar aplikasi terpasang untuk Dashboard Core

Status: accepted, 5 September 2026. Memperluas read model ADR 0016; tidak mengubah ownership mutation atau grant.

## Masalah

Instalasi Emisell Pay telah active tetapi Dashboard Core masih menampilkan placeholder Installed Apps. Status harus dibaca dari Platform, bukan direkonstruksi dari klik Install, URL, browser storage, intent expired, atau tabel PluginApp legacy.

## Keputusan dan boundary

- RPC additive `InstallationService/ListInstallations`, request `merchantId`, `coreActorId`, `pageSize` dan `afterId`. Hanya key platform full-access dengan merchant dikenal; Core memverifikasi ulang sesi, membership dan izin kelola apps setiap request/halaman. Key legacy, sesi portal dan token app bukan credential operasi ini.
- **Read model merchant-wide**, lintas actor dan key Core pemasang. Instalasi adalah milik toko; mengganti staf atau key koneksi tidak menghilangkan aplikasi dari daftar. Ini hanya metadata untuk app manager terotorisasi, bukan transfer ownership. GetInstallation, consent, Activate, IssueToken dan Uninstall tetap terikat merchant + service + actor awal.
- Response hanya ID instalasi/app, nama, developer ID, versi, status, grant state, execution profile dan waktu pemasangan. Tidak ada token, scopes, consent/manifest digest, intent ID, actor atau endpoint provider.
- Query tunggal pada aggregate installation/grant/consumption memastikan snapshot status konsisten. Hanya current intent-managed installation; pending, active, disabling dibedakan. Uninstalled history, consumption receipt lama setelah reinstall dan instalasi legacy tidak disertakan. Grant/status mismatch gagal tertutup, bukan diberi badge Active.
- Pagination keyset `installation_id` ascending, default/maksimum 20, `nextAfterId` kosong di akhir. Cursor hanya posisi baca, bukan akses; filter merchant berasal dari authorization setiap request. Antarhalaman bukan snapshot transaction global; refresh membaca ulang halaman awal.
- Facade cookie-only Core `GET /v1/app-platform/core/installations[?afterId=...]` mempertahankan Origin exact, custom header, no-store, limiter, timeout dan safe-error policy. Browser tidak mengirim merchant/actor/token/pageSize; query asing/duplikat ditolak. Data upstream divalidasi dan output di-allowlist.
- Dashboard membaca daftar setelah mount/refresh/kembali dari Install. Success activation mengarahkan ke Apps; `installed=<id>` hanya navigasi dan banner valid jika daftar backend menemukan ID active/grant active. Membuka daftar tidak memberi consent, mengaktifkan app, mengeluarkan token atau melakukan reinstall.
- List tetap tersedia ketika flag mutation install off selama preview connection aktif. Backend gagal berarti error dengan retry, bukan daftar kosong/sukses palsu. Pergantian merchant/actor membuang state dan membatalkan request browser lama.

## Rollout dan rollback

Tidak ada migration, perubahan key/akun, grant atau instalasi existing. Deploy server Go dahulu, lalu facade Core dan Dashboard. Binary lama tanpa RPC list menghasilkan error terkontrol; jangan fallback ke database lintas modul, endpoint workspace legacy atau client cache. Rollback UI/list tidak mencabut instalasi aktif. Developer runtime, scope resource Plan, billing, Uninstall UI dan recovery ownership mutation tetap di luar milestone.

## Bukti

Unit Dashboard/Core memeriksa schema, redaksi, identitas, pagination dan error handling. Runner disposable lintas repo menguji sesi Express/Prisma → Connect → PostgreSQL, daftar owner/staf yang sama, merchant lain, pencabutan sesi/role dan suspension. `TestCurrentMerchantInstalledList` menguji paginasi, pending/active, pembacaan lintas key tanpa pengambilalihan mutation, uninstall/reinstall dan tidak adanya mutation dari list. Live QA hanya membaca instalasi yang sudah dibuat pengguna, bukan memasang ulang.
