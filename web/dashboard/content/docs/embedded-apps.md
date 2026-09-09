## Tampil di Dashboard seller

Aplikasi embedded tampil di dalam Dashboard seller. Gunakan navigasi untuk halaman aplikasi Anda sendiri; jangan menduplikasi seluruh menu Dashboard seller.

## Pisahkan origin

`parentOrigin` adalah origin Dashboard seller yang dipercaya. `appOrigin` adalah origin aplikasi. Untuk pengujian HTTPS lokal, keduanya dapat berupa domain test yang berbeda dengan sertifikat yang dipercaya komputer penguji.

```json
{
  "parentOrigin": "https://seller.emisell.test",
  "appOrigin": "https://app.emisell.test"
}
```

Ini contoh metadata origin, bukan konfigurasi lengkap. CLI tidak menyiapkan DNS, sertifikat, atau tunnel. Ikuti panduan project untuk proxy lokal dan Host upstream.

## Verifikasi melalui backend

Bridge meminta identitas dari parent yang diizinkan. Frontend meneruskannya ke backend aplikasi untuk verifikasi. Backend harus memeriksa signature dan binding; pesan yang diterima iframe tidak boleh langsung dianggap sebagai izin data toko.

Jangan menyimpan token dalam URL, cookie seller, atau penyimpanan persisten browser. Jangan mengambil lokasi verifikasi dari payload token yang belum dipercaya.

## Uji setelah instalasi

Setelah seller menyetujui izin dan memasang aplikasi, buka **Open app**. Uji koneksi, pembacaan resource yang diizinkan, pencarian, pagination, hasil kosong, dan penolakan setelah akses dicabut.

Identitas valid tanpa `read_products` tetap tidak boleh membaca produk. Baca [autentikasi dan otorisasi](/docs/authentication) untuk perbedaan identitas dan akses.
