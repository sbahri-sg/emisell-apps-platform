export const commandHelp = {
  'app init': `emisell app init [--name NAME] [--path NEW_DIRECTORY] [--template react-router] [--parent-origin ORIGIN]

Membuat project lokal baru melalui panduan interaktif. Tidak membuat aplikasi di server.
Non-interaktif: isi --name atau --path dan --parent-origin; template React Router + TypeScript + Vite.
Jalankan npm install di hasil generate. Template HTML lama sudah dihapus.
--dir tetap didukung sebagai alias --path. Folder yang sudah ada tidak ditimpa.
Contoh: emisell app init --name my-app --parent-origin http://localhost:3000`,
  'app dev': `emisell app dev [--path DIRECTORY] [--port PORT] [--backend MODULE] [--connect] [--app APP_ID] [--store STORE_ID] [--reset] [--no-open]

Menjalankan preview dari folder project saat ini atau subfoldernya. Port default 4330.
--dir tetap didukung. --backend mengeksekusi modul Node tepercaya secara eksplisit.
Path backend relatif terhadap terminal saat perintah dijalankan (kompatibel dengan 0.2.0).
Vite hot reload aktif. Memerlukan Node 22.12+ dan npm install di project.
Menjalankan source/config project tepercaya. Backend bawaan membaca .env privat.
Tanpa konfigurasi backend, UI dapat dibuka tetapi akses identitas/data ditolak.
--connect memilih aplikasi/toko (atau memakai pilihan tersimpan) dan membuka consent Dashboard.
--reset memilih ulang. --no-open hanya mencetak URL consent. Tidak otomatis install atau membuat tunnel.
Tanpa --connect, preview lokal tetap berjalan tanpa login. Koneksi ini bukan binding UI launch; atur backend/URL app secara terpisah.`,
  'app info': `emisell app info [--path DIRECTORY] [--json]

Menampilkan metadata project lokal yang aman. Tidak membaca sesi login, .env atau secret.
Scope template adalah kebutuhan fitur, bukan bukti izin seller. --dir tetap didukung.`,
  'app build': `emisell app build [--path DIRECTORY]

Menjalankan npm run build dari project tepercaya. Memerlukan npm install.
Tidak mengunggah image, men-deploy server, membuat release atau mengubah izin toko.
Lihat DEPLOYMENT.md pada project untuk Docker dan hosting. --dir tetap didukung.`,
  'app doctor': `emisell app doctor [--path DIRECTORY] [--json]

Memeriksa konfigurasi, source, Node dan dependency tanpa memuat .env, mengeksekusi backend atau menghubungi server.
Exit 1 jika ada kesalahan lokal; peringatan kesiapan server tidak berarti preview gagal.
Hasil lulus bukan verifikasi DNS/TLS, consent atau akses data toko. --dir tetap didukung.`,
  'auth login': `emisell auth login --url PORTAL_ORIGIN [--no-open]

Login melalui browser merchant Emisell, kemudian konfirmasi akses CLI.
--no-open mencetak URL jika browser tidak dapat dibuka. Batas tunggu 5 menit.
Tidak menerima email/password atau mengimpor cookie browser. Sesi privat berakhir setelah 1 jam tanpa aktivitas.`,
  'auth status': `emisell auth status\n\nMemeriksa sesi developer terkini. Alias whoami.`,
  'app install': `emisell app install [--app APP_ID] [--store STORE_ID_OR_SLUG] [--path DIRECTORY] [--reset] [--no-open]\n\nPilih aplikasi dan toko, lalu buka consent Dashboard. Pilihan diingat per akun/origin/project di direktori sesi privat.\nDaftar toko berasal dari login merchant terakhir; login ulang untuk menyegarkan. Dashboard memeriksa izin terkini. Tidak otomatis install.`,
  'app config link': `emisell app config link [--app APP_ID] [--store STORE_ID_OR_SLUG] [--path DIRECTORY] [--reset]\n\nSimpan pilihan aplikasi/toko tanpa membuka consent, menulis secret ke project, atau mengganti launch URL. Gunakan app dev --connect sesudahnya.`,
  'stores list': `emisell stores list\n\nLihat profil dan toko dari login merchant terakhir. Login ulang untuk menyegarkan; bukan bukti izin instalasi terkini.`,
  'auth logout': `emisell auth logout [--local]

Alias emisell logout. Default mencabut sesi server; --local hanya menghapus salinan lokal.`,
};

export const help = `Emisell Developer CLI

Development lokal
  emisell app init                 Buat project dengan panduan interaktif
  emisell app dev                  Jalankan preview dari folder project
  emisell app build                Build project tanpa deploy
  emisell app info [--json]        Lihat konfigurasi lokal
  emisell app doctor [--json]      Periksa kesiapan preview
  emisell app COMMAND --help      Bantuan per perintah

Akun developer (memerlukan server portal)
  emisell auth login --url URL
  emisell auth status
  emisell auth logout [--local]
  emisell whoami
  Alias lama: emisell login / emisell logout

Pengelolaan melalui API portal
  emisell apps list
  emisell stores list
  emisell app config link
  emisell app install
  emisell app dev --connect
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

app init membuat project UI; apps init membuat dokumen aplikasi pribadi read_products tanpa halaman UI.
Output API berupa JSON. Tidak ada auto-retry mutasi.
Request-key: 8–128 karakter huruf/angka/_/-. Gunakan key baru untuk operasi baru.
Review bukan publish. CLI tidak memberi akses seller atau menandatangani rilis.
Tunnel dan app deploy belum tersedia; preview bukan server produksi. Login browser memerlukan backend dengan dukungan CLI merchant SSO.
`;
