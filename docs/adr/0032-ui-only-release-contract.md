# ADR 0032 — Kontrak release UI-only

Status: kontrak, validator, authoring/review persisten, API dan menu portal diimplementasikan. Assignment UI dan launch umum belum tersambung. Detail: `docs/ui-release-api.md`.

## Keputusan

Gunakan `emisell.ui-release/v1`, policy `reviewed-ui/v1`, terpisah dari release shipping. V1 memuat app/developer ID, versi numerik semver, nama, ringkasan, URL dan mode eksplisit `embedded` atau `external`. Pricing hanya `free`. Tidak menyediakan scope bisnis, capability, credential, callback OAuth ataupun parent origin dari developer. Unknown fields ditolak oleh decoder; tidak boleh menghapus capability shipping untuk mengonversi release lama.

Canonical JSON bertipe ditandatangani Ed25519 dengan domain `emisell.ui-release/v1` dan key ID trust-domain tersendiri. Verifikasi mencocokkan checksum, key dan signature. Signature release ini bukan approved launch signature, bukti kontrol endpoint, keputusan desain, consent merchant atau grant.

URL wajib HTTPS DNS hostname, port default/443, tanpa userinfo/query/fragment. Validator statis tidak melakukan network probe: DNS publik, endpoint ownership, CSP frame-ancestors dan availability tetap harus diuji pada pipeline app-client/review. Parent origin berasal dari konfigurasi Core tepercaya, bukan manifest. UI-only tidak berarti akses universal ke data toko.

## Validasi developer sekarang

Jalankan `go run ./cmd/cli ui-release-validate < examples/ui-release/embedded.json`. Perintah murni membaca stdin terbatas 8 KiB dan menghasilkan validitas, mode serta checksum canonical. Tidak membaca database/credential, tidak melakukan signing atau publish, dan selalu melaporkan `installable:false`.

Untuk external, ganti mode menjadi `external` pada dokumen baru. External bukan SSO; UI Kit wajib bagi embedded sesuai ADR 0030. Extension tanpa dashboard tidak memakai kontrak ini dan tetap di Settings.

## Penyambungan selanjutnya (belum tersedia)

1. Persist authoring UI-only dengan app identity milik organisasi dari sesi developer, version/revision, review dan audit idempotent. Jangan mempercayai developerId/appId dari contoh CLI sebagai authorization.
2. Persist release signed immutable; app-client memakai adapter release-kind eksplisit. Reuse endpoint proof, bukan bypass lokal untuk app umum.
3. Perluas assignment existing dengan jenis `ui`, FK sumber tepat satu, merchant/digest immutable, status requested/approved/rejected/revoked. Approval assignment tetap terpisah dari consent.
4. Source adapter menahan release → assignment → client → launch locks sebelum callback installation. Verifikasi source UI-only, current signature, current assignment serta launch URL/mode yang identik; jangan hanya memeriksa checksum release di launch.
5. Baru sambungkan RPC/Core/portal dan lakukan pengujian persisten serta browser end-to-end. Release signed saja tidak cukup untuk membuka Open app.

Increment kontrak awal tidak mengubah DDL. Increment berikutnya menambahkan migration 0019 untuk snapshot UI, receipt dan audit; hanya diterapkan di database test terisolasi. Tidak ada key provisioning, restart server pengguna atau perubahan instalasi toko. Kontrak lama tetap identik. Deployment general UI belum diizinkan oleh keberhasilan validator/API review.
