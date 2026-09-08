# Emisell App — React Router

Satu template React Router Framework Mode, TypeScript, Vite, dan backend Node.
UI koneksi dan contoh pembaca produk tersedia tanpa mengganti protokol Emisell.
Memerlukan Node.js **22.12+**. Project ini tidak memasang aplikasi atau memberi izin toko otomatis.

## Mulai

```sh
npm install
emisell app doctor
emisell app dev
```

Tanpa CLI global, `npm run dev` menjalankan server yang sama pada
`http://127.0.0.1:4330`. Ubah halaman di `app/routes/`; Vite memperbarui UI.
Untuk port lain: `emisell app dev --port 4331`. CLI tidak mematikan proses existing.
Menjalankan dev berarti mengeksekusi source/config project tepercaya.

```sh
npm run typecheck
npm run build
npm start
```

Tanpa `NODE_ENV=production`, `npm start` menguji hasil build pada loopback.
Untuk Docker, jalankan `npm run docker:preview`. Untuk hosting dengan reverse proxy
HTTPS dan adapter backend production, ikuti **DEPLOYMENT.md**. Default akses toko
tetap ditolak; image Docker tidak mengaktifkan API/consent otomatis.

## Struktur

- `app/routes/home.tsx`: halaman koneksi.
- `app/routes/products.tsx`: baca produk, pencarian nama/SKU dan pagination.
- `app/routes/resources.tsx`: pesanan, pengiriman, katalog, koleksi, stok, dan lokasi dengan scope terpisah.
- `app/lib/emisell.ts`: helper browser untuk meminta identitas baru dari parent dan memanggil backend same-origin.
- `app/lib/bridge.mjs`: snapshot protokol embedded Emisell; tidak menyimpan bearer.
- `server/backend.mjs`: konfigurasi/adapters backend. Credential tidak diimpor ke UI.
- `server/http.mjs`: validasi Host, Origin, input, identitas, pembatasan data dan CSP.
- `emisell.app.json`: metadata publik project dan origin seller tepercaya, bukan tempat secret.
- `Dockerfile`, `.dockerignore`, `compose.yaml`: kemasan dan preview container.
- `compose.production.yaml`, `.env.production.example`: konfigurasi hosting terpisah.
- `server/production-backend.mjs`: tempat menyambungkan kontrak backend production,
  sengaja menolak startup mode custom sampai diimplementasikan dengan layanan nyata.

Navigasi internal memakai `Link`/`NavLink` React Router. Contoh API mengirim identitas
melalui Authorization, bukan query URL, cookie seller atau localStorage.
Identitas yang valid bukan grant: backend memeriksa izin terkini sebelum membaca data.

## Hubungkan backend lokal

Salin `.env.example` menjadi `.env`, batasi akses file ke pemilik (`chmod 600 .env`).
Isi sesuai aplikasi dan confidential client yang telah ditinjau:

```dotenv
EMISELL_LOCAL_CORE_ORIGIN=http://127.0.0.1:8000
EMISELL_LOCAL_APP_ID=APP_ID
EMISELL_LOCAL_CLIENT_ID=CLIENT_ID
EMISELL_LOCAL_CLIENT_SECRET_FILE=/absolute/private/path/client.secret
```

File terakhir berisi hanya secret `eacs_` milik confidential app client, bukan Core key,
API-Kurir key atau cookie seller. Simpan di luar `public/`, dengan izin `600`, bukan symlink.
Secret dibaca ulang setiap permintaan. Jangan memasukkan secret ke Git, `app/`, `public/`,
`emisell.app.json`, atau variabel `EMISELL_PUBLIC_*` (prefix ini dapat masuk bundel browser).

Hanya empat setting backend di atas yang dibaca dari `.env`; environment terminal
mengambil prioritas. `.env` tidak diteruskan ke Vite dan tidak mengubah process.env.
Tanpa konfigurasi: UI preview dapat dibuka, `/api/session` dan `/api/products` menolak
akses. Konfigurasi sebagian/salah tidak memberi sukses palsu. Restart setelah mengubah env.
`app doctor` memeriksa file dan dependency saja; tidak membaca `.env`, menjalankan backend,
menghubungi server, atau menyatakan izin toko aktif.

Adapter khusus dapat dimuat secara eksplisit dengan `emisell app dev --backend ./my-backend.mjs`.
Path itu relatif terhadap terminal; modul wajib mengekspor `verifySession` dan opsional
`readProducts`. Gunakan source tepercaya. Verifier Ed25519 tersedia di `server/identity.mjs`;
lihat `server/IDENTITY.md` untuk kontrak konfigurasi dan pemeriksaan izin.

## Testing dari seller

Ikuti **TESTING.md**: rilis `read_products` → review → client/launch → assignment
(`emisell testing request --release-kind ui_resource`) → persetujuan seller → install.
Template tidak menambahkan izin otomatis. Untuk demo stok/lokasi, ajukan rilis baru
dengan `"requiredScopes": ["read_inventory", "read_locations"]`, lalu lakukan review,
assignment dan persetujuan seller. Tidak ada izin menulis. Tidak ada polling; data lama
dibersihkan saat izin diperiksa kembali. Harga tidak mengasumsikan mata uang.

Stok memakai URL produk yang sudah ada dengan `view=inventory` (daftar/detail).
Lokasi memakai `/v1/settings/location` (daftar/detail, pencarian nama).
Stok hanya saldo `available` lokasi aktif, termasuk nilai negatif dan saldo nol;
flag pelacakan stok tidak diubah menjadi jumlah tak terbatas. Maksimal 100 varian
dan 100 baris saldo per produk; data yang lebih besar ditolak, bukan dipotong diam-diam.
Lokasi hanya nama/status, tanpa alamat/telepon dan tanpa membuat lokasi default.
Menu ini tidak memberikan grant dan adapter bawaan tetap untuk pengujian lokal.

Untuk proxy HTTPS lokal, set `appOrigin` di `emisell.app.json`, misalnya
`https://app.emisell.test`, terpisah dari `parentOrigin`. Proxy harus mendukung WebSocket
untuk hot reload dan meneruskan Host upstream `127.0.0.1:4330`.
Jangan expose server Vite secara publik. Path `.env`, `.local`, `server/`, `.server.*`,
file key dan akses filesystem mentah ditolak. Letakkan hanya aset publik di `public/`.

Template HTML lama tidak dihasilkan lagi. Project existing tidak ditimpa atau dimigrasikan
diam-diam; pindahkan hanya UI/logika yang diperlukan ke project baru. CLI 0.3.1 tetap dapat
digunakan secara eksplisit untuk project lama bila diperlukan.
