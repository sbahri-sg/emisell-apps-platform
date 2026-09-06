# ADR 0019 — Identitas merchant tunggal pada kontrak API

Status: diterima, 2026-09-05. Keputusan pengguna: integrasi cukup membawa merchant ID, tidak memerlukan tenant ID terpisah. Mengoreksi nomenklatur pada panduan sebelumnya; tidak menghapus isolasi data.

## Kontrak dan kompatibilitas

1. Request/response utama memakai `merchantId`, yaitu ID merchant Emisell yang sama. Actor tetap field terpisah karena mengidentifikasi staf, bukan toko. `AccessContext` tetap mengikat installation/app/scope profile/grant revision; tidak ada bypass authorization.
2. Protobuf menambahkan `merchant_id` dengan field number baru pada pesan yang sebelumnya memakai `tenant_id`. Field lama ditandai deprecated, tidak dihapus atau diganti nomor/tipe. Tidak mengubah baseline Buf. Go bindings lama dan payload legacy tetap dapat dipakai.
3. RPC adapter menormalisasi alias sebelum use case dan ketika membuat response. Blank tetap blank; hanya use case yang menentukan kewajiban merchant context. Nilai berbeda ditolak `invalid_argument`. Nilai sama tidak membuat dua entitas. Domain/database tetap menggunakan nama internal existing sehingga checksum/signature, consent digest, audit dan idempotency receipt tidak berubah.
4. Response RPC v1 menyertakan kedua ejaan bila identitas ada demi kompatibilitas reader lama. Ini alias bernilai sama, bukan dua ID wajib. Dokumentasi utama mengecualikan field deprecated dari schema dan contoh; unduhan Protobuf tetap byte-identik dengan sumber dan menandai alias secara eksplisit. Consumer tidak boleh melakukan exact-object validation yang menolak field additive Protobuf.
5. GET `/api/v1/app/installation-access` memakai header `X-Emisell-Merchant-ID`. Header lama diterima sebagai fallback; konflik atau header identity berulang ditolak. Caller canonical mendapat response `merchantId` tanpa `tenantId`; caller yang hanya memakai header lama tetap mendapat DTO lama. Security gate token/installation/release/revocation tetap sama.
6. Gateway produk masih handoff/Plan. Validator membaca satu resolved merchant ID tanpa mengubah request; conformance suite mengirim canonical merchantId saja, termasuk pengujian isolasi dan cursor. Handler Core masa depan harus memakai identitas terverifikasi/resolved, bukan membaca field legacy kosong atau mempercayai request tanpa grant verification.
7. Tiga operasi pengelolaan API key tenant-bound legacy dikeluarkan dari referensi utama, bukan dihapus dari server. API key full-access tetap rekomendasi, tanpa binding merchant sintetis. Dokumentasi Admin menjadi 66 operasi dari 16 sumber. Tidak ada perubahan akun/key atau frontend merchant.

## Rollout / rollback

Deploy backend yang memahami field additive terlebih dahulu, kemudian docs/caller baru. Caller lama tetap dapat mengirim tenantId dan mendapat alias response. Caller baru tidak wajib mengirim alias. Binary lama akan mengabaikan field additive yang belum dikenal, sehingga caller merchant-only tidak boleh diarahkan ke binary lama; rollback harus memindahkan caller ke versi kompatibel dulu, tanpa mengganti ID/nilai.

Tidak ada migration database atau rename tabel/kolom. Kebutuhan referensi merchant terdaftar tetap berlaku dan berbeda dari input tenant ID tambahan. Tahap ini tidak menambahkan provisioning, integrasi live api-service, OAuth exchange atau gateway resource aktif. Instalasi testing tetap memakai reference fixture existing.

## Verifikasi

- Buf format/lint/breaking tanpa menonaktifkan aturan compatibility.
- ProtoJSON merchantId/merchant_id, alias legacy, konflik, nested message, dan blank context.
- Request HTTP JSON merchant-only → Prepare/Get/Decide/Consume/Activate/GetInstallation/IssueToken/payment/Uninstall; replay key yang sama lewat alias legacy tidak membuat consent baru.
- Self-check canonical/legacy/conflict, foreign merchant dan token setelah uninstall; database hanya test terisolasi.
- Gateway validator + conformance, generated docs drift, schema/example bebas field tenant ID, typecheck/test/lint/build frontend.

Nama field baru bukan bukti scope Active. Seluruh 108 resource scope tetap Plan; tidak ada probe Core atau akses data produksi dalam perubahan ini.
