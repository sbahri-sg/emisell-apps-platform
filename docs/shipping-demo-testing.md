# Pengujian Emisell Kurir dan RajaOngkir

Status terbaru 6 September 2026: **instalasi managed Emisell Kurir, pemeriksaan grant dan API-Kurir lokal terisolasi telah diimplementasikan**, dengan binding signed `api-kurir/emisell`. Hasil uji backend menggunakan merchant sintetis; instalasi pada toko pengguna tetap dilakukan oleh pengguna sendiri. Ini bukan migrasi engine atau checkout production.

## Tahap aktif — pengujian instalasi dengan engine lokal

### Status pada Aplikasi saya

- Label utama membaca status rilis, publikasi dan assignment dari API Developer existing. Emisell Kurir menampilkan **Testing · v1.0.0** ketika release signed dan assignment approved masih siap; **Draft r2** tetap informasi revisi metadata. RajaOngkir tetap Draft selama belum memiliki rilis/pengajuan.
- Status dicocokkan dengan app, versi dan sumber release; testing versi lama tidak otomatis berlaku pada draft versi baru. Assignment approved tetapi belum siap ditandai **Testing belum siap**. Suspend/revoke tidak tetap ditampilkan sebagai Testing siap. Testing bukan publikasi App Store atau grant merchant.
- Data gagal/tidak lengkap tampil **Belum terverifikasi**, bukan fallback Draft. Gunakan **Perbarui status** atau kembali ke Aplikasi saya untuk membaca ulang; ini bukan live monitoring. Tidak ada mutation release, assignment, instalasi atau akun dari penentuan label.

### Batas pengujian lokal

- App `app_V77HKNADFX765TGOJZ44CK6EUP` (Emisell Kurir · Test) versi 1.0.0, release `msrel_RZYDRP4LOAYYPPTMRXR5Q3IDM6` dan assignment `testasgn_CJAEHLAB3LJOZJ5AJMQTTTKQDW` tetap dipakai; tidak membuat app/assignment pengganti.
- Engine API-Kurir lokal berada di loopback `127.0.0.1:8089`, database terpisah `api_kurir_managed_local` pada port 55439. Konfigurasi privat operator, bukan credential portal/developer/browser. Layanan API-Kurir produksi dan credential provider tidak dimuat.
- Core mengarahkan hanya merchant pengujian yang enrolled ke engine ini. Error atau grant tidak aktif tidak boleh jatuh kembali ke produksi. Rute checkout/order server-side belum dimigrasikan.
- Gunakan Dashboard existing: Settings → Apps → Emisell Kurir · Test → Review permissions & install → Install. Tidak ada checkbox tambahan. Instalasi aktif muncul pada Installed Apps dan Settings → Shipping → Additional shipping methods. Open app menuju halaman pengaturan provider Emisell yang lama.
- Install tidak memilih provider/service checkout otomatis. Konfigurasi layanan tetap aksi pengguna. Data engine saat ini hanya lokasi/tarif **sample lokal** (`loc_test_jakarta` → `loc_test_bandung`, JNE JTR); jangan menganggapnya katalog/tarif nasional atau provider live.
- Grant dipastikan ulang untuk setiap request settings/rates; release suspend, assignment revoke, uninstall atau service grant unavailable menutup akses baru. Rate cache/singleflight tidak melewati pemeriksaan ini. Receipt retry membaca status terkini, termasuk setelah uninstall.
- Runner disposable lulus alur signed assignment → consent → installation → active grant → manual provider/service selection → sample rates → suspend/uninstall denial, beserta regresi Core Express/session/Prisma. Tidak memberi consent atau memilih provider di toko pengguna.
- Pembacaan ulang layanan lokal setelah restart memverifikasi assignment asli `installable=true`, readiness engine sehat, full Core key ditolak oleh endpoint grant engine, dan app managed belum aktif pada toko pengguna. Request engine merchant tanpa instalasi ditolak 403. Tidak ada mutation pada pemeriksaan ini.
- Verifikasi akhir: 144 unit Core terkait, 57 tes Dashboard Apps, 35 tes Portal, contract lint/backward compatibility, Go build/vet dan race tests terkait lulus. Typecheck Dashboard penuh masih gagal pada error existing di luar Apps/komponen Shipping yang diubah; tidak ada error pada area tersebut. Request HTTP halaman merchant menerima redirect autentikasi; ini bukan verifikasi browser dengan sesi pengguna.
- Build Portal berhasil. Build penuh Dashboard merchant masih terhalang dependency/type error existing di luar Apps (termasuk modul Domains); verifikasi scoped Apps tetap dijalankan. Browser visual QA tidak dilakukan dalam increment ini.
- RajaOngkir belum diaktifkan; input credential dan integrasi provider live merupakan tahap terpisah. Detail keputusan dan rollback: ADR 0027.

