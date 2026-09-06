# ADR 0013 — Tabel kesiapan scope dan API key Core

Status: diterima untuk implementasi lokal, 2026-09-05.

Keputusan generation tenant-bound di bawah digantikan oleh ADR 0014 untuk key baru pada menu Admin. Catatan ini dipertahankan sebagai kontrak key legacy; keputusan tabel/readiness scope tetap berlaku.

## Keputusan dan batas

- Katalog scope menjadi tabel Admin/Developer dengan pencarian, filter status, detail izin, dependensi dan blocker. Required/optional editor tetap memakai kontrak lama.
- `GET /api/v1/{admin|developer}/access-scopes/verification` membaca inventaris build Platform. `checkedAt` adalah waktu pemeriksaan inventaris, bukan tanggal sukses uji gateway Core. `coreChecked:false`; 108 resource masih planned. Kontrak List/Get parsial tidak cukup untuk active. Kegagalan/ketidakcocokan hasil menjadi unknown, bukan active. Tidak ada toggle manual untuk membuka grant.
- Menu API Key berada di Admin dan khusus administrator. Konteks awalnya **backend Emisell Core → App Platform**, bukan app pihak ketiga. Jenis key developer memerlukan OAuth/installation binding dan keputusan tersendiri.
- Reuse `platform_identity.service_accounts` serta autentikasi RPC yang sudah ada. Migration 0010 hanya menambah metadata managed key dan indeks; key CLI lama tidak dimigrasikan, ditampilkan, atau dicabut oleh UI.
- Setiap key mengikat satu tenant backend yang sudah terdaftar, daftar eksplisit dari tujuh service scopes, dan expiry 1–30 hari. Tidak ada wildcard, resource scope Shopify, tenant otomatis, atau implicit read dari write pada service scopes. Form tenant ID adalah konfigurasi otorisasi administratif, bukan merchant workspace baru.
- Random secret 256-bit, SHA-256 hash saja di database. Secret diberikan sekali; tidak disimpan pada request-idempotency/audit, URL, log, atau browser storage. UI menghapus tampilannya setelah 5 menit atau unmount. Clipboard hanya melalui tindakan eksplisit pengguna.
- Generate idempotent terhadap actor + request key + normalized payload. Replay mengembalikan metadata TERKINI dan secret kosong. Jika response pertama hilang, revoke key tersebut dan generate yang baru. Concurrent retry menghasilkan satu account dan satu audit issued.
- Revoke bersifat naturally idempotent: audit sekali pada transisi, tidak menghapus metadata dan tidak menghidupkan key kembali. Token dicek ke database tiap request; permintaan baru setelah revoke/expiry ditolak. Request yang sudah berjalan tidak dijanjikan dibatalkan. Rotasi dilakukan dengan generate pengganti → pindahkan consumer → revoke lama.
- `emisell.integration.v1.ConnectionService/Check` memakai autentikasi Core yang sama, mengembalikan identitas key sendiri, tenant, service scopes dan expiry. Tidak butuh scope tambahan karena tidak membaca resource, tidak melakukan transaksi, dan tidak memverifikasi kesehatan gateway resource. Menerima semua Core key valid, termasuk legacy, tanpa memperluas haknya.
- Akun admin/operator/developer dan business data tidak berubah. Verifikasi lifecycle key dilakukan di database test terisolasi, bukan dengan membuat key pada database pengguna.

## Security dan rollout

Origin/session/role admin enforced server-side. Internal RPC tetap loopback-only dan menolak Origin/cookie browser. DB workspaces tidak menjadi endpoint publik baru. User-supplied URL tidak pernah diprobe. Status active key berbeda dari status active resource scope.

Deployment sekarang lokal; jangan mengekspos listener plaintext atau memakainya sebagai trust model production. Deployment nyata memerlukan TLS/mTLS sesuai threat model, secret manager pada consumer, re-auth/MFA untuk issuance, rate/quota policy, monitoring penyalahgunaan dan review operasional. Resource gateway tetap melalui gate ADR 0012 dan tidak boleh menggunakan key ini sebagai substitusi app delegation/grant.

Migration additive, rollback aplikasi dapat mengabaikan metadata baru tanpa menghapus keys. Sebelum menarik fitur, cabut managed keys yang tidak lagi diperlukan melalui jalur terotorisasi. Jangan mengubah migration yang sudah diterapkan atau menurunkan baseline kontrak Protobuf lama.
