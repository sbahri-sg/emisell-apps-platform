# Emisell Developer CLI

**0.2.0 — kandidat rilis lokal, belum dipublikasikan.** Versi npm yang diverifikasi saat pengerjaan ini masih 0.1.1; `@latest` belum memuat template produk di bawah. Kandidat memakai Node.js 22+, tanpa dependency runtime.

## Mulai aplikasi pembaca produk

Dengan CLI kandidat dari checkout atau tarball lokal:

```sh
emisell app init --dir product-reader --template products --parent-origin https://seller.emisell.test
```

Hasilnya UI produk dengan pencarian nama/SKU, pagination cursor, serta `local-products-backend.mjs`. Panduan lengkap create → review → app-client/launch → assignment → seller consent/install → read ada di **README.md yang dihasilkan**. Ikuti panduan konfigurasi backend tersebut sebelum menjalankan:

```sh
emisell app dev --dir product-reader --backend ./product-reader/local-products-backend.mjs
```

Simpan client secret hanya di file privat di luar `public/` (izin 600), lalu set path melalui `EMISELL_LOCAL_CLIENT_SECRET_FILE`. Secret tidak pernah masuk browser. Template hanya memakai `read_products`; identitas, assignment, instalasi dan izin terkini tetap diperiksa backend. Preview lokal **bukan adapter OAuth atau server produksi**.

## Submit an embedded UI app

### Experimental resource UI authoring

On a server explicitly configured with the resource UI authoring routes, the
same metadata plus `"requiredScopes": ["read_products"]` can be submitted with
`emisell resource-ui create --file resource-ui.json --request-key unique-001 --yes`.
Use `resource-ui list` and `resource-ui show RELEASE_ID` to inspect it.
These commands submit metadata only. On the locally configured resource runtime,
seller consent, installation and product reads follow review, client/launch and
assignment approval. Order and write scopes are rejected. These routes are
opt-in; this CLI candidate is not yet published.

Create a JSON file with `name`, `summary`, `version`, `mode` (`embedded` or
`external`), `url` (HTTPS), and `reason`. Then run:

```sh
emisell ui create --file ui.json --request-key my-ui-001 --yes
emisell ui list
emisell ui show RELEASE_ID
```

This submits a UI release for review; it does not sign, publish, assign, or
install the app. UI metadata does not request shipping or order scopes.
These commands are in the local development CLI, not the published 0.1.1 release.

## Local HTTPS preview

For an operator-configured local TLS proxy, set `appOrigin` in the generated
`emisell.app.json` to an exact HTTPS `.test` origin, e.g.
`https://app.emisell.test`, separate from `parentOrigin`. The proxy must forward
to the loopback preview and use its loopback Host header. Forwarded headers
never determine which Origin may call `/api/session`.

To serve an endpoint ownership challenge, add `endpointProof` with `id` (the
issued `proof_…` identifier) and `document` (the exact four-field JSON issued by
the platform: `schema`, `clientId`, `releaseSha256`, `challenge`). Only that exact
well-known path is served; no directories or configuration files become public.
Restart the preview after configuration changes. Remove the challenge after
verification. This option does not approve, install, or authorize an app.

CLI awal untuk akun developer Emisell Apps Platform. Memakai API portal yang sama dengan dashboard: organisasi, izin, revisi, validasi dan review tetap ditentukan backend. Tidak mengakses database langsung dan tidak memiliki perintah admin.

Paket **@emisell/cli**, command **emisell**. Memerlukan Node.js 22 atau lebih baru. Tidak ada dependency runtime, telemetry atau install script.

## Instalasi

Instal atau perbarui CLI dari npm:

```sh
npm install -g @emisell/cli@latest
emisell --version
emisell --help
```

Anda tidak memerlukan akun npm untuk memasang paket publik ini. Untuk mengelola aplikasi, Anda memerlukan akun developer **Apps Platform**, bukan akun npm. Tidak ada pendaftaran akun melalui CLI.

## Login

Gunakan akun **developer** yang dibuat administrator dan URL dashboard terpadu, bukan URL Dashboard seller/API-Kurir. Pastikan domain benar sebelum memasukkan password.

```sh
emisell login --url https://apps-platform.emisell.com --email developer@example.com
emisell whoami
```

Password diminta tanpa ditampilkan. Tidak ada opsi `--password` agar password tidak masuk riwayat shell/process list. Untuk otomasi tersedia `--password-stdin`; kirim lewat secret manager, jangan menulis password literal dalam command atau file repository.

Development lokal:

```sh
emisell login --url http://localhost:4317 --email developer@example.com
```

HTTP hanya diizinkan untuk loopback. Semua URL harus origin saja, tanpa `/api/v1`, path, query atau credential. CLI memakai Origin yang sama dengan URL login; backend harus sudah dikonfigurasi untuk origin tersebut. Redirect tidak diikuti, termasuk saat login. Tidak ada opsi mematikan verifikasi TLS.

