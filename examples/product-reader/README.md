# Product Reader — contoh backend developer app

Contoh Go tanpa dependency tambahan: developer mengirim link test-install berdasarkan Merchant ID, merchant menyetujui `read_products`, lalu backend app membaca produk melalui App Gateway. Produk berasal dari module Resource API di checkout `api-service` dan PostgreSQL, bukan array demo di halaman.

Ini **lab lokal dengan data sintetis**, bukan app produksi, SSO produksi, atau publikasi App Store. `read_products` tetap `planned` untuk katalog publik. Payment/Shipping tidak dijalankan atau diubah.

## Jalankan

Dari root `emisell-app-platform`, dengan Node >=22.13, Go >=1.24, Docker aktif, image `postgres:17-alpine`, dan dependencies kedua repo sudah terpasang:

```sh
npm run example:lab -- /absolute/path/to/api-service
```

Runner menyiapkan dua PostgreSQL sementara, migrasi gateway, schema Prisma backend, merchant/produk sintetis, key acak, app development, credential, versi aktif, dan dua test-install request. Ia menjalankan module resource melalui fixture terisolasi — **bukan seluruh server api-service atau middleware bisnis lamanya**.

| Komponen | URL default |
| --- | --- |
| Developer Console / consent terisolasi | `http://localhost:3013` |
| Backend + halaman Product Reader | `http://localhost:3014` |
| App Gateway dengan PostgreSQL sendiri | `http://localhost:3015` |
| Resource backend + dua database | Port loopback acak, dikelola runner |

Port dapat dipindah sekaligus: `PRODUCT_READER_LAB_PORT_BASE=3023 npm run example:lab -- /absolute/path/to/api-service`. Jika port dipakai, hentikan lab ini dan pilih port lain; jangan menghentikan service existing. Runner tidak memuat `.env` repo/shell database, tidak memakai volume existing, dan memakai nama cookie gateway yang unik. Frontend merupakan salinan source terpisah; restart lab untuk memasukkan perubahan source.

## Coba dari browser

1. Buka URL Developer Console yang dicetak runner. Masuk dengan development login lokal bila diminta, lalu buka app **Product Reader Local Lab**.
2. Di bagian development test installation, pilih link pending untuk `merchant-a`. Runner telah membuat request ini; tidak perlu membuat duplikat. Merchant ID adalah penerima, bukan bukti hak akses.
3. Link membuka Product Reader. Klik **Lanjut ke consent Emisell**.
4. Di login merchant lokal, gunakan Merchant ID `merchant-a`. Tinjau app dan izin `read_products`, lalu setujui instalasi. Mode login sintetis ini bukan contoh autentikasi merchant produksi.
5. Callback kembali ke root Product Reader, backend menukar authorization code, kemudian `/products` menampilkan `product-a1` dan `product-a2`. Token/client secret tidak dikirim ke halaman atau localStorage.
6. Buka **Connected Apps**, uninstall Product Reader, lalu muat ulang `/products`: akses ditolak. Untuk mencoba ulang, buat request development baru; jangan memakai kembali authorization code/link yang sudah authorized.

`merchant-b` hanya memiliki `product-b1`. Gunakan profil browser terpisah untuk dua merchant bersamaan; tab baru saja masih berbagi cookie. Jangan mengganti Merchant ID melalui URL pembacaan produk.

## Uji otomatis

```sh
# Unit/security checks untuk contoh Go, tanpa backend/database
npm run example:test

# Dua database baru; tidak membutuhkan server existing
npm run example:check -- /absolute/path/to/api-service
```

`example:check` menjalankan form HTTP → merchant session/consent API → callback → code exchange → pembacaan Prisma nyata. Ia memeriksa state/PKCE, penolakan callback browser lain/replay, isolasi dua merchant, selector tenant ditolak, token terenkripsi di disk, persistensi setelah restart **gateway dan provider**, serta pencabutan akses A tanpa memutus B. Ini bukan browser automation; pemeriksaan tampilan/klik consent dilakukan terpisah.

Pemeriksaan browser lokal pada 3 September 2026 juga berhasil: klik link development → login merchant sintetis A → review scope → Install app → dua produk A tampil; uninstall melalui Connected Apps mengosongkan daftar instalasi dan pembacaan berikutnya ditolak. Pengujian ini menggunakan stack terisolasi, bukan SSO/deployment produksi.

Hentikan `example:lab` dengan Ctrl+C. Runner menghapus **hanya dua container berlabel run ini dan direktori sementara beserta credential/data sintetisnya**. Data lab tidak dapat dipulihkan setelah cleanup; jalankan ulang untuk fixture baru. Saat restart service dalam `example:check`, database dan encryption key tetap sama sehingga persistensi benar-benar diperiksa. Setelah terminasi paksa, periksa label `emisell.product-reader-lab` dan direktori run sebelum cleanup manual; jangan menghapus container/volume lain.

