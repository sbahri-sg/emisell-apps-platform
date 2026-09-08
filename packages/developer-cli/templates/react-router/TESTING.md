# Product Reader — uji lokal

Untuk Stok/Lokasi, gunakan alur review dan instalasi yang sama dengan rilis baru
berisi `"requiredScopes": ["read_inventory", "read_locations"]` (atau tambah
`read_products` bila halaman Produk juga diperlukan). Buka menu **Stok** dan **Lokasi**:
uji baca, Berikutnya, detail, dan pencarian nama lokasi. Tidak ada pencarian nama
produk pada scope inventory. Periksa stok negatif/lokasi nonaktif jika tersedia;
jangan mengubah stok toko hanya untuk testing. Setelah izin dicabut, pembacaan baru
harus ditolak. Instalasi lama tidak otomatis mendapat kedua scope tersebut.

Halaman Produk pada template React Router memakai `read_products`: daftar produk, pencarian nama/SKU, dan pagination cursor. Tidak mengubah produk atau membaca order. Tidak ada polling. Akses aktual diperiksa ulang backend setiap permintaan.

## Konfigurasi backend

Memerlukan api-service dan Apps Platform versi yang mendukung reviewed resource UI lokal. Adapter ini **hanya untuk pengembangan lokal**, bukan OAuth/server produksi. `app dev` memakai `server/backend.mjs`; tanpa konfigurasi yang valid akses ditolak.

Set environment di terminal (ganti placeholder dengan ID milik aplikasimu):

```sh
export EMISELL_LOCAL_CORE_ORIGIN=http://127.0.0.1:8000
export EMISELL_LOCAL_APP_ID=APP_ID
export EMISELL_LOCAL_CLIENT_ID=CLIENT_ID
export EMISELL_LOCAL_CLIENT_SECRET_FILE=/absolute/private/path/client.secret
emisell app dev
```

Simpan **hanya secret confidential client** dalam file privat tersebut, bukan di `public/`, `emisell.app.json`, manifest, argumen CLI, atau Git. Gunakan file biasa (bukan symlink), path absolut, izin `600` pada macOS/Linux, di direktori privat. Backend membacanya setiap permintaan agar rotasi tidak memerlukan perubahan UI. Jangan memakai API key platform atau cookie seller sebagai client secret.

Preview mendengarkan `127.0.0.1:4330`. Konfigurasi domain HTTPS lokal dan sertifikat terpercaya dilakukan operator; CLI tidak membuat tunnel, DNS atau sertifikat. Proxy aplikasi harus meneruskan Host upstream `127.0.0.1:4330`. Di `emisell.app.json`, set `appOrigin` ke domain HTTPS `.test` aplikasi; `parentOrigin` harus origin Dashboard seller yang berbeda. Contoh: `https://app.emisell.test` dan `https://seller.emisell.test`.

## Buat → review → testing → install

1. Login CLI sebagai developer: `emisell login --url http://localhost:4317 --email developer@example.com`, lalu `emisell whoami`. Gunakan akun dan origin portal yang benar; CLI meminta password secara interaktif.
2. Buat dokumen `product-ui.json` berikut, sesuaikan URL dengan HTTPS lokal yang sudah disiapkan:

```json
{
  "name": "Product Reader",
  "summary": "Membaca produk toko untuk uji integrasi",
  "version": "0.1.0",
  "mode": "embedded",
  "url": "https://app.emisell.test/",
  "requiredScopes": ["read_products"],
  "reason": "Uji pencarian dan pagination produk"
}
```

3. Ajukan rilis: `emisell resource-ui create --file product-ui.json --request-key product-ui-001 --yes`. Catat `appId` dan ID rilis. Admin meninjau dan menandatangani rilis; CLI tidak menyetujui sendiri.
4. Di portal, buat confidential app client untuk aplikasi/rilis itu, selesaikan endpoint proof dan persetujuan launch. Salin dokumen proof publik persis dari portal ke `endpointProof: {"id": "PROOF_ID", "document": {...}}` di konfigurasi lokal, lalu jalankan ulang preview. Jangan mengarang challenge atau tanda tangan. Masukkan app/client ID dan secret privat ke konfigurasi backend di atas.
5. Ajukan assignment dengan **ID merchant**, bukan slug URL toko:

```sh
emisell testing request --release-kind ui_resource --release-id RELEASE_ID --merchant-id MERCHANT_ID --reason "Uji baca produk" --request-key product-testing-001 --yes
emisell testing list
```

6. Admin menyetujui assignment. Seller membuka **Settings → Apps → Test apps assigned to this store**, meninjau izin `read_products`, lalu memilih **Install**. Rilis yang ditandatangani dan assignment sendiri **bukan persetujuan seller**.
7. Seller memilih **Open app → Baca produk**. Uji nama/SKU, Berikutnya, Sebelumnya, dan hasil kosong. Pencarian dilakukan di backend, bukan hanya lima baris di layar; cursor terikat toko, instalasi, aplikasi dan filter. Ulangi dari halaman pertama jika produk berubah saat paging.
8. Setelah testing selesai, **Stop testing** mencabut assignment dan menghentikan akses yang bergantung padanya. Ini tidak menghapus instalasi; gunakan **Uninstall** secara terpisah bila diperlukan.

Source frontend dan aset Vite diperlukan untuk hot reload; backend dan file konfigurasi privat tidak disajikan. Harga ditampilkan tanpa simbol mata uang karena DTO saat ini belum mengirim mata uang. Pencarian maksimum 100 byte UTF-8. Jangan menaikkan scope atau memakai identitas browser sebagai izin akses tanpa jalur backend yang telah ditinjau.
