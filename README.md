# Emisell Apps Platform

Platform untuk mengelola aplikasi, developer, review, rilis, distribusi pengujian, dan integrasi aplikasi dengan ekosistem Emisell.

Repository ini berisi backend Go, satu dashboard dengan fungsi Admin/Developer, dan App Store publik. Dashboard seller serta backend Emisell Core (`api-service`) berada di repository lain.

> Status: pengembangan dan pengujian lokal. Fitur yang tersedia tidak berarti seluruh integrasi sudah siap produksi. Approval review, publikasi, instalasi, dan pemberian izin merupakan tahap yang berbeda.

## Komponen

| Komponen | Lokasi | Alamat lokal |
| --- | --- | --- |
| Dashboard Admin & Developer | `web/dashboard` | http://localhost:4317 |
| App Store publik | `web/app-store` | http://localhost:4318 |
| Portal Developer legacy (opsional) | `web/developer` | http://localhost:4319 |
| HTTP API | `cmd/server` | http://127.0.0.1:8087 |
| ConnectRPC internal | `cmd/server` | http://127.0.0.1:8088 |
| Worker | `cmd/worker` | Readiness/metrics pada port 8089 |

PostgreSQL dan NATS development memakai port loopback `55437` dan `54227`. Frontend menggunakan proxy API sesuai surface; sesi Admin dan Developer terpisah.

## Fitur dan batasan

- **Admin:** ringkasan, aplikasi, direktori developer, review, rilis, testing, app clients, scope, API key, dan dokumentasi API.
- **Developer:** draft aplikasi, pengajuan review, konfigurasi integrasi/UI, app clients, dan assignment pengujian.
- **App Store:** katalog dan detail aplikasi yang dipublikasikan.
- **Aplikasi dengan UI:** konsep embedded dan external, dengan sesi serta pemeriksaan akses yang terpisah dari token akses data.
- **Scope:** status mengikuti bukti kesiapan backend. Scope yang tercantum belum tentu aktif atau dapat diberikan kepada aplikasi.
- **Kelola Staf:** direktori hanya-baca untuk administrator, dengan pencarian dan filter peran/status. Undangan, perubahan peran, dan reset password melalui UI belum tersedia.
- **Aktivitas:** riwayat review per pengajuan; belum merupakan audit gabungan seluruh layanan.

API-Kurir merupakan engine/layanan remote. Apps Platform mengelola lifecycle dan otorisasi integrasi; detail tarif dan layanan pengiriman tetap menjadi tanggung jawab engine. Payment gateway checkout dikelola internal Emisell, bukan distribusi aplikasi payment umum.

## Menjalankan lokal

### Prasyarat

- Go **1.26.6** atau toolchain yang kompatibel dengan `go.mod`.
- Node.js **24+** dan npm.
- Docker dengan Docker Compose.

### 1. Ambil source

```sh
git clone git@github.com:sbahri-sg/emisell-apps-platform.git
cd emisell-apps-platform
```

### 2. Siapkan environment baru

Perintah berikut ditujukan untuk **environment development baru**, bukan database produksi atau database existing tanpa backup. Provisioning membuat konfigurasi privat dan data reference lokal.

```sh
go run ./cmd/cli init-events
docker compose -f deploy/compose.local.yaml up -d --wait
go run ./cmd/cli init-local
go run ./cmd/cli init-portals
```

Credential portal disimpan di `.local/portals.json`; baca secara lokal, jangan commit atau menyalinnya ke frontend/log. Akun tenant reference bukan akun Admin. Gunakan identitas yang tercatat pada environment masing-masing.

Untuk database existing, baca [panduan development dan migration](docs/local-development.md) terlebih dahulu. Signing katalog/integrasi, credential Core, serta demo embedded memiliki provisioning terpisah; tidak otomatis diaktifkan oleh langkah dasar ini.

### 3. Jalankan backend

Pada terminal terpisah dari root repository:

```sh
go run ./cmd/server
```

```sh
go run ./cmd/worker
```

### 4. Jalankan frontend

Jalankan setiap frontend yang dibutuhkan pada terminal tersendiri:

```sh
cd web/dashboard
npm ci
npm run dev:local
```

```sh
cd web/developer
npm ci
npm run dev
```

```sh
cd web/app-store
npm ci
npm run dev
```

Backend harus aktif agar login dan pemuatan data berhasil. Tidak perlu menjalankan demo embedded untuk menggunakan portal dasar.

## Pengujian

Dari `web/dashboard`:

```sh
npm run typecheck
npm run lint
npm test
npm run build
```

Frontend Developer dan App Store menyediakan perintah pengujian/build masing-masing. Perubahan komponen bersama perlu diuji pada frontend yang menggunakannya.

Dari root repository:

```sh
make tools
make verify
```