## Riwayat — hasil distribusi managed melalui Testing

- Menu Testing Admin/Developer yang sudah ada menerima signed provider terkelola melalui `releaseKind=managed_shipping`. Assignment demo `testasgn_CJAEHLAB3LJOZJ5AJMQTTTKQDW` berstatus approved, revision 2, untuk merchant `cmrnqsgdy0gj0v8l3xmtg50ur` dari provisioning Core toko yang sudah diotorisasi; tidak membuat merchant baru.
- Assignment dibaca ulang lewat API Developer dan Core RPC `ListAssignments`, tepat satu hasil dengan app developer asli `app_V77HKNADFX765TGOJZ44CK6EUP`, bukan fixture. Signature/configuration ready; `installable=false`. Dua audit request/approve. Pengulangan setup tidak membuat assignment baru.
- Settings → Apps membaca data assignment yang sama dan menjelaskan kendala managed, bukan meminta OAuth app eksternal. Installed Apps dan Additional shipping methods belum berubah karena tidak ada instalasi/grant yang dibuat.
- Migration 0017 sudah diterapkan setelah backup `.local/backups/before-managed-assignment-nzJUvL/emisell-local.dump`; Platform lokal direstart dengan signer managed pada HTTP dan RPC. Tidak ada perubahan account/key/provider/credential atau API-Kurir production.
- Tes PostgreSQL disposable/race lulus: ManagedShippingAssignments, ManagedShippingReleasePipeline, IntegrationReleasePipeline, TestingDistribution, CorePreviewNodeE2E. Core 57 tes terkait dan Dashboard Apps 55 tes lulus. Portal 35 tes, lint/typecheck/build lulus; Go build/vet/race terkait lulus. Core lint scoped tetap memakai override indent existing.
- Kelanjutan: tentukan target engine lokal terisolasi atau deployment oleh operator, implementasikan managed install eligibility/lifecycle dan enforcement grant seluruh jalur pilot, lalu sambungkan Additional shipping methods. Tidak boleh membuka Install hanya berdasarkan assignment. Detail ADR 0026.

## Riwayat — hasil tahap pengamanan dan persiapan release

- App Emisell Kurir tetap `app_V77HKNADFX765TGOJZ44CK6EUP`, draft revision 2 (summary/description disesuaikan), versi 1.0.0. Akun/organisasi/endpoint kosong/scope minimal tetap sama.
- Managed release: `msrel_RZYDRP4LOAYYPPTMRXR5Q3IDM6`, status `signed`, revision 3; tiga audit submit/review/sign. Signature dan checksum hasil pembacaan ulang API diverifikasi. RajaOngkir tetap draft revision 1, tidak diubah.
- API pengelolaan baru: `/api/v1/developer/managed-shipping-releases` dan `/api/v1/admin/managed-shipping-releases`. Detail menampilkan signature, history dan readiness. Belum ada menu authoring managed release pada frontend Portal Developer; endpoint tersedia dalam Dokumentasi API Admin.
- `configurationReady=true`, `installable=false`. Blocker aktual: `managed_distribution_not_available`, `engine_grant_enforcement_not_available`. Generic integration release/Testing dan fixture registry tidak diubah menjadi runtime managed.
- Migration 0016 sudah diterapkan ke database Platform lokal setelah backup privat `.local/backups/before-managed-shipping-1Hlcen/emisell-local.dump`. Key khusus managed shipping diprovision privat tanpa overwrite; server Platform lokal direstart. Tidak ada migration database Core/API-Kurir, assignment, consent, instalasi, token, operasi provider atau request API-Kurir nyata.
- Pengamanan Core menutup bypass source yang ditemukan sebelumnya. Pengujian router **setelah patch**, dengan database/network tiruan, menolak token salah-signature dan domain-only management; verified session, domain publik dan webhook terpisah tetap bekerja. Ini bukan klaim hasil audit production atau grant enforcement seluruh engine.
- Verifikasi: 88 unit Core terkait, 1 pengujian router asli terisolasi, 54 tes Dashboard terkait; PostgreSQL disposable lulus ManagedShippingReleasePipeline, CorePreviewNodeE2E, IntegrationReleasePipeline, TestingDistribution. Go build/vet dan unit/race manifest/key/service lulus. Lint scoped Core memakai override untuk rule indent existing yang invalid. Docs Admin: 80 operasi dari 19 kontrak; 34 tes, lint, typecheck dan build lulus. Build memberi warning chunk >500 kB dan keterbatasan klasifikasi route statis vinext; tidak memblokir build.
- Risiko konfigurasi terpisah: `.env.example` Core yang sudah ada mengandung nilai yang tampak seperti credential asli. Jangan salin ke log/dokumentasi; operator perlu memeriksa dan merotasinya bila aktif. Increment ini tidak mengubah/merotasi secret tersebut.

