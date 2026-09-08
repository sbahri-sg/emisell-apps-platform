# Emisell Developer CLI

## Demo baca data toko

Template React Router menyediakan halaman Produk, Pesanan, Pengiriman, Katalog,
Koleksi, Stok, dan Lokasi. Katalog/koleksi mendukung daftar, pencarian nama, pagination dan detail
referensi produk. Tidak membuka harga khusus katalog atau detail produk otomatis.
Pesanan mendukung daftar, filter status, halaman berikutnya, dan detail item;
Pengiriman membaca konfigurasi, profil, dan nama zona tanpa tarif atau kredensial
kurir. Backend demo menggunakan endpoint Emisell yang sudah ada.

Stok memakai `GET /v1/products?view=inventory` dan
`GET /v1/products/:id?view=inventory`, dengan izin `read_inventory`.
Saldo `available` hanya dari lokasi aktif; varian tidak dijumlahkan lagi dengan
stok induk, nilai negatif tetap ditampilkan. Tidak ada perubahan stok, harga,
atau alamat. Lokasi memakai `GET /v1/settings/location` dan
`GET /v1/settings/location/:id`, dengan izin `read_locations`: nama dan status,
tanpa alamat/telepon. Daftar mendukung pagination; lokasi juga pencarian nama.
Tidak dibuat endpoint bisnis `/inventory` atau `/locations` baru.

Pilih hanya izin yang dibutuhkan dalam rilis UI yang diajukan: `read_catalogs`,
`read_collections`, `read_inventory`, `read_locations`, `read_orders`, `read_products`, atau `read_shipping`
(urut alfabet, tanpa duplikasi). Contoh untuk katalog dan koleksi:
`"requiredScopes": ["read_catalogs", "read_collections"]`. Menu demo
bukan pemberian izin. Rilis harus ditinjau/ditandatangani dan seller harus
menyetujui izin sebelum instalasi aktif. Aplikasi lama dengan izin produk saja
tidak otomatis mendapat akses scope baru, termasuk rincian stok per lokasi.

Adapter bawaan ini untuk pengujian lokal. Deployment produksi memerlukan adapter
server yang terverifikasi; jangan mengaktifkan bypass izin atau memasukkan secret
aplikasi ke kode browser.

CLI untuk membangun aplikasi dan mengelola rilis di Emisell Apps Platform.
Versi **0.4.0** menggunakan satu template **React Router + TypeScript + Vite**,
dengan backend Node, halaman koneksi, serta contoh pembaca produk.

CLI memerlukan Node.js 22+ dan tidak memiliki dependency runtime, telemetry,
atau install script. **Project hasil generate memerlukan Node.js 22.12+**
dan memasang dependency framework tersendiri.

> Akses data toko tetap memerlukan backend yang dikonfigurasi, review, assignment,
> instalasi, serta persetujuan seller. Preview lokal bukan server produksi.

## Mulai cepat

```sh
npm install -g @emisell/cli@latest
emisell --version
emisell app init
cd my-emisell-app
npm install
emisell app doctor
emisell app dev
```

Wizard meminta nama, folder baru, dan origin Dashboard seller yang dipercaya.
Sesuaikan `cd` dengan folder yang dipilih. Buka `http://127.0.0.1:4330`;
edit halaman di `app/routes/` dan Vite memperbarui UI melalui hot reload.

Contoh tanpa prompt:

```sh
emisell app init --name product-reader --path ./product-reader --parent-origin https://seller.emisell.test
cd product-reader
npm install
emisell app info
emisell app dev
```

`--template react-router` opsional karena hanya ada satu template.
`app init` membuat project lokal, **bukan registrasi aplikasi di server**.
Folder existing tidak ditimpa. Dependency dipasang terpisah dengan `npm install`.

## Perintah development

| Perintah | Kegunaan |
| --- | --- |
| `emisell app init` | Buat project melalui wizard |
| `emisell app dev` | Jalankan Vite dan backend lokal, port default 4330 |
| `emisell app build` | Build project dari folder saat ini atau `--path` |
| `emisell app info --json` | Metadata aman; tidak membaca credential |
| `emisell app doctor --json` | Periksa metadata, file, Node dan dependency |
| `npm run typecheck` | Type generation dan pemeriksaan TypeScript |
| `npm run build` | Build frontend dan server React Router |
| `npm start` | Jalankan hasil build; production memerlukan env hosting terpisah |
| `npm run dev` | Alternatif development tanpa CLI global |
| `npm run docker:preview` | Build dan jalankan container UI lokal tanpa akses toko |
| `npm run docker:down` | Hentikan container preview project ini |

Jalankan perintah `npm` di folder aplikasi. `app dev/build/info/doctor` menemukan project
dari folder saat ini atau subfoldernya; `--path` memilih folder secara eksplisit.
`--dir` tetap menjadi alias `--path`. Opsi singkat: `-n`, `-p`, dan `-j`.
Lihat `emisell app init --help` atau bantuan perintah lainnya.

