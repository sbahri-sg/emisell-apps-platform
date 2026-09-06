# ADR 0007 — Admin, App Store, Developer Portal; retire merchant UI

Tanggal: 5 September 2026. Status: disetujui pengguna; removal frontend diterapkan. Implementasi fitur ketiga surface tetap tahap terpisah.

Pembaruan: milestone akses Admin/Developer dan draft–review kini diimplementasikan melalui ADR 0008. Bagian halaman status di bawah mencatat hasil increment removal sebelumnya, bukan keadaan fitur portal terbaru.

## Koreksi konsep

Dashboard merchant sebelumnya salah menjadi pusat produk. Scope frontend ADR 0001, 0002, 0005, serta panel transaksi pada pekerjaan ADR 0006 digantikan oleh keputusan ini. Bagian backend/security pada ADR tersebut tetap berlaku sejauh tidak bertentangan. ADR lama merupakan riwayat, bukan instruksi membangun ulang workspace merchant.

Tiga surface terpisah memakai satu engine Go modular monolith:

| Surface | Port | Scope |
|---|---|---|
| Admin | 4317 | Review, approve/reject, publish/suspend, developer, operasional platform |
| App Store | 4318 | Listing/detail aplikasi published yang layak ditampilkan |
| Developer | 4319 | App ownership, manifest, release, konfigurasi, submission, log yang diizinkan |

Merchant tetap di Emisell Core. Installation terikat tenant Core, bukan workspace buatan portal. Developer organization dan merchant tenant adalah domain berbeda. Submit release tidak sama dengan publish. Aplikasi tetap gratis.

## Penghapusan yang diterapkan

- Menghapus komponen UI merchant dan adapter/state demo maupun API-nya, termasuk selection/rename/reset workspace, daftar installation, consent/install/activate/uninstall, aktivitas dan panel operasional/payment tenant.
- Menghapus stylesheet serta tests frontend yang hanya menguji fitur tersebut, dan harness browser merchant yang memakai port 4319. Port itu sekarang dicadangkan untuk Developer Portal.
- Root frontend menggantinya dengan halaman status Admin tanpa data privat, login, atau tindakan bisnis. Query/view/workspace lama tidak dipakai untuk routing. Tidak ada fallback mode demo merchant.
- Primitive UI/brand, framework, dependency, lockfile, konfigurasi Sites, dan proxy lokal dipertahankan. Tidak ada deployment atau port baru yang dijalankan pada increment removal.
- Penghapusan source dapat dipulihkan dari archive lokal yang dibuat sebelum removal; jangan restore otomatis tanpa permintaan pengguna.

## Batas yang dipertahankan

Tidak menghapus tenant isolation, module installation/capability/OAuth/events/webhook, service credentials, data installation/transaction/audit, schema, maupun migration. Tidak menghapus data `local-store`/`local-studio` atau mencabut instalasinya. APIs owner workspace lama masih tersedia di backend untuk compatibility dan pengujian reference; bukan antarmuka publik final dan tidak digunakan halaman Admin.

Callback OAuth existing, API session, dan contract Go tidak dipindah diam-diam. Flow browser merchant tidak lagi tersedia; OAuth/consent surface final harus diselesaikan bersama integrasi Core berikutnya. Pekerjaan callback/payment Tahap 5 tetap belum dianggap selesai secara keseluruhan dan tidak dilanjutkan pada task removal ini.

## Security dan migration path berikutnya

1. Tetapkan principal/permission Admin terpisah dari owner toko. Tidak ada privilege upgrade implisit bagi akun lama. Endpoint admin fail closed sampai authorization benar-benar diimplementasikan.
2. Buat Portal Developer dengan ownership organisasi/app dan pengujian akses lintas developer. Cookie audience/name/path dan CSRF harus eksplisit; beda port tidak mengisolasi cookie atau hak akses.
3. Implementasikan draft → submission → review → publish lewat module developer/review/registry. Pisahkan metadata publik dari configuration, secret, review notes, dan draft.
4. App Store pada frontend/port sendiri hanya membaca listing published. Install/consent memakai principal tenant dari Core; bukan tenant ID arbitrer dari URL. Jangan menyalin sesi admin/developer sebagai akses merchant.
5. Migrasikan callers legacy bertahap setelah contract pengganti tersedia. Baru deprecate/remove route backend jika compatibility window dan tests tersedia; jangan drop schema sebagai cara menghapus UI.

Tidak membuat folder/halaman/role palsu sekadar agar tiga portal terlihat selesai. Batas removal dinyatakan tuntas ketika source merchant dan entrypoint-nya hilang, dokumentasi konsisten, build/test/lint lulus, dan URL lama tidak lagi merender atau mengambil data merchant.
