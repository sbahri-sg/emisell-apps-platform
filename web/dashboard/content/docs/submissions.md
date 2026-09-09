## Draft dan snapshot

Buat aplikasi, lengkapi metadata, lalu simpan draft. Pengajuan review menyimpan snapshot versi. Mengubah draft setelah pengajuan tidak mengubah snapshot yang sedang ditinjau.

Pantau status dan feedback di [Pengajuan](/development?view=reviews). Persetujuan metadata, penandatanganan rilis UI/integrasi, dan publikasi merupakan tahapan berbeda.

## Siapkan konfigurasi aplikasi

Pilih jenis rilis sesuai aplikasi. Konfigurasi client, proof endpoint, dan launch harus mengikuti rilis yang benar. Jangan menukar App ID atau memakai secret milik aplikasi lain.

## Alur pengujian toko

1. Siapkan versi/rilis dengan scope yang diperlukan.
2. Tunggu review dan persetujuan konfigurasi yang dibutuhkan.
3. Ajukan penugasan ke toko pengujian menggunakan merchant ID, bukan slug URL toko.
4. Setelah penugasan disetujui, seller membuka Settings → Apps.
5. Seller meninjau izin lalu memilih Install.
6. Seller membuka aplikasi dan menguji resource yang telah diizinkan.

> Penugasan tidak memasang aplikasi otomatis. Rilis yang ditandatangani bukan persetujuan seller dan bukan grant data toko.

## Apa yang perlu diuji

- Baca data hanya dari toko yang memasang aplikasi.
- Pencarian, halaman berikutnya/sebelumnya, detail, dan hasil kosong.
- Token tidak valid atau kedaluwarsa.
- Scope tidak tersedia, izin dicabut, dan instalasi tidak aktif.
- Backend tidak tersedia tanpa mengembalikan data contoh sebagai hasil nyata.

Gunakan `TESTING.md` hasil generate untuk langkah teknis sesuai versi template.

## Selesaikan pengujian

Stop testing mencabut penugasan dan menghentikan akses yang bergantung pada penugasan tersebut. Ini tidak otomatis menghapus instalasi. Gunakan Uninstall secara terpisah jika aplikasi juga perlu dihapus dari toko.
