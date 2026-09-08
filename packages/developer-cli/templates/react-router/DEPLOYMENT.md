# Build, Docker dan hosting aplikasi

Template menyediakan web server hasil build dan kemasan Docker. Keduanya **bukan**
instalasi seller, publikasi rilis, OAuth, atau aktivasi API produk production.
Tidak diperlukan CLI Emisell pada container runtime. Tidak ada database/Prisma,
migrasi otomatis, token contoh, tunnel, atau akses toko otomatis.

## 1. Development biasa

```sh
npm install
emisell app dev
```

Gunakan `npm run dev` jika tidak memasang CLI. Development tetap loopback; adapter
Core lokal yang sudah ada tidak berubah. Jangan expose Vite ke internet.

## 2. Build yang dapat diulang

```sh
npm run typecheck
emisell app build
```

Alternatif tanpa CLI: `npm run build`. Simpan `package-lock.json` hasil `npm install`
ke Git. CI dan Docker menggunakan `npm ci`; build Docker sengaja gagal jika lockfile
tidak tersedia atau tidak cocok. Setiap update dependency perlu diuji ulang.
Tanpa `NODE_ENV=production`, `npm start` tetap menjalankan preview hasil build lokal.

## 3. Coba Docker di komputer sendiri

```sh
npm run docker:preview
```

Buka `http://127.0.0.1:4330`. Container menjalankan build, bukan Vite, dengan label
**Container preview · no store access**. Tidak ada hot reload pada mode ini; build
ulang setelah mengedit UI. File `.env`, key, database dan konfigurasi Core host tidak
disalin atau di-mount. API toko mengembalikan 503. Ini sengaja hanya uji kemasan UI:
pengujian produk lokal tetap memakai `emisell app dev` dengan Core host-local.

```sh
docker compose ps
docker compose logs --tail 30 app
npm run docker:down
```

Port hanya diterbitkan pada loopback. Jangan ubah ke `4330:4330` untuk membuka server
ke jaringan. Jika port terpakai, hentikan preview milik project ini terlebih dahulu;
jangan menghentikan proses project lain. Tidak ada persistent volume yang dihapus.

## 4. Web runtime production

Environment development dan hosting dipisahkan. Lihat `.env.production.example`.
Server production hanya menggunakan environment proses, tidak membaca `.env` lokal.

| Setting | Fungsi |
| --- | --- |
| `NODE_ENV=production` | Menjalankan hasil build tanpa Vite |
| `HOST` | Default `127.0.0.1`; Docker memakai `0.0.0.0` di jaringan container |
| `PORT` | Default 3000; samakan dengan proxy/port container |
| `EMISELL_APP_URL` | Origin HTTPS aplikasi yang sudah ditinjau, tanpa path |
| `EMISELL_DASHBOARD_ORIGIN` | Origin HTTPS seller tepercaya, berbeda dari aplikasi |
| `EMISELL_TLS_TERMINATION=external` | Konfirmasi TLS di reverse proxy dan jalur upstream privat |
| `EMISELL_BACKEND_MODE` | `disabled` untuk shell UI; `custom` untuk adapter nyata |

Konfigurasi hosting mengganti origin development dalam metadata hanya saat runtime;
tidak mengubah release, bukti origin, assignment, consent atau grant di Platform.
Semua `EMISELL_LOCAL_*` ditolak di production/container. Jangan menyalin Core key ke aplikasi.

Setelah menyediakan environment pada host:

```sh
npm ci
npm run build
NODE_ENV=production npm start
```

Atau Docker, memakai **file terpisah** (jangan merge dengan compose preview):

```sh
docker compose --env-file .env.production -f compose.production.yaml up --build -d
```

File `.env.production` adalah konfigurasi privat lokal untuk Compose, bukan isi image.
Compose hanya meneruskan setting yang tercantum di YAML. Adapter custom yang memerlukan
credential perlu deklarasi environment/secret runtime tersendiri. Jangan memakai build
ARG, prefix `EMISELL_PUBLIC_*`, source UI, atau metadata app untuk secret.
Simpan private key/credential pada secret manager atau mount file read-only dengan
pemilik/izin yang sesuai user container (UID/GID 1000). Jangan commit nilainya.

### Batas jaringan