Doctor tidak menjalankan source/backend, membaca `.env` atau secret,
dan tidak menghubungi server. Exit 1 berarti ada masalah lokal.
Lulus doctor **bukan bukti izin seller atau koneksi produksi sudah aktif**.

Dev menjalankan source/config project tepercaya seperti `npm run dev`.
Pilih port lain dengan `--port 4331` jika terpakai; CLI tidak mematikan proses lain.
Ctrl+C menghentikan preview. Tidak ada tunnel atau deployment otomatis.

## Struktur project

```text
app/
  routes/home.tsx        Halaman koneksi Dashboard
  routes/products.tsx    Produk, pencarian, pagination
  lib/emisell.ts         Helper request browser ke backend aplikasi
  lib/bridge.mjs         Protokol embedded Emisell
  root.tsx              Layout dan navigasi
server/
  backend.mjs           Konfigurasi backend lokal
  http.mjs              Batas akses HTTP dan validasi request
  local-core.mjs         Verifikasi identitas melalui Core lokal
  local-products.mjs     Pembaca produk dengan pemeriksaan grant
emisell.app.json         Metadata publik project, bukan tempat secret
.env.example            Contoh konfigurasi privat
README.md               Panduan development
TESTING.md              Alur review sampai seller install
```

Navigasi menggunakan React Router; UI tidak menduplikasi menu Dashboard seller.
Produk mendukung pencarian nama/SKU, halaman berikut/sebelumnya, loading,
empty state, dan error akses. Harga ditampilkan sesuai API tanpa menebak mata uang.

## Hubungkan backend lokal

Salin `.env.example` ke `.env`, batasi izin file ke pemilik (`chmod 600 .env`),
lalu isi sesuai aplikasi dan confidential app client yang telah ditinjau:

```dotenv
EMISELL_LOCAL_CORE_ORIGIN=http://127.0.0.1:8000
EMISELL_LOCAL_APP_ID=APP_ID
EMISELL_LOCAL_CLIENT_ID=CLIENT_ID
EMISELL_LOCAL_CLIENT_SECRET_FILE=/absolute/private/path/client.secret
```

Ganti ID contoh dengan ID sebenarnya. Core origin harus HTTP literal
`127.0.0.1`, bukan endpoint produksi. File secret berisi confidential client
secret `eacs_`, bukan Core key, API-Kurir key, atau cookie seller.
Simpan file privat dengan izin 600, bukan symlink, di luar source/aset browser.

Backend bawaan membaca empat setting tersebut dari `.env`; environment terminal
lebih diprioritaskan. Vite tidak memuat file env otomatis pada template ini.
Jangan menaruh secret di `app/`, `public/`, Git, metadata, atau environment
`EMISELL_PUBLIC_*` yang dapat dibundel ke browser. Restart setelah mengubah env.

Tanpa backend, UI tetap bisa dilihat tetapi akses identitas/data ditolak.
Setiap pembacaan meminta identitas baru dan memeriksa izin terkini.
Identitas saja tidak memberi grant `read_products`; tidak ada izin order,
write, atau akses toko lain. Tidak ada token di URL/cookie seller/localStorage.

Adapter khusus boleh dimuat melalui `emisell app dev --backend ./my-backend.mjs`.
Modul tepercaya itu mengekspor `verifySession` dan opsional `readProducts`.
Path relatif terhadap terminal saat perintah dijalankan. Lihat `server/IDENTITY.md`
pada project untuk verifier Ed25519 dan kewajiban pemeriksaan binding/grant.

## Testing dari Dashboard seller

Alur: rilis → review → konfigurasi client/launch → assignment → persetujuan
seller → install → Open app. Panduan lengkap ada di `TESTING.md` hasil generate.

Untuk proxy HTTPS lokal, tambahkan `appOrigin: "https://app.emisell.test"` di
`emisell.app.json`, terpisah dari `parentOrigin`. Proxy harus mendukung WebSocket
dan meneruskan Host upstream `127.0.0.1:4330`. Jangan expose Vite secara publik.
Forwarded headers tidak menentukan Origin yang dipercaya.

Jika diperlukan verifikasi endpoint, tambahkan `endpointProof` berisi `id`
(`proof_…`) dan `document` empat field persis dari platform:
`schema`, `clientId`, `releaseSha256`, `challenge`.
Hanya path challenge tersebut yang disajikan. Restart setelah perubahan,
hapus challenge setelah verifikasi. Ini bukan persetujuan install.

## Akun developer

Akun Apps Platform dibuat administrator; berbeda dengan akun npm.
Login tidak diperlukan untuk generate/preview lokal.

```sh
emisell auth login --url https://portal.example.com --email developer@example.com
emisell whoami
emisell auth logout
```

Ganti origin portal contoh dengan alamat yang diberikan administrator.
Untuk portal lokal gunakan `http://localhost:4317`. HTTP hanya untuk loopback;
URL harus origin tanpa path, query atau credential. Tidak ada bypass TLS/redirect.