Tahap berikutnya: implementasi assignment managed + eligibility/lifecycle dan engine grant enforcement untuk pilot, baru hubungkan Installed Apps → Additional shipping methods. **Jangan meminta pengguna klik Install sebelum jalur tersebut benar-benar tersedia.** Detail keputusan: ADR 0025.

## Riwayat tahap awal — pembuatan draft demo

Bagian riwayat berikut adalah kondisi sebelum tahap pengamanan/release di atas. Status Emisell Kurir terbaru adalah **signed**; RajaOngkir tetap draft. Rencana kontrak managed pada riwayat ini telah diselesaikan oleh ADR 0025, tetapi assignment/install dan engine grant enforcement masih belum tersedia.

Portal: `http://localhost:4319/`, akun `developer@emisell.local`, organisasi `dev-emisell-local` (Emisell Developer). Tidak membuat atau merotasi akun/password/key.

| Nama draft | ID app developer | Provider API-Kurir |
|---|---|---|
| Emisell Kurir · Test | `app_V77HKNADFX765TGOJZ44CK6EUP` | `emisell` — built-in |
| RajaOngkir · Test | `app_552FDGDBJ5Y4ZTLKGEKV2YZ3YG` | `rajaongkir` — credential merchant |

Keduanya versi `1.0.0`, capability `shipping/v1`, scope draft `shipping.read`, revision 1 saat dibuat. Ditulis melalui login/API Developer dan dibaca ulang dari API list/detail dengan ownership organisasi. Pengulangan setup mempertahankan ID/revision dan tidak membuat duplikat.

Tidak ada endpoint pada draft karena engine belum mempunyai kontrak managed extension App Platform. URL dokumentasi API-Kurir bukan endpoint capability, OAuth callback atau health yang boleh dikarang. Draft belum di-submit, approved, signed, published atau assigned. Validator submission Aplikasi Integrasi umum saat ini masih meminta endpoint dan scope fixture lengkap; jangan menambah scope yang tidak dibutuhkan hanya untuk melewati validator.

## Riwayat — pengujian backend fixture

`local-kurir-provider` menambahkan fixture `emisell-provider-reference` dengan binding signed `engine=api-kurir`, `providerCode=emisell`. Fixture ini bukan ID app developer pada tabel di atas, tidak dimiliki/di-assign melalui portal, tidak di-seed pada server normal dan tidak memperoleh runtime produksi.

Readiness membaca detail exact provider menggunakan dummy credential pada HTTP loopback bertanda fixture:

- Emisell: `code=emisell`, `built_in=true`, `installed=true`, `available=true`.
- RajaOngkir: `code=rajaongkir`, `built_in=false`, `installed=true`, `available=true`.
- `active` bukan syarat install; provider yang digunakan checkout tetap dipilih oleh API-Kurir.
- Field wajib hilang, salah provider, salah built-in flag, unavailable, atau belum installed ditolak. Tidak mengirim POST/PATCH credential/activate/deactivate.

`TestShippingProviderLifecycle` menjalankan dua pasangan: RajaOngkir–KiriminAja (regresi) dan RajaOngkir–Emisell. Test memakai PostgreSQL disposable dan ConnectRPC asli: consent wajib; grant pending/active; isolasi merchant/actor/provider; replay/idempotency; uninstall saat engine outage; audit; reinstall; checkout fixture legacy tetap berfungsi. Kedua provider dapat di-uninstall tanpa menghapus credential atau menonaktifkan engine. Tidak menerbitkan app token atau mengaktifkan capability routing untuk profile lifecycle-only.

Dari checkout api-service:

