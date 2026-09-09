## Framework yang digunakan

Template menggunakan React Router + TypeScript + Vite dengan backend Node. Developer dapat memakai pola route dan komponen React yang familiar tanpa menduplikasi navigasi utama Dashboard seller.

## Struktur project

```text
app/
  routes/          Halaman koneksi dan pembaca resource
  lib/             Helper request dan bridge embedded
  root.tsx         Layout aplikasi
server/
  backend.mjs      Konfigurasi adapter lokal
  http.mjs         Validasi request HTTP
  identity.mjs     Verifier identitas
  production-backend.mjs
emisell.app.json   Metadata publik project
.env.example      Contoh konfigurasi privat
Dockerfile
TESTING.md
DEPLOYMENT.md
```

## Frontend dan backend

Frontend meminta data melalui backend aplikasi. Jangan memindahkan secret atau pemeriksaan izin ke komponen React. Backend memverifikasi identitas, binding toko/aplikasi/instalasi, dan grant terkini sebelum mengembalikan data.

Template menyediakan contoh produk, pesanan, pengiriman, katalog, koleksi, stok, dan lokasi. Menu yang tampil bukan pemberian scope; akses yang belum dikonfigurasi harus ditolak.

## Konfigurasi project

`emisell.app.json` menyimpan metadata publik seperti nama project dan origin, bukan tempat secret. File `.env.example` menjelaskan konfigurasi backend lokal. Simpan nilai rahasia di luar source frontend dan jangan commit `.env` berisi credential.

Ikuti [credential aplikasi](/docs/credentials) dan [aplikasi embedded](/docs/embedded-apps) sebelum menyambungkan data toko.

## Pemeriksaan sebelum perubahan dikirim

```sh
npm run typecheck
npm run build
```

Simpan lockfile dependency project. Perubahan template tidak menimpa aplikasi existing; bandingkan perubahan dan migrasikan project dengan pengujian yang sesuai.
