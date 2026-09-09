## Instalasi CLI

```sh
npm install -g @emisell/cli@latest
emisell --version
```

CLI membutuhkan Node.js 22+. Untuk project template, gunakan Node.js 22.12+. Jalankan perintah project dari folder aplikasi atau gunakan `--path`.

## Login merchant dan instalasi

CLI 0.5.0 menggunakan login browser merchant, bukan email/password developer. Backend harus mendukung alur CLI merchant SSO.

```sh
emisell auth login --url http://localhost:4317
emisell auth status
emisell stores list
emisell app config link
emisell app install
emisell app dev --connect
```

Konfirmasi login CLI di browser. Sesi privat berakhir setelah satu jam tanpa aktivitas; tidak membaca cookie Dashboard. Untuk non-interaktif, `app install --app APP_ID --store STORE_ID --no-open` mencetak tautan Dashboard. Pilihan diingat per akun/origin/project, dan `--reset` memilih ulang. Login ulang untuk memperbarui daftar toko.

Dashboard memeriksa sesi dan izin saat Anda mengonfirmasi toko, lalu menampilkan consent. CLI tidak memasang otomatis atau mengklaim instalasi berhasil. `app dev --connect` belum membuat tunnel atau App URL; sesi CLI bukan token akses data aplikasi. `apps init --file app.json` membuat dokumen aplikasi pribadi baca produk tanpa halaman UI; `app init` membuat source project UI, bukan registrasi server.

## Perintah development

| Perintah | Kegunaan |
| --- | --- |
| `emisell app init` | Membuat project melalui wizard |
| `emisell app dev` | Menjalankan frontend dan backend lokal |
| `emisell app build` | Build project aplikasi |
| `emisell app info` | Membaca metadata aman tanpa credential |
| `emisell app doctor` | Memeriksa metadata, file, Node.js, dan dependency |

`app dev` menemukan project dari folder saat ini atau subfoldernya. `--dir` tetap menjadi alias `--path`. Gunakan `--json` untuk keluaran terstruktur pada perintah yang mendukungnya.

## Buat tanpa prompt

```sh
emisell app init --name my-app --path ./my-app --parent-origin https://seller.example.com
cd my-app
npm install
emisell app dev
```

Ganti origin contoh dengan Dashboard seller yang dipercaya. Dependency project dipasang terpisah; CLI tidak menimpa folder existing.

## Periksa masalah lokal

```sh
emisell app info --json
emisell app doctor --json
emisell app init --help
```

Doctor tidak menjalankan source/backend, membaca secret, atau menghubungi server. Hasil lulus tidak membuktikan persetujuan seller atau koneksi produksi.

## Build dan berhenti

```sh
emisell app build
```

Tekan Ctrl+C untuk menghentikan development server. Gunakan `--port 4331` jika port default terpakai. CLI tidak menghentikan proses lain dan tidak membuat tunnel, DNS, atau deployment otomatis.

Baca [build dan deployment](/docs/deployment) untuk container dan runtime produksi.
