export const commandHelp = {
  'app init': `emisell app init [--name NAME] [--path NEW_DIRECTORY] [--template embedded|products] [--parent-origin ORIGIN]

Membuat project lokal baru melalui panduan interaktif. Tidak membuat aplikasi di server.
Non-interaktif: isi --name atau --path dan --parent-origin; template default embedded.
--dir tetap didukung sebagai alias --path. Folder yang sudah ada tidak ditimpa.
Contoh: emisell app init --name my-app --template products --parent-origin http://localhost:3000`,
  'app dev': `emisell app dev [--path DIRECTORY] [--port PORT] [--backend MODULE]

Menjalankan preview dari folder project saat ini atau subfoldernya. Port default 4330.
--dir tetap didukung. --backend mengeksekusi modul Node tepercaya secara eksplisit.
Path backend relatif terhadap terminal saat perintah dijalankan (kompatibel dengan 0.2.0).
Tanpa backend, UI dapat dibuka tetapi akses identitas/data ditolak. Refresh manual setelah edit.
Tidak membuat tunnel, mengubah server, atau menginstal aplikasi ke toko.`,
  'app info': `emisell app info [--path DIRECTORY] [--json]

Menampilkan metadata project lokal yang aman. Tidak membaca sesi login, .env atau secret.
Scope template adalah kebutuhan fitur, bukan bukti izin seller. --dir tetap didukung.`,
  'app doctor': `emisell app doctor [--path DIRECTORY] [--json]

Memeriksa konfigurasi dan aset preview tanpa mengeksekusi backend atau menghubungi server.
Exit 1 jika ada kesalahan lokal; peringatan kesiapan server tidak berarti preview gagal.
Hasil lulus bukan verifikasi DNS/TLS, consent atau akses data toko. --dir tetap didukung.`,
  'auth login': `emisell auth login --url PORTAL_ORIGIN --email EMAIL [--password-stdin]

Alias emisell login. Memakai sesi portal developer; login browser/OAuth belum tersedia.
Password diminta tanpa echo; --password-stdin hanya untuk pipe dari sumber privat.`,
  'auth logout': `emisell auth logout [--local]

Alias emisell logout. Default mencabut sesi server; --local hanya menghapus salinan lokal.`,
};

export const help = `Emisell Developer CLI

Development lokal
  emisell app init                 Buat project dengan panduan interaktif
  emisell app dev                  Jalankan preview dari folder project
  emisell app info [--json]        Lihat konfigurasi lokal
  emisell app doctor [--json]      Periksa kesiapan preview
  emisell app COMMAND --help      Bantuan per perintah

Akun developer (memerlukan server portal)
  emisell auth login --url URL --email EMAIL
  emisell auth logout [--local]
  emisell whoami
  Alias lama: emisell login / emisell logout

Pengelolaan melalui API portal
  emisell apps list
  emisell apps show APP_ID
  emisell apps init --file app.json
  emisell apps create --file app.json --request-key UNIQUE_KEY
  emisell apps update APP_ID --file app.json --revision NUMBER --request-key UNIQUE_KEY
  emisell reviews list
  emisell reviews show REVIEW_ID
  emisell reviews submit APP_ID --revision NUMBER --request-key UNIQUE_KEY --yes
  emisell scopes
  emisell ui list | show RELEASE_ID
  emisell ui create --file ui.json --request-key UNIQUE_KEY --yes
  emisell resource-ui list | show RELEASE_ID
  emisell resource-ui create --file ui.json --request-key UNIQUE_KEY --yes
  emisell testing list [--after-id CURSOR]
  emisell testing show ASSIGNMENT_ID
  emisell testing request --release-id RELEASE_ID [--release-kind ui|ui_resource] --merchant-id MERCHANT_ID --reason TEXT --request-key UNIQUE_KEY --yes

  emisell --version

app init membuat project; apps init hanya membuat dokumen draft shipping.
Output API berupa JSON. Tidak ada auto-retry mutasi.
Request-key: 8–128 karakter huruf/angka/_/-. Gunakan key baru untuk operasi baru.
Review bukan publish. CLI tidak memberi akses seller atau menandatangani rilis.
Login browser, tunnel dan app deploy belum tersedia; preview bukan server produksi.
`;
