## Mulai dari lokasi kegagalan

Bedakan browser aplikasi, backend aplikasi, API-service Emisell, dan Apps Platform. Catat waktu serta status respons yang gagal tanpa menyertakan password, cookie, token, atau secret.

Jangan menonaktifkan verifikasi untuk membuat status terlihat berhasil. Periksa versi layanan, base URL, dan kemampuan endpoint yang benar-benar berjalan.

## Settings Apps tidak terhubung

Pastikan Dashboard menunjuk ke API-service yang sudah memuat integrasi Apps Platform. Jika pesan menyebut endpoint koneksi tidak tersedia, periksa branch/versi dan alamat API-service terlebih dahulu. Ini tidak otomatis berarti merchant belum terdaftar.

Jika koneksi sudah siap tetapi toko belum tersedia, periksa sinkronisasi identitas merchant di backend. Slug URL toko bukan pengganti merchant ID.

## Installation unavailable

Periksa rilis yang dipilih, scope, konfigurasi client/launch, persetujuan penugasan toko, dan readiness instalasi. Rilis yang sudah ditandatangani belum berarti semua tahapan selesai.

Ikuti [alur review dan instalasi](/docs/submissions#alur-pengujian-toko), lalu minta seller meninjau izin sebelum memasang aplikasi.

## Identitas belum terverifikasi

Periksa origin aplikasi dan parent, konfigurasi client, waktu berlaku token, serta koneksi verifier backend. Jangan menggunakan Core key atau cookie seller sebagai client secret aplikasi.

Lihat [credential aplikasi](/docs/credentials#konfigurasi-pengujian-lokal) untuk konfigurasi adapter lokal.

## Produk atau resource ditolak

Periksa apakah instalasi memiliki scope yang diminta dan masih aktif. Aplikasi lama dengan izin produk tidak otomatis bisa membaca orders, stok, atau lokasi. Untuk filter baru, gunakan cursor baru dari halaman pertama.

## Respons 503 pada container

Container preview UI tidak membuka data toko. Pada produksi, backend yang dinonaktifkan atau adapter yang belum siap juga dapat menghasilkan 503. Periksa mode backend dan readiness, bukan mengganti respons error dengan data palsu.

## Port sudah digunakan

Gunakan port lain pada development:

```sh
emisell app dev --port 4331
```

Hentikan hanya proses preview milik project yang diketahui. Jangan mematikan layanan lain untuk mengosongkan port.
