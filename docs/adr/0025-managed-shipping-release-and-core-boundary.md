# ADR 0025 — Release provider terkelola dan boundary Shipping Core

Status: diterapkan untuk pengamanan pengaturan dan pipeline release lokal, 6 September 2026. **Distribusi managed release, instalasi merchant, dan cutover engine belum diimplementasikan.** Keputusan ini tidak mengubah larangan metadata → fixture pada ADR 0021/0024.

## Konteks dan keputusan

Pengguna menyetujui pengerjaan prasyarat Emisell Kurir. API-Kurir tetap engine, provider `emisell` adalah aplikasi terpisah. Auth engine server-to-server bukan OAuth aplikasi eksternal. Jangan mengarang endpoint/callback atau menambah scope agar draft lulus pipeline Remote App.

1. Core memakai boundary khusus untuk proxy pengaturan shipping: sesi JWT HS256 terverifikasi, current session/device, user/merchant aktif, membership dan izin `storeSettings.shippingAndDelivery` pada primary database. Owner tetap diperbolehkan; izin install apps saja tidak memberi izin shipping. Shared authorizer Apps mempertahankan default permission sebelumnya.
2. Route/metode management adalah allowlist. Custom header `X-Emisell-Shipping: 1` dan Origin dashboard wajib; GET melalui Next rewrite boleh memakai Referer trusted jika Origin tidak ada. Header merchant/bearer legacy hanya boleh cocok dengan sesi, bukan mengganti identitas. Duplicate cookie/header dan query identity ditolak. Tidak ada auto-refresh/retry mutation.
3. Browser proxy hanya melayani pengaturan provider/services serta rute lokasi/ongkir publik yang dikenal. Shipment, pickup, label, tracking/subscription dan endpoint arbitrary tidak lagi tersedia lewat wildcard. Alur server-side order/checkout existing tetap berada di service masing-masing dan tidak dipindahkan. Webhook tetap memakai verifier signature existing. Public domain resolution memakai `customerOnly`, sehingga cookie admin yang hanya di-decode tidak dapat memilih merchant.
4. Forwarder hanya memakai origin engine dari konfigurasi server, service key server-only, merchant hasil verifikasi; JSON content type eksplisit, no redirect, timeout 5 detik, maksimum 8 request bersamaan dan response 512 KiB. Error/body/key upstream tidak diteruskan. Credential response yang memuat key mentah ditolak. Keberhasilan mutation yang responsnya hilang tetap ambigu: UI tidak melakukan retry otomatis.
5. Pipeline baru `emisell.managed-shipping-release/v1`, policy `managed-kurir-provider/v1`, menyimpan snapshot draft, app/organisasi/versi, source digest, binding engine/provider, requested scopes dan free pricing. Pilot hanya binding exact `api-kurir/emisell`, `shipping/v1`, scope `shipping.read`, tanpa URL, credential, resource scope atau OAuth eksternal. RajaOngkir belum dibuka pada policy managed ini; fixture historis tetap kompatibel.
6. Developer mengajukan draft miliknya dan revision saat ini; reviewer/admin menyetujui/menolak; hanya administrator sign/suspend. Satu release per app/version, immutable setelah submit; perubahan memerlukan versi baru. State `submitted → approved|rejected → signed|suspended`, signed hanya dapat suspended. Receipt/audit atomik; retry membaca state terkini termasuk setelah draft diedit atau release disuspend. Repository mengunci dan memverifikasi source draft saat create.
7. Signature Ed25519 mengikat canonical manifest dengan prefix policy. Key khusus `.local/managed-shipping-signing.json` diinisialisasi eksplisit, acak, privat, tidak overwrite; tidak memakai key fixture/katalog/integrasi. Source dan signature release lama tidak diubah. Tidak ada download/eksekusi binary provider.

## Batas yang belum selesai

`configurationReady` berarti signature dan snapshot sesuai policy, **bukan installable**. API mengembalikan `installable:false` dengan blocker `managed_distribution_not_available` dan `engine_grant_enforcement_not_available`. Release tidak masuk registry fixture, tidak membuat assignment/consent/grant/token atau memilih provider checkout.

Sebelum membuka tombol Install: tambahkan assignment managed release ke merchant terverifikasi, eligibility yang diperiksa ulang pada Prepare/Consent/Consume/Activate, binding consent yang tepat, engine readiness/delegation dan grant enforcement pada seluruh jalur pilot tanpa fallback legacy. Integrasikan daftar Installed Apps dan Additional shipping methods dari state backend. Uninstall wajib menutup operasi baru tanpa menghapus credential/pengiriman lama. Snapshot fixture dan generic integration release tetap tidak dapat dipromosikan otomatis.

## Rollout dan compatibility

- Migration `0016_managed_shipping.sql` additive; backup privat sebelum apply. Tidak ada perubahan instalasi/akun/key lama dan tidak menyentuh database API-Kurir/Core. Server memverifikasi migration sebelum start; tidak melakukan DDL otomatis.
- Backend Core pengamanan dan header Dashboard di-rollout bersama. Perbaikan slash ganda credential endpoint diperlukan karena path ambigu ditolak. Session expired mengikuti login/refresh existing; jangan membuka bypass decode.
- Production harus mengatur `SHIPPING_MANAGEMENT_DASHBOARD_ORIGINS` ke origin HTTPS dashboard exact. Fallback origin Core-preview hanya compatibility lokal; tidak ada konfigurasi valid berarti management gagal tertutup. Normalisasi URL/path atau fallback wildcard dilarang.
- Provider codes lain, client wildcard shipping pihak ketiga dan alur SDK management perlu inventaris/migrasi eksplisit; endpoint yang tidak ada dalam allowlist ditolak, bukan diteruskan diam-diam. Existing checkout/order calls yang langsung menuju engine tidak berubah, dan **belum mendapat grant App Platform**.
- Forward-fix bila ada client shipping yang perlu adaptasi; jangan rollback ke proxy tanpa autentikasi. Rollback pipeline release boleh menghentikan signing/submission, tetap simpan tabel/key/release/audit dan memungkinkan suspend; jangan menghapus schema atau mengganti key.
- Pengujian ini tidak membuktikan kondisi deployment production atau kesiapan ongkir nyata; tidak ada request API-Kurir nyata, booking atau transaksi yang dijalankan.

## Verifikasi

- Pengujian route Express asli dengan Prisma/Redis/network tiruan: bypass token/domain tertutup; management sah, public domain dan webhook tetap terpisah.
- Unit Core: privilege shipping, sesi dicabut, cross-merchant, CSRF, path/method allowlist, credential redaction, ukuran response dan error handling; regresi Apps/lifecycle/testing.
- PostgreSQL disposable + HTTP portal nyata: submit/review/sign/suspend, concurrent review, idempotency, organisasi lain, immutable manifest/signature, signer hilang dan registry fixture tetap tertutup. Regresi Core → ConnectRPC + Prisma dan generic integration/testing juga dijalankan.
- Manifest/key tests, Go build/vet/race, lint scoped, serta API docs generated drift check. ESLint Core existing memiliki rule `indent: never` yang invalid; pemeriksaan scoped memakai override `indent: off`, tanpa mengubah konfigurasi global.
