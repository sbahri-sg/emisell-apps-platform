# Navigasi aplikasi pada Portal Developer

Setelah membuat draft, developer masuk ke Overview aplikasi. Sidebar menjadi kontekstual: Overview, Monitoring, Logs, Versions, App settings. Tombol Semua aplikasi menghapus konteks. Query `app` hanya memilih record dari daftar organisasi yang diotorisasi backend; tidak menentukan role atau tenant.

| Halaman | Implementasi Emisell |
| --- | --- |
| Overview | Ringkasan draft, status lifecycle, aktivitas review, testing, dan CLI. |
| Monitoring | Status konfigurasi dari draft/pengajuan. Metrik request, performa embedded, dan webhook belum tersedia; tidak diisi angka simulasi. |
| Logs | Aktivitas draft terakhir, pengajuan, dan keputusan review. Filter waktu/jenis/pencarian. Bukan log runtime atau seluruh audit. |
| Versions | Draft yang dapat diedit dan snapshot pengajuan yang immutable. Detail memakai review existing. Approved bukan active/published. Alat rilis dan testing organisasi tetap dapat dibuka dari sini. |
| App settings | Client ID tetap per aplikasi, Secret terenkripsi yang bisa ditampilkan/disalin/dirotasi, identitas aplikasi dan konfigurasi versi. Client rilis lama tersedia terpisah untuk kompatibilitas. |

`?view=reviews&app=...` menyorot Versions; `?view=app-clients&app=...` menyorot App settings. Tautan lama tetap berfungsi. Create app menerbitkan identitas aplikasi secara atomik, tetapi tidak membuat grant atau instalasi.

App settings menampilkan Credentials langsung di atas informasi akun, identitas aplikasi, dan konfigurasi versi. Reveal membutuhkan tindakan eksplisit dan dicatat di audit. Verifikasi client rilis lama tetap terpisah. Lihat `docs/application-credentials.md` untuk upgrade database dan konfigurasi enkripsi.

## Referensi desain, bukan kontrak Emisell

Dipertimbangkan dari dashboard aplikasi Shopify (Overview, Monitoring, Logs, Versions, App settings) dan dokumentasi resminya:

- https://shopify.dev/docs/apps/build/dev-dashboard
- https://shopify.dev/docs/apps/build/dev-dashboard/monitoring-and-logs
- https://shopify.dev/docs/apps/build/dev-dashboard/create-apps-using-dev-dashboard

Bagian Settings referensi juga memiliki kontak, Pub/Sub, EventBridge, automation token, ikon, dan penghapusan. Tidak ditambahkan sebagai kontrol palsu: kontrak pengelolaan fitur tersebut belum tersedia pada portal Emisell. Fungsi Shopify-specific tidak disalin. Referensi ini internal, tidak diterbitkan ke dokumentasi publik atau README CLI.

Tidak ada perubahan backend, database, akun, scope, endpoint runtime, atau deployment untuk increment navigasi ini.