## Melihat aplikasi dan scope

```sh
emisell apps list
emisell apps show APP_ID
emisell scopes
emisell reviews list
emisell reviews show REVIEW_ID
```

Output adalah JSON. Daftar mengikuti batas API (saat ini maksimal 200), bukan janji mengambil semua halaman. Status scope yang masih Plan tidak berubah menjadi aktif karena digunakan dari CLI.

## Membuat dan memperbarui draft

```sh
emisell apps init --file app.json
# Edit app.json: nama, ringkasan, deskripsi dan endpoint HTTPS aplikasi.
emisell apps create --file app.json --request-key create-app-001
emisell apps show APP_ID
emisell apps update APP_ID --file app.json --revision 1 --request-key update-app-001
```

`apps init` hanya membuat file baru dan tidak menimpa file yang ada. Ini **template dokumen draft shipping** yang didukung API saat ini, bukan generator frontend/embedded app atau server lokal. File berisi AppDocument langsung, tanpa pembungkus `document`, tanpa ID organisasi, revision atau API key provider. Validasi final dilakukan backend.

Contoh isi `app.json` (ganti endpoint dengan alamat HTTPS aplikasi Anda):

```json
{
  "name": "My Shipping App",
  "summary": "Integrasi pengiriman toko",
  "description": "Aplikasi pengiriman untuk toko Emisell.",
  "version": "1.0.0",
  "capability": "shipping/v1",
  "scopes": ["orders.read", "shipping.read", "shipping.write"],
  "endpoint": "https://your-app.example.com/shipping"
}
```

Gunakan `app.id` dari respons create sebagai `APP_ID`. Angka revision pada contoh update/review hanya ilustrasi; selalu gunakan `app.revision` terbaru. Endpoint contoh di atas bukan layanan yang disediakan Emisell.

Gunakan revision dari hasil `apps show` terakhir. Jangan otomatis menambah revision atau mengulang update ketika terjadi konflik: baca data terbaru dan rekonsiliasi perubahan terlebih dahulu.

Setiap mutasi wajib request-key unik (8–128 karakter huruf/angka/`_`/`-`). Jika koneksi putus dan hasil tidak diketahui, cek data/status terlebih dahulu. Untuk mengulangi **operasi dan payload yang sama**, gunakan key yang sama. Untuk operasi baru, gunakan key baru. CLI tidak melakukan auto-retry.

## Mengajukan review

```sh
emisell reviews submit APP_ID --revision 2 --request-key review-app-001 --yes
emisell reviews list
```

`--yes` mengonfirmasi pengiriman ke reviewer. Review tidak berarti publish, signing, install atau akses data seller. Semua tahapan lanjutan tetap mengikuti platform.

## Sesi dan logout

```sh
emisell logout
# Jika server tidak tersedia atau sesi telah habis:
emisell logout --local
```

Sesi disimpan sebagai token sensitif di `~/.emisell-cli/session.json`, bukan di folder aplikasi. Password tidak disimpan. Pada macOS/Linux direktori harus 0700 dan file 0600; symlink file sesi ditolak. File sesi **bukan terenkripsi/keychain**: jangan dibagikan, di-commit atau disertakan dalam backup publik. Pada Windows perlindungan bergantung pada ACL direktori profil pengguna.

Satu profil login lokal pada versi ini. Sesi mengikuti batas server, maksimal delapan jam; tidak ada token permanen atau refresh loop. `logout` mencabut sesi server sebelum menghapus salinan lokal. `logout --local` hanya menghapus salinan lokal, **bukan mencabut sesi server**. Error 401 memerlukan login kembali; 403 periksa akun/domain; 409 periksa revision/request-key; 429 tunggu lalu coba lagi.

## Starter embedded dan preview lokal (0.2.0)

```sh
emisell app init --dir my-app --parent-origin http://localhost:3000
emisell app dev --dir my-app --port 4330
```

`app init` (singular) membuat project baru dengan UI Kit Emisell 0.1.0 dan Bridge yang dibundel, tanpa memasang dependency atau mengubah aplikasi di server. Berbeda dengan `apps init --file`, yang hanya membuat dokumen draft shipping. Direktori tujuan harus belum ada. `--parent-origin` adalah origin Dashboard seller yang dipercaya, bukan origin portal developer; jangan mengambilnya dari query/Referrer saat runtime.

Buka `http://127.0.0.1:4330`. Edit `public/index.html` dan `public/app.mjs`, lalu refresh browser. Ctrl+C menghentikan preview. Port terpakai akan menghasilkan error, bukan mematikan proses lain. Tidak ada auto-refresh, tunnel, atau login loop di background.

