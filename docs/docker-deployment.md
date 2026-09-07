# Docker deployment — portal/control plane

Paket ini menjalankan Admin, Developer Portal, App Store, API Go, PostgreSQL, dan reverse proxy Caddy. Domain berasal dari konfigurasi, bukan hasil membaca header pengguna. Caddy mengurus HTTPS otomatis setelah DNS mengarah ke server dan port 80/443 tersedia.

**Batas paket:** belum deployment engine transaksi penuh. Worker/NATS reference tidak dijalankan karena masih memakai tenant/broker lokal. RPC tetap loopback dan endpoint merchant/simulator tidak diekspos dalam mode production. Alur shipping, callback, embedded pilot dan processing background tidak boleh dinyatakan siap production dengan paket ini. Jangan memindahkan `.local` demo ke volume production.

## Persiapan

1. Siapkan server dengan Docker Compose v2, kapasitas disk memadai, serta tiga domain berbeda.
2. Arahkan DNS domain Admin, Developer, dan Store ke server. Buka hanya port 80/443 untuk publik.
3. Salin `deploy/production.env.example` menjadi `deploy/production.env` secara lokal di server. Isi domain **tanpa** `https://`, path, atau port; isi email ACME dan password PostgreSQL acak minimal 16 karakter. Gunakan karakter alfanumerik agar aman pada URL database. File ini diabaikan Git; batasi izin file ke pemilik.
4. Jangan memakai database/volume development atau password demo. Backup sebelum memakai database existing.

Semua perintah berikut dijalankan dari root repository:

```sh
docker compose --env-file deploy/production.env -f deploy/compose.production.yaml config -q
docker compose --env-file deploy/production.env -f deploy/compose.production.yaml build
docker compose --env-file deploy/production.env -f deploy/compose.production.yaml up -d postgres
docker compose --env-file deploy/production.env -f deploy/compose.production.yaml run --rm migrate
```

Migration adalah langkah eksplisit. Startup API hanya memverifikasi schema; tidak migrate atau membuat data contoh otomatis. Jangan menjalankan migration serentak dari beberapa host.

## Akun Admin pertama

Jalankan provisioning satu akun secara eksplisit, ganti email berikut:

```sh
docker compose --env-file deploy/production.env -f deploy/compose.production.yaml run --rm -T migrate cli bootstrap-admin admin@domain-anda.com
```

Perintah menerima password melalui stdin hingga EOF. Gunakan input dari secret manager atau file privat; jangan memasukkan password ke argumen, shell history, atau percakapan. Panjang 16–256 byte. Perintah tidak mencetak password dan tidak membuat akun reviewer/developer demo. Pengulangan hanya menerima identitas/password yang sama; tidak merotasi credential existing. Undangan akun lain belum tersedia.

## Menjalankan dan update

```sh
docker compose --env-file deploy/production.env -f deploy/compose.production.yaml up -d
docker compose --env-file deploy/production.env -f deploy/compose.production.yaml ps
```

Setelah update source: backup database dan volume signing key, build ulang, jalankan migration secara eksplisit, lalu `up -d`. Publikasi katalog membutuhkan signing key yang diprovision terpisah melalui CLI; jangan mengubah key existing atau menganggap login berhasil berarti semua workflow aktif.

Volume `postgres_data` menyimpan database, `platform_keys` menyimpan konfigurasi/signing key privat, dan volume Caddy menyimpan sertifikat. Jangan menggunakan `down -v` untuk restart/update. Salinan source Git tidak mencakup data dan secret tersebut.

## Pengamanan dan verifikasi

- PostgreSQL dan API tidak memiliki published host port; hanya proxy yang membuka 80/443.
- Konfigurasi HTTPS harus lengkap; cookie Admin/Developer memakai Secure, HttpOnly, SameSite Strict, tanpa domain bersama.
- Proxy hanya meneruskan API sesuai portal. Endpoint internal RPC dan simulator tidak diproxy ke internet.
- Runtime Go non-root; dependency dan aplikasi dibangun dalam stage build, tanpa menyertakan `.env`, `.local`, atau cache host.
- Belum ada klaim load test, HA, backup otomatis, worker production, atau uji sertifikat/domain nyata. Jalankan staging terlebih dahulu; uji login, otorisasi lintas portal, migration, restart, dan pemulihan backup.
- Mode lokal tetap memakai batas loopback lama. Perubahan origin production tidak mengaktifkan scope Plan, instalasi, atau koneksi provider secara otomatis.