## Bagian kode untuk dipelajari

| File | Tanggung jawab |
| --- | --- |
| `main.go` | Konfigurasi privat, validasi origin, loopback bind, timeout dan shutdown |
| `server.go` | Launch link, form CSRF, OAuth state + PKCE, callback, token exchange, proxy baca produk |
| `store.go` | Session server-side, token terenkripsi AES-256-GCM, atomic file write, single-writer lock |
| `page.html`, `style.css` | Halaman server-rendered, escaped output, tanpa JavaScript/dependency UI |
| `server_test.go` | Penolakan input berbahaya, browser/session binding, penyimpanan dan revocation |

App URL sekaligus callback URL pada model snapshot platform saat ini: `http://localhost:3014/` (termasuk `/` terakhir). Jangan menggantinya menjadi `/callback` tanpa mengubah konfigurasi redirect versi app. Versi aktif adalah snapshot konfigurasi; mengaktifkan versi **tidak** mempublikasikan app ke App Store.

Konfigurasi dibaca dari `PRODUCT_READER_CONFIG_FILE`, file JSON privat mode `0600` yang dibuat runner. Field: `AppID`, `ClientID`, `ClientSecret`, `PublicURL`, `GatewayURL`, `ConsentURL`, `ConnectedAppsURL`, `StoreFile`, `EncryptionKeyFile`, `ListenAddress`, `LocalDevelopment`. Encryption key adalah 32 byte acak di file terpisah; jangan regenerasi key saat restart jika ingin mempertahankan store. Jangan commit, menyalin ke frontend, atau membagikan file konfigurasi/lab state.

## Batas keamanan dan data

- Browser mendapat cookie opaque HttpOnly, SameSite=Lax; Secure di HTTPS. Token, verifier, dan state hash disimpan server-side, berumur terbatas. State sekali pakai dan terikat browser; form POST memerlukan CSRF + origin tepat.
- `Referrer-Policy: same-origin` menjaga native form POST mengirim origin yang bisa diverifikasi, tanpa mengirim referrer lintas origin. Jangan mengganti menjadi `no-referrer` lalu menerima `Origin: null`; perilaku form ini dijelaskan di [MDN Referrer-Policy](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Referrer-Policy).
- Config origin tetap, redirect upstream tidak diikuti, proxy environment diabaikan, timeout 5 detik, body upstream maksimum 4 MiB. CSP hanya mengizinkan redirect form ke origin consent yang dikonfigurasi.
- Token hanya dikirim ke App Gateway. Resource backend menerima assertion RS256 dari gateway, bukan token provider atau cookie merchant. Tenant diambil dari instalasi, tidak dari input provider.
- `GET /api/products` dan `GET /api/products/{productId}` adalah endpoint **contoh app**, membutuhkan cookie session contoh ini; kontrak provider sebenarnya tetap `/v1/products` di gateway. Query contoh hanya `cursor`; ukuran halaman tetap 10.
- Harga adalah decimal string Product dasar; stok adalah `Product.stock`, bukan stok variant/tersedia untuk checkout. Tidak ada mata uang, write scope, resource webhook, atau kalkulasi payment/shipping.
- Token memiliki expiry; contoh tidak membuat refresh-token flow baru. Jika kadaluwarsa/dicabut, hentikan pembacaan dan ulangi instalasi sesuai alur platform. Pada `401`, salinan token lokal dihapus.
- Store satu proses maksimal 1.000 session/4 MiB dan cookie 12 jam. Ini bukan penyimpanan akun multi-user/distributed, autentikasi pengguna provider, atau siap produksi. File mode/enkripsi tidak menggantikan secret manager, HTTPS, otorisasi akun provider, audit dan review operasional.

## Gate pilot (khusus operator)

Runner memasok `APP_ENV=development`, resource adapter/key, `EMISELL_RESOURCE_TEST_MERCHANT_IDS=merchant-a,merchant-b`, dan `DEVELOPMENT_LOOPBACK_APP_HTTP=true` hanya pada gateway lab. Backend memiliki allowlist merchant dan environment sendiri; keduanya harus cocok.

Default gate tetap mati. Hanya `read_products` untuk merchant terdaftar/diizinkan dalam konteks internal sandbox yang boleh melewati status planned saat test-install; scope lain tidak ikut terbuka. Tidak ada pilihan environment baru pada form developer. HTTP hanya menerima host exact `localhost`, `127.0.0.1`, atau `::1` dan **tidak boleh dipublikasikan ke App Store**. Kedua opsi lab ditolak saat startup di luar development.

Lihat [alur development installation](../../docs/development-test-installation.md), [Resource pilot](../../docs/resource-pilot.md), [Provider API](../../docs/provider-api.md), dan [kontrak gateway → backend](../../docs/emisell-resource-api.md). Pengujian lokal ini tidak membuktikan deployment/SSO produksi atau kesiapan scope selain product-read pilot.