```sh
node scripts/test-app-platform-core-preview.mjs /absolute/path/emisell-app-platform --shipping-sandbox
```

Nama flag adalah harness backend; tidak membuat `/sandbox/settings/apps`. Tidak memakai database merchant, credential RajaOngkir, saldo Emisell/Biteship, atau provider API production. Tanpa runner/DSN disposable, Go integration tests dapat skip; skip bukan bukti lulus.

## Riwayat — gate sebelum uji merchant

1. Kontrak managed extension yang mengikat app developer/release/checksum ke engine/provider secara signed; bukan mapping nama atau konversi metadata menjadi simulator. Pertahankan compatibility release lama.
2. Publikasi release pengujian terverifikasi, assignment ke merchant yang diotorisasi, consent dan install eligibility melalui lifecycle existing. Draft/approval assignment bukan grant.
3. Auth khusus engine server-to-server, grant enforcement/freshness/revocation dan provider/version binding pada seluruh jalur operasi yang diaktifkan, termasuk mencegah bypass legacy. Core full-access key bukan secret app developer. Jangan mewajibkan OAuth app eksternal fiktif untuk engine internal terkelola.
4. Hubungkan UI Settings → Apps dan konfigurasi Shipping **existing**, tanpa shell baru. Bedakan aplikasi terpasang dari provider checkout terpilih. Uninstall tidak menghapus engine, secret upstream atau shipment.
5. Pilih endpoint API-Kurir lokal/staging yang disetujui dan batas pengujian. Environment `sandbox` tidak otomatis berarti semua pembacaan ongkir/tracking bebas quota; belum ada koneksi nyata yang dilakukan oleh increment ini.

Daftar gate lengkap, race grant/provider dan policy shipment berjalan: ADR 0024. Kebutuhan environment di atas bukan satu-satunya blocker: kontrak/pipeline managed extension dan engine grant enforcement juga belum diimplementasikan. Jangan menyatakan dua app siap dipasang hanya karena URL testing telah diberikan.

## Riwayat — keputusan pengujian dan temuan sebelum patch

Pengguna memilih **Emisell Kurir terlebih dahulu**; credential RajaOngkir akan diisi sendiri setelah alur pertama aman. App gratis bukan bukti semua operasi shipping gratis. Batas uji yang disampaikan: readiness/koneksi dan cek ongkir saja, tanpa booking, shipment, pickup, label berbayar atau transaksi. Sampai catatan ini dibuat, belum ada request nyata yang dikirim ke API-Kurir dan belum ada draft yang di-install.

Inspeksi source Core menunjukkan blocker sebelum cutover:

- `src/modules/extentions/shipping/routes.js` memasang proxy seluruh metode/path setelah `domainResolve()` dan `ctx()`, bukan authorizer App Platform terverifikasi.
- `src/middlewares/domainResolve.js` memakai `jwt.decode` untuk mengambil role/merchant dari access cookie; cabang MERCHANT_ADMIN dapat melanjutkan tanpa signature verification di middleware tersebut. Domain resolution bukan pengganti otorisasi operasi pengelolaan integrasi.
- `src/modules/extentions/shipping/controller.js` meneruskan request memakai service key API-Kurir dan merchant dari context. Jalur ini tidak melakukan pemeriksaan current installation grant App Platform. Mount `/v1` pada source tidak menambahkan auth global yang menutup gap tersebut.

Ini **temuan source**, bukan bukti deployment API-Kurir/Core production memakai source yang sama, dan belum diuji dengan token palsu atau request eksploit pada layanan hidup. Shared middleware juga dipakai checkout/storefront; jangan mengganti semuanya secara global atau menutup checkout publik tanpa migration path. Diperlukan pengerjaan boundary shipping legacy yang scoped: pisahkan operasi checkout publik dari pengelolaan provider terautentikasi, validasi sesi/merchant/izin dan CSRF, allowlist metode/path, kemudian grant enforcement untuk merchant yang dimigrasi tanpa fallback bypass.

Verifikasi increment ini: unit/race tests manifest + readiness + installation lulus; runner PostgreSQL disposable lulus untuk kedua pasangan provider termasuk uninstall provider built-in dan regression Core/agregator; `go build ./...` dan `go vet ./...` lulus. Setup draft dijalankan ulang dengan ID/revision tetap dan tidak membuat duplikat. Semua data/container test disposable dibersihkan. Source Core, API-Kurir, Dashboard dan layanan yang sedang berjalan tidak diubah pada increment ini.