Preview hanya menyajikan empat aset starter (ditambah `products.css` pada template produk), bukan seluruh folder project. Bind hanya pada loopback, memakai Host check dan CSP frame-ancestors yang spesifik. Jangan gunakan server preview untuk production. Untuk menambah aset/routing atau implementasi backend, gunakan server aplikasi Anda sendiri dengan kontrol keamanan yang sesuai.

Saat dibuka langsung, UI menampilkan preview, bukan identitas toko palsu. Saat di-embed, Bridge meminta identitas ke parent terpercaya dan mengirimnya ke backend same-origin. Tanpa verifier endpoint tetap 503. Gunakan `emisell app dev --dir my-app --backend ./my-app/backend.mjs` untuk memuat modul backend lokal secara eksplisit. Ini mengeksekusi kode Node, gunakan hanya source tepercaya. Verifier Ed25519 tersedia, tetapi public key, issuer/audience dan adapter binding/izin terkini wajib dikonfigurasi server-side. Lihat `server/README.md` pada starter. Konfigurasi kosong atau akses dicabut tetap ditolak. Starter lama tidak diperbarui otomatis.

## Meminta Testing untuk rilis UI

Untuk aplikasi test yang sudah memiliki binding reviewed UI pada Core lokal,
starter juga menyediakan `local-core-backend.mjs`. Konfigurasi server-side
`EMISELL_LOCAL_CORE_ORIGIN`, `EMISELL_LOCAL_APP_ID`, dan `EMISELL_LOCAL_CLIENT_ID`,
lalu jalankan `emisell app dev --dir my-app --backend ./my-app/local-core-backend.mjs`.
Lihat `server/README.md` pada hasil generate. Adapter memakai introspeksi lokal
Core yang memeriksa ulang sesi seller dan launch, bukan menjadikan signature
sebagai izin. Ini tidak otomatis membuat localhost bisa diinstal seller.

```sh
emisell testing list
emisell testing list --after-id ASSIGNMENT_CURSOR
emisell testing request --release-id UI_RELEASE_ID --merchant-id MERCHANT_ID --reason "Pengujian aplikasi embedded" --request-key testing-app-001 --yes
emisell testing show ASSIGNMENT_ID
```

Perintah request memakai **releaseKind: ui** secara default. Untuk rilis `resource-ui` berizin produk, tambahkan **`--release-kind ui_resource`**. Jangan memakai draft ID atau integration shipping release. Rilis harus valid menurut backend dan merchant harus dikenali platform; gunakan ID merchant sebenarnya, bukan slug URL toko. Admin memutuskan permintaan melalui portal, lalu seller melakukan instalasi/consent. Assignment bukan instalasi. `testing show` menampilkan status asli tanpa menganggap approved berarti sudah bisa diakses. Jika `nextAfterId` tidak kosong, gunakan sebagai `--after-id` untuk halaman berikutnya. Seller dapat memilih **Stop testing** untuk mencabut assignment; Uninstall tetap tindakan terpisah.

Starter localhost belum otomatis layak untuk Testing seller. Alur rilis UI umum memerlukan hosting HTTPS, metadata/rilis yang direview dan konfigurasi app-client/launch yang cocok. Pengecualian localhost demo lama tidak diwariskan ke project baru. CLI tidak membuat trust key, menyetujui rilis, mengaktifkan scope Plan atau mengakses Core/DB langsung.

## Batas versi 0.2

Belum ada device/browser OAuth login khusus CLI, perintah pengelolaan app-client secret, tunnel/hosting HTTPS, publish/sign release, adapter otorisasi backend produksi atau worker webhook universal. CLI memakai sesi portal, bukan OAuth token data merchant. Template produk sudah mendukung Open app → verified identity → akses `read_products` di runtime lokal yang dikonfigurasi operator. Akses order, write dan webhook belum termasuk template ini. Review, approval assignment, consent dan instalasi tidak dilewati CLI.

## Perubahan 0.2.0

- Starter embedded dengan snapshot UI Kit/Bridge, preview loopback dan CSP.
- List/show/request assignment rilis UI melalui API Testing existing.
- Template `products`, pencarian nama/SKU dan pagination dengan pemeriksaan akses setiap permintaan.
- Adapter produk lokal dengan secret file privat serta assignment `--release-kind ui_resource`.
- Tidak mengubah Emisell Kurir bawaan atau memberikan izin seller otomatis.

## Perubahan 0.1.1

- Panduan instalasi, login dan alur draft sampai review diperjelas.
- Link repository sementara dihapus dari metadata paket.
- Perintah dan perilaku runtime sama dengan 0.1.0.

Paket saat ini menggunakan `UNLICENSED`; ketersediaan di npm tidak berarti pemberian lisensi open-source.
