## Minta izin minimum

Deklarasikan scope sesuai kebutuhan pada versi yang diajukan. Backend menentukan kesiapan endpoint dan akses aktual; menambahkan nama scope ke konfigurasi saja tidak memberi izin.

## Resource baca dalam template

| Scope | Data yang dibaca |
| --- | --- |
| `read_products` | Daftar, pencarian, dan detail produk |
| `read_orders` | Daftar, filter status, dan detail item pesanan |
| `read_shipping` | Konfigurasi, profil, dan nama zona pengiriman |
| `read_catalogs` | Daftar katalog dan referensi produk, bukan harga khusus |
| `read_collections` | Daftar, pencarian, dan referensi produk dalam koleksi |
| `read_inventory` | Persediaan tanpa mengubah jumlah stok |
| `read_locations` | Nama dan status lokasi, tanpa alamat atau telepon |

Scope pengiriman ini bukan izin membuat shipment, pickup, cek tarif, atau membuka credential kurir. Scope baca tidak mengizinkan perubahan data.

## Deklarasi pada rilis UI

```json
{
  "requiredScopes": ["read_orders", "read_products"]
}
```

Urutkan scope menurut alfabet, tanpa duplikasi. Aplikasi lama tidak otomatis memperoleh izin baru. Pengajuan rilis dan persetujuan seller diperlukan sesuai alur platform.

## Endpoint bisnis yang digunakan

Adapter resource menggunakan endpoint Emisell yang sudah ada. Contoh yang dipakai untuk produk, stok, dan lokasi:

| Resource | Endpoint backend Emisell | Scope |
| --- | --- | --- |
| Produk | `GET /v1/products` dan `GET /v1/products/:id` | `read_products` |
| Stok | `GET /v1/products?view=inventory` dan `GET /v1/products/:id?view=inventory` | `read_inventory` |
| Lokasi | `GET /v1/settings/location` dan `GET /v1/settings/location/:id` | `read_locations` |

Ini adalah pemetaan adapter backend, bukan instruksi memanggil API langsung dari browser atau memakai base URL Apps Platform untuk endpoint bisnis. Parameter, DTO, dan autentikasi mengikuti kontrak backend yang tersedia di lingkungan Anda.

## Pencarian dan pagination

Gunakan cursor dari respons, jangan mengarang nilainya atau memakainya untuk toko/filter lain. Mulai kembali dari halaman pertama ketika filter berubah. Template tidak memberi izin membuka detail resource lain hanya karena sebuah respons memuat ID referensinya.

> Daftar scope yang terdokumentasi bukan bukti seluruh scope sudah diaktifkan pada server produksi. Periksa rilis, instalasi, dan readiness lingkungan sebelum menguji data.
