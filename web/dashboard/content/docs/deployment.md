## Build bukan aktivasi toko

Template menyediakan runtime web dan Docker. Build yang berhasil tidak menerbitkan aplikasi, memasang aplikasi pada seller, atau mengaktifkan akses resource produksi.

## Buat hasil build

```sh
npm run typecheck
emisell app build
```

Alternatif tanpa CLI adalah `npm run build`. Simpan `package-lock.json`; CI dan Docker memakai `npm ci`. Tanpa mode produksi, `npm start` tetap digunakan untuk preview build lokal.

## Preview Docker lokal

```sh
npm run docker:preview
docker compose ps
npm run docker:down
```

Container preview menguji kemasan UI, bukan akses toko. Tidak ada hot reload; build ulang setelah mengubah UI. File privat lokal tidak disalin ke image. Pada mode ini API toko mengembalikan 503 sesuai batas preview.

## Siapkan hosting

Ikuti `DEPLOYMENT.md` dan `.env.production.example` dari project hasil generate. Konfigurasi runtime mencakup:

| Setting | Fungsi |
| --- | --- |
| `NODE_ENV=production` | Menjalankan hasil build tanpa Vite |
| `EMISELL_APP_URL` | Origin HTTPS aplikasi yang ditinjau |
| `EMISELL_DASHBOARD_ORIGIN` | Origin HTTPS Dashboard seller yang dipercaya |
| `EMISELL_TLS_TERMINATION=external` | TLS ditangani reverse proxy dengan upstream privat |
| `EMISELL_BACKEND_MODE` | `disabled` untuk shell UI; `custom` untuk adapter nyata |

Server produksi tidak memakai konfigurasi adapter lokal `EMISELL_LOCAL_*`. Jangan mengirim secret lewat build ARG, metadata publik, atau bundle frontend.

## Sambungkan adapter nyata

Mode `custom` memerlukan implementasi backend produksi yang nyata. Factory bawaan bukan koneksi produksi siap pakai. Verifikasi identitas, grant, binding toko/instalasi, timeout, dan readiness tetap diperlukan.

Gunakan origin backend yang dikonfigurasi operator, HTTPS terverifikasi, serta jalur private upstream. Jangan menjadikan endpoint adapter lokal sebagai API publik dengan membuka proxy.

## Health dan pemeriksaan rilis

- `GET /health/live` memeriksa proses HTTP.
- `GET /health/ready` memeriksa kesiapan framework dan adapter sesuai mode.
- Mode backend `disabled` pada produksi tetap mengembalikan readiness 503.
- Uji pencabutan akses, token tidak valid, origin salah, dependency mati, dan restart.

Hosting, review rilis, dan persetujuan seller dilakukan terpisah. Tidak ada `app deploy` otomatis yang dijanjikan oleh panduan ini.
