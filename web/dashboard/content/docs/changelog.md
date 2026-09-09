## Tentang catatan ini

Ringkasan berikut mengikuti CHANGELOG CLI dalam repository. Nomor versi di sini tidak membuktikan paket sudah diterbitkan ke npm atau server tertentu sudah diperbarui. Periksa versi CLI yang terpasang dengan `emisell --version`.

## CLI 0.4.0

- Satu template React Router + TypeScript + Vite menggantikan generator HTML lama.
- Tersedia perintah build, Docker multi-stage non-root, serta runtime web produksi dengan adapter eksplisit.
- Demo stok dan lokasi menggunakan endpoint produk/lokasi yang sudah ada serta scope `read_inventory` dan `read_locations`.
- Akses produksi tidak diaktifkan otomatis; review, konfigurasi adapter, dan persetujuan seller tetap diperlukan.

## CLI 0.3.1

Dokumentasi publik difokuskan pada fitur dan penggunaan Emisell. Tidak mengubah perilaku runtime maupun izin merchant.

## CLI 0.3.0

- Wizard `app init` untuk nama, folder, template, dan origin seller.
- Penemuan project dari folder aktif, serta alias `--path` dan `--dir`.
- `app info` untuk metadata aman dan `app doctor` untuk pemeriksaan lokal.
- Tidak menyediakan browser OAuth, tunnel, atau persetujuan seller otomatis.

## Memperbarui project

Update CLI tidak menghapus atau menimpa project existing. Bandingkan struktur template dan uji perubahan pada project Anda. Jangan menganggap update dependency sebagai penambahan scope atau migrasi data toko.
