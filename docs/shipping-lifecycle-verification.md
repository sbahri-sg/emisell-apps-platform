# Verifikasi lifecycle shipping lokal — 6 September 2026

## Hasil

`TestManagedShippingAssignments` memakai database Platform `emisell_local_test`, merchant sintetis, server RPC sementara dan binary API-Kurir `apps/local-managed` dengan database lokal terpisah. Tidak mengubah instalasi, provider aktif, credential atau layanan toko pengguna. Tarif pada harness ini adalah sample, bukan panggilan provider eksternal.

Skenario yang lulus:

- Signed assignment saja tidak memberi akses; consume sebelum consent ditolak.
- Instalasi pending belum memberi grant tarif. Aktivasi tidak memilih provider checkout.
- Setelah tarif berhasil, Platform mengembalikan 503: permintaan tarif identik ditolak 503, bukan dilayani tanpa grant. Pemulihan Platform mengembalikan keberhasilan.
- Uninstall menolak permintaan tarif identik (403). Replay consume lama tetap uninstalled.
- Install ulang menghasilkan intent dan installation ID baru; tetap memerlukan consent dan activation sebelum tarif kembali tersedia.
- Uninstall installation lama tidak mencabut replacement yang aktif.
- Release suspended menolak tarif; uninstall tetap dapat dilakukan.

Tes Core `shipping-platform-grant.test.mjs` juga lulus untuk grant fresh, mismatch merchant, revoke, outage dan urutan pemeriksaan sebelum cache. Paket API-Kurir `internal/enginegrant`, `internal/rates`, `internal/httpapi` lulus. Ini bukti terpisah, bukan pengujian end-to-end API-Kurir utama dengan grant lifecycle nyata pada seluruh endpoint.

## Antarmuka

Create order kini membedakan kegagalan memuat tarif dari hasil kosong, dengan pesan penolakan akses/gangguan layanan dan tombol coba lagi. Daftar provider membedakan provider aktif, konfigurasi provider dan ketersediaan engine; tidak mengklaim grant aplikasi dari flag provider. Installation tetap dikelola melalui Settings → Apps.

## Batas

Tidak ada shipment, booking, pickup, credential RajaOngkir, perubahan konfigurasi produksi atau penghentian server pengguna. Revocation lifecycle terhadap seluruh jalur API-Kurir utama, validasi UI error secara browser dan kesiapan produksi tetap membutuhkan verifikasi berikutnya. Tidak menaikkan resource scope Planned menjadi Active.