Server internal memakai HTTP. HTTPS wajib ditangani reverse proxy sebelum trafik
browser mencapai aplikasi. Proxy harus mempertahankan **Host publik persis** sesuai
`EMISELL_APP_URL`. `Forwarded`/`X-Forwarded-*` diabaikan, bukan sumber kepercayaan.
Jangan buka port HTTP upstream ke internet, dan batasi akses ke proxy saja. Compose
contoh memetakan port ke loopback host; proxy di host dapat mengakses port tersebut.
Jika memakai proxy/container/hosting lain, tentukan private network dan firewall-nya.
Template tidak mengatur DNS, sertifikat atau firewall provider secara otomatis.

Dockerfile menggunakan build bertahap, lockfile, Node base dengan digest tetap,
dependency production saja, dan user non-root. Compose memakai read-only filesystem,
tmpfs `/tmp`, menghapus Linux capabilities dan melarang privilege escalation.
Refresh digest Node secara berkala, kemudian build/test ulang untuk patch keamanan.

## 5. Sambungan backend: wajib nyata, bukan mock

Default `disabled`: UI hidup, API toko **503**, `/health/ready` **503**. Container
production akan terlihat unhealthy sampai adapter disambungkan; ini bukan izin toko.
`custom` memuat `server/production-backend.mjs`; factory bawaan sengaja melempar error.
Tidak ada fallback ke adapter lokal atau nilai identitas/produk palsu.

Operator harus menyediakan kontrak API production yang telah diaktifkan dan ditinjau.
Implementasikan `createBackend({ env, config })` dengan hasil:

- `verifySession(token, { signal })`: verifikasi signature, issuer, audience/client,
  waktu berlaku, app/installation/merchant/staff binding dan akses terkini melalui
  layanan tepercaya. Kembalikan hanya `{merchantId, actorId, appId, installationId,
  expiresAt}`. `createIdentityVerifier` tersedia, tetapi callback izin harus nyata.
- `readProducts(token, { signal, query })`: autentikasi app-client dan periksa grant
  `read_products` terkini pada setiap permintaan. Merchant/installation berasal dari
  sesi terverifikasi, tidak dari query browser. Kembalikan DTO `data` dan
  `meta.nextCursor`, lihat `server/product-query.mjs`.
- `readResource(token, { path, signal, query })`: diperlukan untuk menu Pesanan dan
  Pengiriman. Allowlist hanya endpoint baca Emisell yang didukung; periksa izin
  `read_orders` atau `read_shipping`, tenant dan instalasi pada setiap permintaan.
  Jika tidak dikonfigurasi, menu tersebut mengembalikan 503, bukan akses anonim.
- `checkReady({ signal })`: pemeriksaan dependency/konfigurasi yang bounded tanpa
  membaca data seller, hanya `true` jika dependency siap. Bukan pemeriksaan grant
  semua toko dan bukan callback yang selalu mengembalikan true.

Gunakan HTTPS terverifikasi ke origin backend yang dipin operator, timeout, batas
respons dan tanpa redirect. Jangan mengambil endpoint/JWKS dari JWT atau browser,
meneruskan cookie seller, menyimpan bearer browser, atau mencatat credential ke log.
Untuk identitas tidak valid gunakan `IdentityError('invalid_identity')`, penolakan
izin `IdentityError('access_denied')`, dan kegagalan dependency throw (503).

Endpoint loopback `/v1/app-platform/core/reviewed-ui/*` tidak boleh dijadikan API
production atau dibuka lewat proxy. Komposisi resource production Platform/Core
masih perlu diaktifkan dan diuji operator secara terpisah. Image yang berhasil
dibangun bukan bukti bahwa produk, order, webhook atau instalasi production aktif.

## 6. Health, shutdown dan pemeriksaan sebelum rilis

- `GET /health/live`: proses HTTP hidup, tidak menghubungi backend.
- `GET /health/ready`: framework siap; di production juga adapter lengkap dan
  `checkReady` berhasil (timeout 2 detik). Tidak membocorkan konfigurasi atau secret.
- SIGTERM/SIGINT: berhenti menerima request dan menunggu maksimal 10 detik.
  Deadline terlewati memutus request dan keluar nonzero; Compose memberi 15 detik.
- Nonaktif/revoke/uninstall tetap harus ditolak pada permintaan data berikutnya.

Uji origin/Host salah, token palsu/kedaluwarsa, merchant lain, izin dicabut,
backend mati, pencarian/pagination, restart, dan pastikan log/image/client bundle
bebas secret. Ada batas 64 request aktif per proses production. Terapkan rate limiting
di proxy; timeout dan batas concurrency aplikasi bukan rate limiter
terdistribusi. Deploy aplikasi dan persetujuan rilis/instalasi seller adalah langkah
berbeda. Jangan menganggap `app build` sebagai `app deploy`.
