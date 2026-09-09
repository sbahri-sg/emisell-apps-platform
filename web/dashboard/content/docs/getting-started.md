## Sebelum mulai

Gunakan **Node.js 22.12 atau lebih baru** untuk project hasil generate. Template menggunakan React Router, TypeScript, dan Vite. Klik **Login**, lalu gunakan akun merchant Emisell yang memiliki izin mengelola aplikasi. Profil developer dibuat otomatis dari identitas akun Emisell; tidak perlu undangan, organisasi, atau kata sandi developer terpisah.

Jika sesi merchant masih aktif, portal masuk otomatis. Sesi developer berakhir setelah satu jam tanpa aktivitas, tanpa mengeluarkan akun merchant. Buka kembali [dashboard developer](/development) untuk masuk kembali melalui sesi merchant.

Membuat project lokal, mendaftarkan aplikasi, dan memasang aplikasi pada toko adalah langkah yang berbeda.

## Buat project lokal

```sh
npm install -g @emisell/cli@latest
emisell app init
```

Wizard meminta nama aplikasi, folder baru, dan origin Dashboard seller yang dipercaya. Folder yang sudah ada tidak ditimpa. `app init` tidak mendaftarkan aplikasi ke server dan tidak memberikan izin toko.

## Jalankan aplikasi

Masuk ke folder yang dipilih pada wizard, lalu jalankan:

```sh
npm install
emisell app doctor
emisell app dev
```

Buka alamat lokal yang ditampilkan CLI. Edit halaman di `app/routes/` untuk melihat perubahan melalui hot reload. Gunakan [panduan struktur template](/docs/template) untuk menemukan frontend dan backend.

## Daftarkan aplikasi

Buka [dashboard developer](/development), buat aplikasi, lalu lengkapi metadata versi dan izin yang diperlukan. Simpan draft sebelum mengajukannya untuk review. Credential dikonfigurasi untuk aplikasi/rilis yang sesuai, bukan dibuat dengan mengarang ID lokal.

## Uji dengan seller

Ikuti [review dan pengujian](/docs/submissions) untuk konfigurasi rilis, client, penugasan toko, persetujuan seller, dan instalasi. Mulai dengan izin paling kecil, misalnya `read_products` untuk demo produk.

> UI yang berhasil dibuka bukan bukti akses toko sudah aktif. Backend harus memverifikasi identitas dan izin pada setiap permintaan data.