Password diminta tanpa echo. Tidak ada opsi `--password`.
`--password-stdin` hanya untuk pipe dari secret manager; jangan tulis password
literal di command atau repository. `login/logout` tetap menjadi alias lama.

Sesi disimpan di `~/.emisell-cli/session.json`, bukan folder aplikasi.
Password tidak disimpan; file sesi bukan keychain/enkripsi. Pada macOS/Linux
direktori harus 0700 dan file 0600; symlink ditolak. Windows bergantung pada ACL.
Jangan bagikan, commit, atau masukkan sesi ke backup publik.

Satu profil lokal; batas sesi mengikuti server, maksimal delapan jam.
Logout default mencabut sesi server. `emisell auth logout --local` hanya
menghapus salinan lokal ketika server tidak tersedia, bukan mencabut sesi server.

## Kelola rilis dan assignment

```sh
emisell scopes
emisell ui list
emisell ui show RELEASE_ID
emisell ui create --file ui.json --request-key ui-create-001 --yes
emisell resource-ui create --file resource-ui.json --request-key resource-create-001 --yes
emisell resource-ui list
emisell resource-ui show RELEASE_ID
emisell testing request --release-id RELEASE_ID --release-kind ui_resource --merchant-id MERCHANT_ID --reason "Pengujian baca produk" --request-key testing-001 --yes
emisell testing list
emisell testing list --after-id ASSIGNMENT_CURSOR
emisell testing show ASSIGNMENT_ID
```

Dokumen UI berisi `name`, `summary`, `version`, `mode` (`embedded` atau
`external`), `url` HTTPS, dan `reason`. Resource UI menambahkan
`"requiredScopes": ["read_products"]`. Template framework tidak mengubah mode
rilis atau kontrak API. Order/write scopes belum didukung template ini.

Default testing adalah `--release-kind ui`; rilis produk memakai `ui_resource`.
Gunakan ID merchant sebenarnya, bukan slug URL toko. Admin memutuskan assignment;
seller melakukan consent/install. Approved assignment bukan akses data.
Seller dapat Stop testing; Uninstall tetap tindakan terpisah.

## Draft shipping dan review

```sh
emisell apps list
emisell apps show APP_ID
emisell apps init --file app.json
emisell apps create --file app.json --request-key app-create-001
emisell apps update APP_ID --file app.json --revision 1 --request-key app-update-001
emisell reviews submit APP_ID --revision 2 --request-key app-review-001 --yes
emisell reviews list
emisell reviews show REVIEW_ID
```

`apps init` (plural) hanya membuat dokumen draft shipping, bukan project frontend.
Edit nama dan endpoint HTTPS sebelum submit. Gunakan revision terbaru dari API,
bukan angka contoh. Review bukan publish, signing, instalasi, atau izin seller.

Output API berupa JSON. Daftar mengikuti batas API; bukan jaminan seluruh halaman.
Request-key 8–128 karakter huruf/angka/`_`/`-`: key baru untuk operasi baru,
key sama hanya untuk operasi dan payload sama jika hasil perlu direkonsiliasi.
Tidak ada auto-retry mutasi. Error 401: login kembali; 403: periksa akses/domain;
409: baca revision terbaru; 429: tunggu lalu coba lagi.

## Perubahan dari template lama

Generator HTML `embedded` dan `products` sudah dihapus. Keduanya diganti satu
template React Router yang menyediakan koneksi dan produk. Project lama tidak
dihapus, ditimpa, atau dimigrasikan otomatis. Buat folder baru lalu pindahkan
UI/logika yang diperlukan; jangan menyalin credential ke source frontend.

Jika masih perlu menjalankan project lama, gunakan CLI lama secara eksplisit:
`npx @emisell/cli@0.3.1 app dev`. CLI baru akan memberi petunjuk migrasi
ketika membaca metadata lama, bukan menjalankannya dengan asumsi yang salah.

## Batas versi ini

Vite dan adapter Core bawaan tetap khusus lokal. Template kini menyediakan Dockerfile
multi-stage, Compose preview/production, health/readiness, shutdown terkontrol dan
runtime hosting melalui reverse proxy HTTPS. Ikuti `DEPLOYMENT.md` pada project.
Docker tidak diperlukan untuk `emisell app dev`. Runtime image tidak memerlukan CLI.

**Koneksi resource produksi belum aktif otomatis.** Mode backend `disabled` menyajikan
UI tetapi menolak API toko; readiness production 503. Mode `custom` wajib menggunakan
adapter nyata di `server/production-backend.mjs`; stub bawaan menolak startup. Jangan
membuka endpoint preview lokal ke internet. Hosting/DNS/TLS, adapter otorisasi
production, browser/device OAuth, pengelolaan secret via CLI, tunnel, `app deploy`,
dan worker webhook bukan fitur otomatis template. Tidak ada database/migrasi bawaan.
Install CLI tidak mengubah server, scope Plan, atau consent seller.
Emisell Kurir bawaan tidak dipindahkan atau diubah oleh template ini.