`make verify` memeriksa kontrak, menjalankan race tests, vet, dan build. Integration tests memerlukan database test terpisah melalui `EMISELL_TEST_DATABASE_URL` dan NATS test melalui `EMISELL_NATS_SERVER`. Ikuti [instruksi verifikasi](docs/local-development.md#verifikasi); test yang dilewati karena environment belum tersedia bukan bukti integrasi berhasil. Jangan gunakan database pengguna untuk test.

## Dokumentasi API

Buka menu **Dokumentasi API** di Dashboard Admin. Kontrak sumber berada di `api/openapi` dan `api/proto`; dokumentasi generated tidak diedit langsung.

Setelah mengubah kontrak:

```sh
npm run docs:generate --prefix web/dashboard
npm test --prefix web/dashboard
```

Generator memerlukan Buf lokal (`make tools`). Contoh request merupakan struktur acuan, bukan credential atau payload bisnis siap dijalankan.

## Struktur source

```text
api/          Kontrak OpenAPI, Protobuf, dan manifest
cmd/          Entrypoint server, worker, CLI, dan aplikasi reference
internal/     Domain, service, repository, dan transport backend
migrations/   Migration database
pkg/          SDK, kontrak integrasi, dan UI kit aplikasi
web/          Admin, Developer Portal, dan App Store
examples/     Contoh integrasi
deploy/       Infrastruktur development lokal
docs/         Panduan, keputusan arsitektur, dan mockup desain
```

## Panduan lanjutan

- [Development lokal](docs/local-development.md)
- [Keputusan arsitektur](docs/adr/)
- [Install intent](docs/core-install-intents.md) dan [lifecycle instalasi](docs/core-installation-lifecycle.md)
- [Rilis integrasi](docs/integration-releases.md) dan [app clients](docs/app-clients.md)
- [Embedded apps](docs/embedded-apps.md), [demo lokal](docs/embedded-demo.md), dan [UI kit](docs/embedded-ui-kit.md)
- [API rilis UI](docs/ui-release-api.md)
- [Handoff gateway Emisell](docs/emisell-gateway-handoff.md)
- [Pengujian shipping](docs/shipping-demo-testing.md) dan [pemetaan scope](docs/shipping-scope-mapping.md)
- [Catatan keputusan dan status implementasi](codex.md)

Sebagian panduan mencatat milestone historis. Untuk status endpoint gunakan kontrak terbaru; untuk kesiapan scope gunakan verifikasi backend, bukan angka atau status pada catatan lama.

## Keamanan dan operasional

### Domain deployment

Base URL publik dibaca dari `EMISELL_DASHBOARD_ORIGIN` dan `EMISELL_STORE_ORIGIN`. Lihat [contoh konfigurasi domain](deploy/domains.env.example). Isi keduanya dengan domain HTTPS berbeda, tanpa path, port, atau trailing slash. Pada `EMISELL_ENV=production`, domain wajib diisi; konfigurasi yang tidak valid ditolak. Konfigurasi legacy Admin/Developer masih diterima jika tidak memakai `EMISELL_DASHBOARD_ORIGIN`; jangan mencampur keduanya.

Login gabungan memakai `/api/v1/portal/login` dan sesi `/api/v1/portal/session`. Backend memilih surface berdasarkan credential; client tidak boleh mengirim peran. Cookie gabungan tetap terikat satu surface di database, sehingga Developer tidak dapat memakai API Admin. Logout mencabut sesi. Credential yang cocok dengan dua akun sekaligus ditolak, bukan otomatis memilih Admin. Frontend Developer legacy tidak dijalankan oleh Compose baru.

Frontend tetap meminta `/api/v1/...` pada origin sendiri. Reverse proxy harus meneruskan `Host`, `Origin`, dan `Referer` asli serta memisahkan route API sesuai portal. Backend tidak mengambil base URL dari `Host` atau `X-Forwarded-Host`. Cookie portal HTTPS memakai `Secure`, `HttpOnly`, dan host-only; tidak dibagikan antar-subdomain.

Paket Docker untuk **portal/control plane** tersedia: [panduan deployment](docs/docker-deployment.md). Mode production memakai database eksplisit dan API pada jaringan container; RPC tetap loopback. Proxy HTTPS, volume, migration eksplisit dan bootstrap satu akun Admin disediakan. Worker/NATS reference serta engine transaksi belum termasuk paket production. Jangan menyalin konfigurasi simulator atau pilot lokal ke server.

- Jangan commit `.local/`, `.env`, private key, token, password, atau database dump.
- API key full-access hanya untuk backend terpercaya, bukan browser atau aplikasi pihak ketiga.
- Jangan membuka listener development ke internet. Konfigurasi ini bukan panduan deployment produksi.
- Backup database sebelum migration; jangan mengganti credential atau mencabut instalasi sebagai langkah setup rutin.
- Untuk menghentikan infrastruktur lokal tanpa menghapus volume:

```sh
docker compose -f deploy/compose.local.yaml stop
```
