## Kelola melalui dashboard

Buka [dashboard developer](/development), pilih aplikasi, lalu **App settings**. Client ID dan Secret dibuat bersama aplikasi, tanpa menunggu rilis ditandatangani atau endpoint diverifikasi. Client ID tetap sama ketika versi, nama, atau endpoint aplikasi berubah.

Secret disembunyikan secara default. Gunakan tombol mata untuk menampilkan, tombol salin untuk menyalin, atau **Rotate** untuk mengganti Secret. Tampilan Secret ditutup setelah satu menit, ketika tab disembunyikan, atau ketika meninggalkan halaman. Secret hanya boleh disimpan pada backend aplikasi, bukan kode browser atau Git.

Credential mengenali aplikasi, bukan memberikan akses toko. Instalasi, persetujuan seller, scope, dan pemeriksaan rilis tetap diperlukan. Pemeriksaan identitas backend tersedia melalui `POST /api/v1/app/client-check` dengan HTTP Basic menggunakan Client ID dan Secret; ini bukan endpoint penerbit token OAuth.

## Kompatibilitas integrasi lama

Client `eac_` yang terikat rilis tetap tersedia pada bagian **Kompatibilitas client rilis lama**. Client utama `eai_` tidak menggantikan client rilis pada adapter pengujian lama secara otomatis. Jika adapter masih meminta proof atau client rilis, gunakan credential rilis yang sudah dikonfigurasi sampai adapter tersebut dimigrasikan.

## Identitas dan rahasia

| Nilai | Penggunaan |
| --- | --- |
| App ID | Identitas aplikasi terdaftar |
| Client ID | Identitas client aplikasi |
| Client secret | Rahasia untuk backend confidential client |

Client secret aplikasi bukan Core key, key API kurir, atau cookie akun seller. API key internal platform tidak boleh dikirim ke aplikasi embedded.

## Konfigurasi pengujian lokal

Berikut placeholder untuk adapter lokal bawaan template, bukan konfigurasi produksi:

```dotenv
EMISELL_LOCAL_CORE_ORIGIN=http://127.0.0.1:8000
EMISELL_LOCAL_APP_ID=APP_ID
EMISELL_LOCAL_CLIENT_ID=CLIENT_ID
EMISELL_LOCAL_CLIENT_SECRET_FILE=/absolute/private/path/client.secret
```

Gunakan ID sebenarnya milik aplikasi. File secret harus berada di luar source/aset browser, berupa file biasa, bukan symlink. Pada macOS/Linux, batasi izin ke pemilik dengan mode `600`. Jangan commit nilainya ke Git.

Ikuti `.env.example` dan `TESTING.md` dari project Anda; restart development server setelah perubahan konfigurasi bila diperlukan.

## Rotasi dan pencabutan

Jika Secret terpapar, pilih **Rotate** dan konfirmasi. Secret lama langsung tidak berlaku, sedangkan Client ID tetap sama. Tampilkan atau salin Secret baru, perbarui backend aplikasi, dan uji kembali. Rotasi dicatat pada audit; jangan menyalin Secret ke tiket dukungan atau log diagnosis.

Credential baru tidak menambah scope instalasi. Penambahan izin tetap mengikuti [review dan persetujuan seller](/docs/submissions).
